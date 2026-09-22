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
	"io"
	"os"
	"sync"
	"sync/atomic"

	"charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/colorprofile"
)

var (
	yellowColor = lipgloss.Color("3")
	blackColor  = lipgloss.Color("16")
)

var stdinFallbackBuf []byte
var stdinFallbackMu sync.Mutex

type teaStdinReader struct {
	fallbackFn func([]byte)
	cancelled  atomic.Bool
}

func (r *teaStdinReader) Read(p []byte) (int, error) {
	stdinFallbackMu.Lock()
	defer stdinFallbackMu.Unlock()

	if len(stdinFallbackBuf) > 0 {
		n := copy(p, stdinFallbackBuf)
		if n < len(stdinFallbackBuf) {
			stdinFallbackBuf = stdinFallbackBuf[n:]
		} else {
			stdinFallbackBuf = nil
		}
		return n, nil
	}

	n, err := os.Stdin.Read(p)

	if n > 0 && r.cancelled.Load() {
		if r.fallbackFn != nil {
			r.fallbackFn(p[:n])
		} else {
			stdinFallbackBuf = append(stdinFallbackBuf, p[:n]...)
		}
		return 0, io.EOF
	}

	return n, err
}

func newTeaOptions(fallbackFn func([]byte)) ([]tea.ProgramOption, func()) {
	if !isRunningOnOldWindows.Load() {
		return []tea.ProgramOption{tea.WithInput(os.Stdin)}, func() {}
	}

	width, height, err := getTerminalSize()
	if err != nil {
		warning("get terminal size failed: %v", err)
		width, height = 80, 40
	}

	trr := &teaStdinReader{fallbackFn: fallbackFn}
	return []tea.ProgramOption{
		tea.WithInput(trr),
		tea.WithWindowSize(width, height),
		tea.WithColorProfile(colorprofile.ANSI256),
	}, func() { trr.cancelled.Store(true) }
}
