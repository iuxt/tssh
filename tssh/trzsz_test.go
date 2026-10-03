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
	"context"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/trzsz/trzsz-go/trzsz"
)

func TestTransferOptionsDefaultToTrzszAndLrzsz(t *testing.T) {
	oriUserConfig := userConfig
	userConfig = &tsshConfig{}
	defer func() { userConfig = oriUserConfig }()

	options := getTransferOptions(&sshArgs{})

	assert.True(t, options.enableTrzsz)
	assert.True(t, options.enableZmodem)
	assert.False(t, options.enableDragFile)
	assert.False(t, options.enableOSC52)
	assert.False(t, options.disableFilter)
}

func TestTransferOptionsCanDisableTrzsz(t *testing.T) {
	previous := userConfig
	userConfig = &tsshConfig{}
	t.Cleanup(func() { userConfig = previous })

	for _, value := range []string{"No", "false", "0", " OFF "} {
		t.Run(value, func(t *testing.T) {
			options := getTransferOptions(&sshArgs{
				Option: sshOption{options: map[string][]string{"enabletrzsz": {value}}},
			})
			assert.False(t, options.enableTrzsz)
			assert.True(t, options.enableZmodem)
			assert.False(t, options.disableFilter)
		})
	}
}

func TestTransferOptionsCanDisableLrzsz(t *testing.T) {
	oriUserConfig := userConfig
	userConfig = &tsshConfig{}
	defer func() { userConfig = oriUserConfig }()

	options := getTransferOptions(&sshArgs{
		Option: sshOption{options: map[string][]string{"enablezmodem": {"No"}}},
	})

	assert.True(t, options.enableTrzsz)
	assert.False(t, options.enableZmodem)
	assert.False(t, options.disableFilter)
}

func TestTransferOptionsCanDisableAll(t *testing.T) {
	oriUserConfig := userConfig
	userConfig = &tsshConfig{}
	defer func() { userConfig = oriUserConfig }()

	options := getTransferOptions(&sshArgs{
		Option: sshOption{options: map[string][]string{
			"enabletrzsz":    {"No"},
			"enablezmodem":   {"No"},
			"enabledragfile": {"No"},
			"enableosc52":    {"No"},
		}},
	})

	assert.False(t, options.enableTrzsz)
	assert.False(t, options.enableZmodem)
	assert.False(t, options.enableDragFile)
	assert.False(t, options.enableOSC52)
	assert.True(t, options.disableFilter)
}

func TestDragFileUploadCommandDefaultsToRz(t *testing.T) {
	oriUserConfig := userConfig
	userConfig = &tsshConfig{}
	defer func() { userConfig = oriUserConfig }()

	assert.Equal(t, "rz", getDragFileUploadCommand(&sshArgs{}))
}

// Run the actual remote tools in a separate process because their entry points
// use process-wide standard streams and signal handlers.
func TestTransferToolProcess(t *testing.T) {
	tool := os.Getenv("TSSH_TEST_TRANSFER_TOOL")
	if tool == "" {
		return
	}
	_ = os.Unsetenv("TMUX")
	os.Args = []string{tool, "-q", os.Getenv("TSSH_TEST_TRANSFER_PATH")}
	switch tool {
	case "trz":
		os.Exit(trzsz.TrzMain())
	case "tsz":
		os.Exit(trzsz.TszMain())
	default:
		os.Exit(1)
	}
}

type transferTestClient struct {
	SshClient
	tunnel bool
	dials  atomic.Int32
}

func (c *transferTestClient) DialTimeout(network, addr string, timeout time.Duration) (net.Conn, error) {
	c.dials.Add(1)
	if !c.tunnel {
		return nil, fmt.Errorf("TCP forwarding disabled")
	}
	return net.DialTimeout(network, addr, timeout)
}

type transferTestSession struct{ SshSession }

func (*transferTestSession) RedrawScreen() {}

type transferTestOutput struct{ io.Writer }

func (transferTestOutput) Close() error { return nil }

func TestTransferFilterUploadAndDownload(t *testing.T) {
	previous := userConfig
	userConfig = &tsshConfig{}
	t.Cleanup(func() { userConfig = previous })

	for _, tunnel := range []bool{false, true} {
		for _, tool := range []string{"trz", "tsz"} {
			t.Run(fmt.Sprintf("%s/tunnel=%t", tool, tunnel), func(t *testing.T) {
				sourceDir, destinationDir := t.TempDir(), t.TempDir()
				name := "传输 test.bin"
				content := bytes.Repeat([]byte("trzsz\x00\x01\r\n\xff"), 16384)
				sourcePath := filepath.Join(sourceDir, name)
				require.NoError(t, os.WriteFile(sourcePath, content, 0600))

				remotePath := sourcePath
				if tool == "trz" {
					remotePath = destinationDir
				}
				ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
				defer cancel()
				cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestTransferToolProcess$")
				cmd.Env = append(os.Environ(), "TSSH_TEST_TRANSFER_TOOL="+tool,
					"TSSH_TEST_TRANSFER_PATH="+remotePath)
				var stderr bytes.Buffer
				cmd.Stderr = &stderr
				serverIn, err := cmd.StdinPipe()
				require.NoError(t, err)
				defer serverIn.Close()
				serverOut, serverOutput := io.Pipe()
				defer serverOut.Close()
				defer serverOutput.Close()
				cmd.Stdout = serverOutput

				clientIn, clientInput := io.Pipe()
				defer clientIn.Close()
				defer clientInput.Close()
				client := &transferTestClient{tunnel: tunnel}
				args := &sshArgs{Option: sshOption{options: map[string][]string{
					"enablezmodem": {"No"}, "defaultdownloadpath": {destinationDir},
				}}}
				filter := newTransferFilter(&sshConnection{
					client: client, session: &transferTestSession{},
					serverIn: serverIn, serverOut: serverOut, param: &sshParam{args: args},
				}, clientIn, transferTestOutput{io.Discard}, 80, getTransferOptions(args))
				defer filter.Close()
				var uploaded <-chan error
				if tool == "trz" {
					uploaded, err = filter.OneTimeUpload([]string{sourcePath})
					require.NoError(t, err)
				}

				require.NoError(t, cmd.Start())
				err = cmd.Wait()
				require.NoError(t, err, "remote tool stderr: %s", stderr.String())
				require.NoError(t, ctx.Err(), "transfer timed out")
				if uploaded != nil {
					select {
					case err := <-uploaded:
						require.NoError(t, err)
					case <-ctx.Done():
						t.Fatal("upload result timed out")
					}
				}
				got, err := os.ReadFile(filepath.Join(destinationDir, name))
				require.NoError(t, err)
				assert.Equal(t, content, got)
				assert.Positive(t, client.dials.Load(), "SSH tunnel connector was not used")
			})
		}
	}
}
