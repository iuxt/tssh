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

func TestPromptShortcutsDoNotIncludeMultipleSelection(t *testing.T) {
	prompt := &sshPrompt{selector: &promptui.Select{}, showShortcuts: true}
	shortcuts := strings.Join(prompt.getShortcuts(), "\n")

	for _, action := range []string{"TglSelect", "SelectAll", "SelectOpp", "Open Wins", "Open Tabs", "Open Pane"} {
		assert.NotContains(t, shortcuts, action)
	}
}

func TestCalculatePromptPageSize(t *testing.T) {
	assert.Equal(t, defaultPromptPageSize, calculatePromptPageSize(0, 5))
	assert.Equal(t, 15, calculatePromptPageSize(24, 5))
	assert.Equal(t, 1, calculatePromptPageSize(8, 5))
}

func TestPromptPageCountUsesFullScreenSize(t *testing.T) {
	prompt := &sshPrompt{
		selector: &promptui.Select{Size: 15},
		hosts:    make([]*sshHost, 31),
	}
	assert.Equal(t, 3, prompt.getPageCount())
}

func TestPromptDetailRows(t *testing.T) {
	host := &sshHost{Alias: "dev", Host: "dev.example.com", Port: "22", User: "root"}
	assert.Equal(t, 4, getPromptDetailRows(host))

	host.Port = "2222"
	host.ProxyJump = "gateway"
	assert.Equal(t, 6, getPromptDetailRows(host))
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
