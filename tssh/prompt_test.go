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
	"strings"
	"testing"
	"text/template"

	"github.com/charmbracelet/x/ansi"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/trzsz/promptui"
)

func TestPromptOnlyConfirmsWithEnter(t *testing.T) {
	prompt := &sshPrompt{}
	assert.True(t, prompt.userConfirm([]byte{keyEnter}))

	for _, key := range []byte{'p', 'P', 't', 'T', 'w', 'W', '\x10', '\x14', '\x17'} {
		assert.False(t, prompt.userConfirm([]byte{key}), "key %q should not start a login", key)
	}

	prompt.search = true
	assert.False(t, prompt.userConfirm([]byte{keyEnter}))
}

func TestPromptShortcutsRenderInRightPanel(t *testing.T) {
	layout := newPromptLayout(100, 20)
	prompt := &sshPrompt{selector: &promptui.Select{}, layout: layout, showShortcuts: true}
	assert.Nil(t, prompt.getShortcuts())
	assert.True(t, layout.showShortcuts.Load())
	assert.True(t, prompt.selector.HideHelp)
}

func TestCalculatePromptPageSize(t *testing.T) {
	assert.Equal(t, defaultPromptPageSize, calculatePromptPageSize(0))
	assert.Equal(t, 20, calculatePromptPageSize(24))
	assert.Equal(t, 1, calculatePromptPageSize(4))
}

func TestPromptPageCountUsesFullScreenSize(t *testing.T) {
	prompt := &sshPrompt{
		selector: &promptui.Select{Size: 15},
		hosts:    make([]*sshHost, 31),
	}
	assert.Equal(t, 3, prompt.getPageCount())
}

func TestPromptTwoColumnLayout(t *testing.T) {
	layout := newPromptLayout(100, 20)
	hosts := []interface{}{
		&sshHost{Alias: "dev", Host: "dev.example.com", Port: "22", User: "root"},
		&sshHost{Alias: "prod", Host: "prod.example.com", Port: "2222"},
	}

	output := layout.render(hosts, 0)
	lines := strings.Split(output, "\n")
	require.Len(t, lines, 20)
	for _, line := range lines {
		assert.Equal(t, 100, ansi.StringWidth(line))
	}
	plain := ansi.Strip(output)
	assert.Contains(t, plain, "dev.example.com")
	assert.Contains(t, plain, "SSH Details")
	assert.Contains(t, plain, "Instructions")
}

func TestPromptStyleTemplates(t *testing.T) {
	funcMap := template.FuncMap{}
	for name, fn := range promptui.FuncMap {
		funcMap[name] = fn
	}
	funcMap["getExConfig"] = func(string, string) string { return "" }
	funcMap["hasField"] = func(any, string) bool { return true }

	style := getPromptStyle()
	for name, source := range map[string]string{
		"help": style.Help, "label": style.Label, "active": style.Active,
		"inactive": style.Inactive, "details": style.Details, "shortcuts": style.Shortcuts,
	} {
		_, err := template.New(name).Funcs(funcMap).Parse(source)
		require.NoError(t, err, name)
	}
}
