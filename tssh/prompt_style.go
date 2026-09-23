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
)

type promptStyle struct {
	Help      string
	Label     string
	Active    string
	Inactive  string
	Details   string
	Shortcuts string
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
