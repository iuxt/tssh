/*
MIT License

Copyright (c) 2023-2026 The Trzsz SSH Authors.

Permission is hereby granted, free of charge, to any person obtaining a copy
of this software and associated documentation files (the "Software"), to deal
in the Software without restriction, including without limitation the rights
to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
copies of the Software, and to permit persons to whom the Software is
furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in all
copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
SOFTWARE.
*/

package tssh

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"testing/iotest"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/trzsz/ssh_config"
)

func TestSessionLogPlainTextAndChunkBoundaries(t *testing.T) {
	// Read one byte at a time, splitting UTF-8, CRLF and every escape sequence.
	source := "\x1b]0;private title\x07\x1b[32m中文🙂\x1b[0m\r\n" +
		"\x1b]8;;https://example.com\x1b\\link\x1b]8;;\x1b\\\ttext\x07\r\n" +
		"\x1bPdiscard this payload\x1b\\10%\r20%\r\nlast prompt> "
	for _, size := range []int{1, 2, 7, 32768} {
		t.Run(fmt.Sprint(size), func(t *testing.T) {
			home := t.TempDir()
			now := time.Date(2026, 10, 21, 16, 0, 0, 0, time.Local)
			log, err := newSessionLog(home, "开发主机", now)
			require.NoError(t, err)
			defer log.close()
			reader := newSessionLogReader(iotest.DataErrReader(strings.NewReader(source)), log)
			var forwarded bytes.Buffer
			buffer := make([]byte, size)
			for {
				n, err := reader.Read(buffer)
				forwarded.Write(buffer[:n])
				if err != nil {
					require.ErrorIs(t, err, io.EOF)
					break
				}
			}
			assert.Equal(t, source, forwarded.String(), "terminal bytes must be unchanged")
			data, err := os.ReadFile(filepath.Join(home, ".tssh", "log", "开发主机-2026-10-21.log"))
			require.NoError(t, err)
			assert.Equal(t, "中文🙂\nlink\ttext\n10%\n20%\nlast prompt> ", string(data))
		})
	}
}

func TestSessionLogAppendPermissionsAndSafeName(t *testing.T) {
	home := t.TempDir()
	now := time.Date(2026, 10, 21, 0, 0, 0, 0, time.Local)
	alias := "../开发\\主机:22\n"
	name := sessionLogName(alias, now)
	assert.Equal(t, ".._开发_主机_22-2026-10-21.log", name)
	assert.Equal(t, "session-2026-10-21.log", sessionLogName("", now))
	for _, text := range []string{"first session\n", "second session\n"} {
		log, err := newSessionLog(home, alias, now)
		require.NoError(t, err)
		log.write([]byte(text))
		log.close()
		log.close()
	}
	path := filepath.Join(home, ".tssh", "log", name)
	data, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Equal(t, "first session\nsecond session\n", string(data))
	if runtime.GOOS != "windows" {
		file, err := os.Stat(path)
		require.NoError(t, err)
		dir, err := os.Stat(filepath.Dir(path))
		require.NoError(t, err)
		assert.Equal(t, os.FileMode(0600), file.Mode().Perm())
		assert.Equal(t, os.FileMode(0700), dir.Mode().Perm())
	}
}

func TestSessionLogConcurrentStreams(t *testing.T) {
	log, err := newSessionLog(t.TempDir(), "host", time.Now())
	require.NoError(t, err)
	defer log.close()
	path := log.file.Name()
	stdout := newSessionLogReader(strings.NewReader("\x1b[31mstdout\x1b[0m\n"), log)
	stderr := newSessionLogReader(strings.NewReader("stderr\n"), log)
	// Leave stdout in the middle of an escape sequence while stderr prints.
	prefix := make([]byte, 3)
	_, err = io.ReadFull(stdout, prefix)
	require.NoError(t, err)
	_, err = io.Copy(io.Discard, stderr)
	require.NoError(t, err)
	_, err = io.Copy(io.Discard, stdout)
	require.NoError(t, err)
	var group sync.WaitGroup
	for i := range 2 {
		group.Go(func() {
			reader := newSessionLogReader(strings.NewReader(strings.Repeat(fmt.Sprintf("stream%d\n", i), 1000)), log)
			_, err := io.Copy(io.Discard, reader)
			assert.NoError(t, err)
		})
	}
	group.Wait()
	data, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.True(t, strings.HasPrefix(string(data), "stderr\nstdout\n"))
	assert.Equal(t, 1000, strings.Count(string(data), "stream0\n"))
	assert.Equal(t, 1000, strings.Count(string(data), "stream1\n"))
}

func TestSessionLogWriteFailureDoesNotAffectOutput(t *testing.T) {
	log, err := newSessionLog(t.TempDir(), "host", time.Now())
	require.NoError(t, err)
	defer log.close()
	require.NoError(t, log.file.Close()) // Simulate a log device that can no longer be written.
	reader := newSessionLogReader(strings.NewReader("remote output\n"), log)
	data, err := io.ReadAll(reader)
	require.NoError(t, err)
	assert.Equal(t, "remote output\n", string(data))
	assert.Nil(t, log.file, "disable further logging after the first write failure")
	log.write([]byte("more output"))
}

func TestSetupSessionLoggingConfiguration(t *testing.T) {
	previousConfig, previousHome := userConfig, userHomeDir
	previousExitFuncs := onExitFuncs
	t.Cleanup(func() {
		userConfig, userHomeDir = previousConfig, previousHome
		onExitFuncs = previousExitFuncs
	})
	for _, tc := range []struct {
		name, main, extra, option string
		enabled                   bool
	}{
		{name: "default"},
		{name: "host", main: "Host target\n EnableSessionLog yes", enabled: true},
		{name: "inherited", main: "Host *\n EnableSessionLog yes", enabled: true},
		{name: "extension", main: "EnableSessionLog no", extra: "EnableSessionLog yes", enabled: true},
		{name: "extension-disable", main: "EnableSessionLog yes", extra: "EnableSessionLog no"},
		{name: "cli", option: "yes", enabled: true},
		{name: "cli-disable", main: "EnableSessionLog yes", extra: "EnableSessionLog yes", option: "no"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			userHomeDir = t.TempDir()
			onExitFuncs = nil
			main, err := ssh_config.Decode(strings.NewReader(tc.main))
			require.NoError(t, err)
			extra, err := ssh_config.Decode(strings.NewReader(tc.extra))
			require.NoError(t, err)
			userConfig = &tsshConfig{config: main, exConfig: extra}
			args := &sshArgs{Destination: "target"}
			if tc.option != "" {
				args.Option.options = map[string][]string{"enablesessionlog": {tc.option}}
			}
			out, stderr := strings.NewReader("stdout\n"), strings.NewReader("stderr\n")
			conn := &sshConnection{param: &sshParam{args: args}, serverOut: out, serverErr: stderr}
			closeLog := setupSessionLogging(conn)
			defer closeLog()
			for _, reader := range []io.Reader{conn.serverOut, conn.serverErr} {
				_, err := io.Copy(io.Discard, reader)
				require.NoError(t, err)
			}
			closeLog()
			files, err := filepath.Glob(filepath.Join(userHomeDir, ".tssh", "log", "target-*.log"))
			require.NoError(t, err)
			if tc.enabled {
				require.Len(t, files, 1)
				data, err := os.ReadFile(files[0])
				require.NoError(t, err)
				assert.Equal(t, "stdout\nstderr\n", string(data))
				cleanupOnExit() // Deferred and signal cleanup can both close safely.
			} else {
				assert.Empty(t, files)
				assert.Same(t, out, conn.serverOut)
				assert.Same(t, stderr, conn.serverErr)
				_, err := os.Stat(filepath.Join(userHomeDir, ".tssh", "log"))
				assert.True(t, os.IsNotExist(err))
			}
		})
	}
}

func TestSetupSessionLoggingOpenFailure(t *testing.T) {
	previousConfig, previousHome := userConfig, userHomeDir
	t.Cleanup(func() { userConfig, userHomeDir = previousConfig, previousHome })
	userConfig = &tsshConfig{}
	userHomeDir = t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(userHomeDir, ".tssh"), []byte("blocked"), 0600))
	out := strings.NewReader("remote output")
	conn := &sshConnection{param: &sshParam{args: &sshArgs{
		Destination: "target", Option: sshOption{options: map[string][]string{"enablesessionlog": {"yes"}}},
	}}, serverOut: out}
	closeLog := setupSessionLogging(conn)
	defer closeLog()
	assert.Same(t, out, conn.serverOut)
	data, err := io.ReadAll(conn.serverOut)
	require.NoError(t, err)
	assert.Equal(t, "remote output", string(data))
}
