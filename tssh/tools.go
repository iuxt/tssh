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
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/colorprofile"
)

var (
	redColor    = lipgloss.Color("1")
	greenColor  = lipgloss.Color("2")
	yellowColor = lipgloss.Color("3")
	cyanColor   = lipgloss.Color("6")
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

func toolsErrorExit(format string, a ...any) {
	msg := fmt.Sprintf(format, a...)
	fmt.Fprintf(os.Stderr, "\033[0;31m%s\033[0m\r\n", msg)
	cleanupOnExit()
	os.Exit(kExitCodeToolsError)
}

type inputValidator struct {
	validate func(string) error
}

type passwordModel struct {
	promptLabel   string
	helpMessage   string
	passwordInput string
	validator     *inputValidator
	cursorVisible bool
	done          bool
	quit          bool
	err           error
}

type tickMsg time.Time

func tickEvery(d time.Duration) tea.Cmd {
	return tea.Tick(d, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

func (m *passwordModel) Init() tea.Cmd {
	return tickEvery(500 * time.Millisecond)
}

func (m *passwordModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		switch keypress := msg.String(); keypress {
		case "ctrl+c":
			m.quit = true
			return m, tea.Quit
		case "ctrl+w":
			m.passwordInput = ""
			return m, nil
		case "enter":
			err := m.validator.validate(m.passwordInput)
			if err != nil {
				m.err = err
				return m, nil
			}
			m.done = true
			return m, tea.Quit
		case "backspace":
			if len(m.passwordInput) > 0 {
				m.passwordInput = m.passwordInput[:len(m.passwordInput)-1]
			}
		default:
			m.passwordInput += msg.Key().Text
			m.err = nil
		}
	case tea.PasteMsg:
		m.passwordInput += msg.String()
		m.err = nil
	case error:
		m.err = msg
		return m, nil
	case tickMsg:
		m.cursorVisible = !m.cursorVisible
		return m, tickEvery(500 * time.Millisecond)
	}
	return m, nil
}

func (m *passwordModel) View() tea.View {
	if m.done {
		return tea.NewView(fmt.Sprintf("%s%s%s\n\n", lipgloss.NewStyle().Foreground(greenColor).Render(m.promptLabel),
			lipgloss.NewStyle().Faint(true).Render(": "), strings.Repeat("*", len(m.passwordInput))))
	}

	var builder strings.Builder
	builder.WriteString(lipgloss.NewStyle().Foreground(cyanColor).Render(m.promptLabel))
	builder.WriteString(": ")
	for i := 0; i < len(m.passwordInput); i++ {
		builder.WriteByte('*')
	}
	if !m.quit && m.cursorVisible {
		builder.WriteRune('█')
	} else {
		builder.WriteRune(' ')
	}
	builder.WriteByte('\n')
	if m.err != nil {
		builder.WriteString(lipgloss.NewStyle().Foreground(redColor).Render(m.err.Error()))
	} else if m.helpMessage != "" {
		builder.WriteString(lipgloss.NewStyle().Faint(true).Render(m.helpMessage))
	}
	return tea.NewView(builder.String())
}

func promptPassword(promptLabel, helpMessage string, validator *inputValidator) string {
	teaOpts, cancelReader := newTeaOptions(nil)
	defer cancelReader()

	m, err := tea.NewProgram(&passwordModel{
		promptLabel: promptLabel,
		helpMessage: helpMessage,
		validator:   validator,
	}, teaOpts...).Run()

	if model, ok := m.(*passwordModel); err == nil && ok {
		if model.quit {
			cleanupOnExit()
			os.Exit(0)
		}
		return model.passwordInput
	}
	toolsErrorExit("input error: %v", err)
	return ""
}

// execLocalTools execute local tools if necessary
//
// return true to quit with return code
// return false to continue ssh login
func execLocalTools(args *sshArgs) (int, bool) {
	switch {
	case args.EncSecret:
		return execEncodeSecret()
	case args.ListHosts:
		return execListHosts()
	default:
		return 0, false
	}
}
