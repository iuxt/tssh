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
	"encoding/base64"
	"fmt"
	"os"
	"strconv"
	"time"
)

func tmuxDebug(format string, a ...any) {
	if !enableDebugLogging {
		return
	}

	msg := fmt.Sprintf(format, a...)
	buf := fmt.Appendf(nil, "\r\033[0;36mdebug:\033[0m %s\033[K\r\n", msg)

	if iterm2Session := getIterm2Session(); iterm2Session != nil && iterm2Session.Inject(buf) == nil {
		return
	}

	_, _ = os.Stderr.Write(buf)
}

func isRunningTmuxIntegration() bool {
	iterm2Session := getIterm2Session()
	if iterm2Session == nil {
		return false
	}

	tmux, err := iterm2Session.IsTmuxIntegrationSession()
	if err != nil {
		if enableDebugLogging && !iterm2Session.GetApp().IsClosed() {
			tmuxDebug("check tmux integration failed: %v", err)
		}
		return false
	}

	return tmux
}

func logToTmuxIntegration(buf []byte, paneId string) bool {
	if paneId != "" {
		writeTmuxOutput(buf, paneId)
		return true
	}

	iterm2Session := getIterm2Session()
	if iterm2Session == nil {
		return false
	}

	cmd := fmt.Sprintf("run-shell 'echo %s | base64 -d >#{pane_tty}'", base64.StdEncoding.EncodeToString(buf))
	if _, err := iterm2Session.RunTmuxCommand(cmd, 0.3); err != nil {
		tmuxDebug("run tmux command [%s] failed: %v", cmd, err)
		return iterm2Session.Inject(buf) == nil
	}

	return true
}

func getTmuxPaneIdAndColumns() (string, int) {
	iterm2Session := getIterm2Session()
	if iterm2Session == nil {
		return "", 0
	}

	session, err := iterm2Session.GetApp().GetCurrentTmuxSession()
	if err != nil {
		tmuxDebug("get process session failed: %v", err)
		return "", 0
	}

	values, err := session.GetVariable("tmuxWindowPane", "columns")
	if err != nil {
		tmuxDebug("get session variable failed: %v", err)
		return "", 0
	}
	if len(values) != 2 {
		tmuxDebug("get session variable values count is not two: %v", values)
		return "", 0
	}

	paneId := ""
	if values[0] != "null" {
		if _, err := strconv.ParseUint(values[0], 10, 32); err != nil {
			tmuxDebug("tmux window pane id [%s] invalid: %v", values[0], err)
			return "", 0
		}
		paneId = values[0]
	}

	columns, err := strconv.ParseUint(values[1], 10, 32)
	if err != nil {
		tmuxDebug("tmux window columns [%s] invalid: %v", values[1], err)
		return paneId, 0
	}

	return paneId, int(columns)
}

func writeTmuxOutput(output []byte, paneId string) {
	buffer := bytes.NewBuffer(make([]byte, 0, 10+len(paneId)+len(output)<<2+2))
	buffer.WriteString("%output %")
	buffer.WriteString(paneId)
	buffer.WriteByte(' ')

	for _, b := range output {
		if b < ' ' || b == '\\' || b > '~' {
			fmt.Fprintf(buffer, "\\%03o", b)
		} else {
			buffer.WriteByte(b)
		}
	}
	buffer.Write([]byte("\r\n"))

	_, _ = os.Stderr.Write(buffer.Bytes())
}

func detachTmuxIntegration() {
	iterm2Session := getIterm2Session()
	if iterm2Session == nil {
		return
	}

	_, _ = doWithTimeout(func() (string, error) {
		return iterm2Session.RunTmuxCommand("detach", 0.3) // detach from tmux integration
	}, 300*time.Millisecond)

	time.Sleep(200 * time.Millisecond)

	if isRunningTmuxIntegration() {
		_, _ = os.Stderr.Write([]byte("%exit\r\n")) // force exit tmux integration
		time.Sleep(100 * time.Millisecond)
	}
}
