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

	leftWidth := usableWidth * 56 / 100
	if usableWidth >= 50 && leftWidth < 28 {
		leftWidth = 28
	}
	if leftWidth < 1 {
		leftWidth = 1
	}
	if leftWidth >= usableWidth {
		leftWidth = usableWidth - 1
	}
	if usableWidth >= 55 && usableWidth-leftWidth < 30 {
		leftWidth = usableWidth - 30
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

func (l *promptLayout) hostColumnWidths() (int, int) {
	// Reserve three cells for the cursor and two between the columns.
	available := l.leftWidth - 5
	if available < 2 {
		return 1, 1
	}
	nameWidth := available * 55 / 100
	if nameWidth < 12 {
		nameWidth = 12
	}
	if nameWidth >= available {
		nameWidth = available - 1
	}
	return nameWidth, available - nameWidth
}

func truncatePromptMiddle(value string, width int) string {
	valueWidth := ansi.StringWidth(value)
	if valueWidth <= width {
		return value
	}
	if width < 4 {
		return ansi.Truncate(value, width, "…")
	}
	firstWidth := width / 2
	lastWidth := width - firstWidth - 1
	return ansi.Truncate(value, firstWidth, "") + "…" +
		ansi.TruncateLeft(value, valueWidth-lastWidth, "")
}

func (l *promptLayout) renderHostColumns(name, address, nameColor, addressColor string) string {
	nameWidth, addressWidth := l.hostColumnWidths()
	name = truncatePromptMiddle(name, nameWidth)
	address = ansi.Truncate(address, addressWidth, "…")
	name = promptColor(nameColor, name)
	address = promptColor(addressColor, address)
	return fitPromptText(name, nameWidth) + "  " + fitPromptText(address, addressWidth)
}

func (l *promptLayout) renderPromptHost(host *sshHost, active bool) string {
	if host == nil {
		return ""
	}

	prefix := "   "
	nameColor, addressColor := "36", "35"
	if active {
		prefix = promptColor("1;32", promptCursorIcon) + " "
		nameColor, addressColor = "1;36", "1;35"
	}
	return prefix + l.renderHostColumns(host.Alias, host.Host, nameColor, addressColor)
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
		shortcuts := []string{}
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

	details := []string{}
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
	lines := make([]string, 0, l.pageSize+1)
	leftHeading := "   " + l.renderHostColumns("NAME", "IP / HOST", "2", "2")
	rightHeading := "SSH Details"
	if l.showShortcuts.Load() {
		rightHeading = "All Shortcuts"
	}
	lines = append(lines,
		fitPromptText(leftHeading, l.leftWidth)+promptColor("2", " │ ")+
			fitPromptText(promptColor("1;34", rightHeading), l.rightWidth))
	for row := 0; row < l.pageSize; row++ {
		left := ""
		if row < len(items) {
			host, _ := items[row].(*sshHost)
			left = l.renderPromptHost(host, row == active)
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
