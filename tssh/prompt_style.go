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
	"strings"
	"sync/atomic"

	"github.com/charmbracelet/x/ansi"
)

type promptStyle struct {
	Help      string
	Label     string
	Active    string
	Inactive  string
	Details   string
	Shortcuts string
}

type promptLayout struct {
	pageSize      int
	leftWidth     int
	rightWidth    int
	showShortcuts atomic.Bool
}

func newPromptLayout(width, pageSize int) *promptLayout {
	const separatorWidth = 3
	usableWidth := width - separatorWidth
	if usableWidth < 2 {
		usableWidth = 2
	}

	leftWidth := usableWidth * 45 / 100
	if usableWidth >= 50 && leftWidth < 24 {
		leftWidth = 24
	}
	if leftWidth < 1 {
		leftWidth = 1
	}
	if leftWidth >= usableWidth {
		leftWidth = usableWidth - 1
	}

	return &promptLayout{
		pageSize:   pageSize,
		leftWidth:  leftWidth,
		rightWidth: usableWidth - leftWidth,
	}
}

func promptColor(code, value string) string {
	if value == "" {
		return ""
	}
	return "\033[" + code + "m" + value + "\033[0m"
}

func fitPromptText(value string, width int) string {
	if width <= 0 {
		return ""
	}
	if ansi.StringWidth(value) > width {
		value = ansi.Truncate(value, width, "")
	}
	return value + strings.Repeat(" ", width-ansi.StringWidth(value))
}

func renderPromptHost(host *sshHost, active bool) string {
	if host == nil {
		return ""
	}

	prefix := "   "
	alias := promptColor("36", host.Alias)
	hostName := promptColor("35", host.Host)
	group := promptColor("34", host.GroupLabels)
	if active {
		prefix = promptColor("1;32", promptCursorIcon) + " "
		alias = promptColor("1;36", host.Alias)
		hostName = promptColor("1;35", host.Host)
		group = promptColor("1;34", host.GroupLabels)
	}

	line := prefix + alias
	if host.Host != "" {
		line += " (" + hostName + ")"
	}
	if host.GroupLabels != "" {
		line += "  " + group
	}
	return line
}

func promptDetailLine(label, value string) string {
	if value == "" {
		return ""
	}
	return promptColor("2", label+":") + " " + value
}

func (l *promptLayout) renderDetails(host *sshHost) []string {
	lines := make([]string, l.pageSize)
	if l.pageSize == 0 {
		return lines
	}
	if l.showShortcuts.Load() {
		shortcuts := []string{promptColor("1;34", "All Shortcuts")}
		for _, shortcut := range normalShortcuts {
			keys := append([]string{}, shortcut.globalKeys...)
			keys = append(keys, shortcut.nonSearchKeys...)
			keys = append(keys, shortcut.searchKeys...)
			shortcuts = append(shortcuts,
				strings.TrimSpace(shortcut.actionName)+": "+strings.Join(keys, "  "))
		}
		for row, shortcut := range shortcuts {
			if row >= l.pageSize {
				break
			}
			lines[row] = promptColor("2", shortcut)
			if row == 0 {
				lines[row] = shortcut
			}
		}
		return lines
	}

	instructions := []string{
		promptColor("1;34", "Instructions"),
		"↑/k  Previous   ↓/j  Next",
		"←/h  Page up    →/l  Page down",
		"/    Search     Enter Connect",
		"g/G  First/Last q     Quit",
		"?    All shortcuts",
	}
	instructionStart := l.pageSize - len(instructions)
	if instructionStart < 0 {
		instructionStart = 0
	}

	details := []string{promptColor("1;34", "SSH Details")}
	if host == nil {
		details = append(details, "Select a machine")
	} else {
		details = append(details,
			promptDetailLine("Alias", host.Alias),
			promptDetailLine("Host", host.Host),
			promptDetailLine("Port", host.Port),
			promptDetailLine("User", host.User),
			promptDetailLine("Group", host.GroupLabels),
			promptDetailLine("Identity", host.IdentityFile),
			promptDetailLine("ProxyCommand", host.ProxyCommand),
			promptDetailLine("ProxyJump", host.ProxyJump),
			promptDetailLine("RemoteCommand", host.RemoteCommand),
		)
	}

	row := 0
	for _, detail := range details {
		if detail == "" {
			continue
		}
		if row >= instructionStart {
			break
		}
		lines[row] = detail
		row++
	}
	for i, instruction := range instructions {
		row := instructionStart + i
		if row >= l.pageSize {
			break
		}
		lines[row] = promptColor("2", instruction)
		if i == 0 {
			lines[row] = instruction
		}
	}
	return lines
}

func (l *promptLayout) render(items []interface{}, active int) string {
	if len(items) == 0 {
		return ""
	}

	var activeHost *sshHost
	if active >= 0 && active < len(items) {
		activeHost, _ = items[active].(*sshHost)
	}
	rightLines := l.renderDetails(activeHost)
	lines := make([]string, 0, l.pageSize)
	for row := 0; row < l.pageSize; row++ {
		left := ""
		if row < len(items) {
			host, _ := items[row].(*sshHost)
			left = renderPromptHost(host, row == active)
		}
		lines = append(lines,
			fitPromptText(left, l.leftWidth)+promptColor("2", " │ ")+fitPromptText(rightLines[row], l.rightWidth))
	}
	return strings.Join(lines, "\n")
}

func getPromptDetailsTemplate() string {
	var builder strings.Builder
	builder.WriteString(`{{ "--------- SSH Details ----------\n" | default }}`)
	addItem := func(name string) {
		fmt.Fprintf(&builder, `{{ if hasField . "%s" }}`+
			`{{- if .%s }}{{ "%s:" | faint }}{{ "\t" }}{{ .%s | default }}{{ "\n" }}{{ end }}`+
			`{{ else }}{{ $value := getExConfig .Alias "%s" }}`+
			`{{- if $value }}{{ "%s:" | faint }}{{ "\t" }}{{ $value | default }}{{ "\n" }}{{ end }}`+
			`{{ end }}`,
			name, name, name, name, name, name)
	}
	addItem("Alias")
	addItem("Host")
	builder.WriteString(`{{- if ne .Port "22" }}{{ "Port:" | faint }}{{ "\t" }}{{ .Port | default }}{{ "\n" }}{{ end }}`)
	addItem("User")
	addItem("GroupLabels")
	addItem("IdentityFile")
	addItem("ProxyCommand")
	addItem("ProxyJump")
	addItem("RemoteCommand")
	return builder.String()
}

func getPromptStyle() *promptStyle {
	return &promptStyle{
		Label: `{{ "? " | blue }}{{ . | default }}{{ ":" | default }}`,
		Active: fmt.Sprintf(`{{ "%s" | green | bold }} `+
			`{{ .Alias | cyan | bold }} ({{ .Host | magenta | bold }}){{ "\t" }}{{ .GroupLabels | blue | bold }}`,
			promptCursorIcon),
		Inactive:  `   {{ .Alias | cyan }} ({{ .Host | magenta }}){{ "\t" }}{{ .GroupLabels | blue }}`,
		Details:   getPromptDetailsTemplate(),
		Help:      `{{ "Use ← ↓ ↑ → h j k l to navigate, / toggles search, ? toggles help" | faint }}`,
		Shortcuts: `{{ . | faint }}`,
	}
}
