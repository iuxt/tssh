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
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/charmbracelet/x/ansi"
)

// sessionLog is shared by stdout and stderr. Each stream has its own parser so
// an incomplete escape sequence on one stream cannot consume the other stream.
type sessionLog struct {
	mu   sync.Mutex
	file *os.File
}

func sessionLogName(alias string, now time.Time) string {
	alias = strings.Map(func(r rune) rune {
		if unicode.IsControl(r) || strings.ContainsRune(`/\<>:"|?*`, r) {
			return '_'
		}
		return r
	}, strings.TrimSpace(alias))
	if alias == "" {
		alias = "session"
	}
	return alias + "-" + now.Format("2006-01-02") + ".log"
}

func newSessionLog(home, alias string, now time.Time) (*sessionLog, error) {
	dir := filepath.Join(home, ".tssh", "log")
	if err := os.MkdirAll(dir, 0700); err != nil {
		return nil, fmt.Errorf("create session log directory: %w", err)
	}
	path := filepath.Join(dir, sessionLogName(alias, now))
	file, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0600)
	if err != nil {
		return nil, fmt.Errorf("open session log [%s]: %w", path, err)
	}
	return &sessionLog{file: file}, nil
}

func (l *sessionLog) write(data []byte) {
	if len(data) == 0 {
		return
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.file == nil {
		return
	}
	if err := writeAll(l.file, data); err != nil {
		warning("write session log [%s] failed, logging disabled: %v", l.file.Name(), err)
		_ = l.file.Close()
		l.file = nil
	}
}

func (l *sessionLog) close() {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.file != nil {
		if err := l.file.Close(); err != nil {
			warning("close session log [%s] failed: %v", l.file.Name(), err)
		}
		l.file = nil
	}
}

type sessionLogReader struct {
	reader io.Reader
	log    *sessionLog
	parser *ansi.Parser
	buffer []byte
	lastCR bool
}

func newSessionLogReader(reader io.Reader, log *sessionLog) *sessionLogReader {
	r := &sessionLogReader{reader: reader, log: log, parser: ansi.NewParser()}
	// Escape payloads (titles, clipboard data, images, etc.) are discarded.
	// Keep a bounded buffer even for very large or unterminated sequences.
	r.parser.SetDataSize(1)
	r.parser.SetHandler(ansi.Handler{
		Print: func(ch rune) {
			r.buffer = utf8.AppendRune(r.buffer, ch)
			r.lastCR = false
		},
		Execute: func(ch byte) {
			switch ch {
			case '\r':
				r.buffer = append(r.buffer, '\n')
				r.lastCR = true
			case '\n':
				if !r.lastCR {
					r.buffer = append(r.buffer, '\n')
				}
				r.lastCR = false
			case '\t':
				r.buffer = append(r.buffer, '\t')
				r.lastCR = false
			}
		},
	})
	return r
}

func (r *sessionLogReader) Read(p []byte) (int, error) {
	n, err := r.reader.Read(p)
	r.buffer = r.buffer[:0]
	for _, ch := range p[:n] {
		r.parser.Advance(ch)
	}
	r.log.write(r.buffer)
	return n, err
}

// Called after the transfer filter: log displayed output, excluding file data.
// Only remote output is captured; stdin is never recorded separately.
func setupSessionLogging(sshConn *sshConnection) func() {
	args := sshConn.param.args
	if !isConfigYes(getExOptionConfig(args, "EnableSessionLog")) {
		return func() {}
	}
	log, err := newSessionLog(userHomeDir, args.Destination, time.Now())
	if err != nil {
		warning("session logging disabled: %v", err)
		return func() {}
	}
	if sshConn.serverOut != nil {
		sshConn.serverOut = newSessionLogReader(sshConn.serverOut, log)
	}
	if sshConn.serverErr != nil {
		sshConn.serverErr = newSessionLogReader(sshConn.serverErr, log)
	}
	addOnExitFunc(log.close)
	return log.close
}
