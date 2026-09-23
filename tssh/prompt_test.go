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

	"github.com/stretchr/testify/assert"
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
