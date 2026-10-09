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
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/trzsz/ssh_config"
)

func editorTestPrompt(t *testing.T, content string) (*sshPrompt, string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config")
	require.NoError(t, os.WriteFile(path, []byte(content), 0600))
	previous := userConfig
	t.Cleanup(func() { userConfig = previous })
	userConfig = &tsshConfig{configPath: path}
	p := newSSHPrompt(getAllHosts(), "")
	p.config = userConfig
	return p, path
}

func setEditorField(t *testing.T, e *hostConfigEditor, key, value string) {
	t.Helper()
	for i, field := range hostEditorFields {
		if field.key == key {
			e.focus, e.caret = i, 0
			e.values[i] = ""
			if key == "Password" {
				e.passwordChanged = true
			}
			e.insert(value)
			return
		}
	}
	t.Fatalf("unknown field %s", key)
}

func TestPromptCreateHostBeforeInheritedDefaults(t *testing.T) {
	p, path := editorTestPrompt(t, "# original\nUser default-user\nPort 2222\nHost existing\n    HostName old.example\n")
	p.openEditor(true)
	require.NotNil(t, p.editor)
	setEditorField(t, p.editor, "Host", "开发新主机")
	setEditorField(t, p.editor, "HostName", "new.example")
	setEditorField(t, p.editor, "User", "ubuntu")
	setEditorField(t, p.editor, "Port", "22")
	setEditorField(t, p.editor, "IdentityFile", "~/.ssh/key1\n\"~/keys/key two\"")
	setEditorField(t, p.editor, "LocalForward", "8080 localhost:80\n9090 localhost:90")
	setEditorField(t, p.editor, "EnableOSC52", "yes")
	p.saveEditor()
	require.Nil(t, p.editor, p.notice)
	require.NotNil(t, p.currentHost())
	assert.Equal(t, "开发新主机", p.currentHost().Alias)
	assert.Equal(t, "ubuntu", getConfig("开发新主机", "User"))
	assert.Equal(t, "22", getConfig("开发新主机", "Port"))
	assert.Equal(t, "default-user", getConfig("existing", "User"))
	assert.Equal(t, "2222", getConfig("existing", "Port"))
	assert.Equal(t, []string{"~/.ssh/key1", "~/keys/key two"}, getAllConfig("开发新主机", "IdentityFile"))
	assert.Equal(t, []string{"8080 localhost:80", "9090 localhost:90"}, getAllConfig("开发新主机", "LocalForward"))
	data, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Contains(t, string(data), "Host *\n# original\nUser default-user\n")
	assert.Contains(t, ansi.Strip(p.View().Content), "已保存")
}

func TestPromptCreateMissingConfigAndDuplicateAlias(t *testing.T) {
	p, path := editorTestPrompt(t, "")
	require.NoError(t, os.Remove(path))
	p.config.configPath = filepath.Join(filepath.Dir(path), "new-directory", "config")
	p.openEditor(true)
	require.NotNil(t, p.editor)
	setEditorField(t, p.editor, "Host", "new")
	p.saveEditor()
	require.Nil(t, p.editor)
	assert.Equal(t, "new", p.currentHost().Alias)
	info, err := os.Stat(p.config.configPath)
	require.NoError(t, err)
	if runtime.GOOS != "windows" {
		assert.Equal(t, os.FileMode(0600), info.Mode().Perm())
	}
	p.openEditor(true)
	setEditorField(t, p.editor, "Host", "new")
	p.saveEditor()
	require.NotNil(t, p.editor)
	assert.Contains(t, p.editor.err, "别名已存在")
}

func TestPromptEditPreservesOtherAliasesCommentsAndMatch(t *testing.T) {
	original := "# top\r\nHost one two # shared\r\n\tHostName=old.example\r\n\tUser root # user note\r\n\tCustomOption custom # custom note\r\n\tIdentityFile ~/.ssh/one\r\n\tIdentityFile ~/.ssh/two\r\n\r\nMatch exec \"false\"\r\n  UnknownKey keep-me\r\nHost *\r\n  ServerAliveInterval 15\r\n"
	p, path := editorTestPrompt(t, original)
	p.openEditor(false)
	require.NotNil(t, p.editor)
	assert.Equal(t, "~/.ssh/one\n~/.ssh/two", p.editor.values[5])
	setEditorField(t, p.editor, "User", "ubuntu")
	setEditorField(t, p.editor, "LocalForward", "8000 localhost:80\n9000 localhost:90")
	p.saveEditor()
	require.Nil(t, p.editor, p.notice)
	assert.Equal(t, "ubuntu", getConfig("one", "User"))
	assert.Equal(t, "root", getConfig("two", "User"))
	assert.Equal(t, []string{"8000 localhost:80", "9000 localhost:90"}, getAllConfig("one", "LocalForward"))
	assert.Empty(t, getAllConfig("two", "LocalForward"))
	data, err := os.ReadFile(path)
	require.NoError(t, err)
	text := string(data)
	assert.Contains(t, text, "# user note")
	assert.Contains(t, text, "\tCustomOption custom # custom note\r\n")
	assert.Contains(t, text, "Match exec \"false\"\r\n  UnknownKey keep-me\r\n")
	assert.NotContains(t, strings.ReplaceAll(text, "\r\n", ""), "\n")
}

func TestPromptEditIncludeSource(t *testing.T) {
	p, main := editorTestPrompt(t, "")
	included := filepath.Join(filepath.Dir(main), "included")
	original := "Host included-host\n  HostName inside.example\n  User root\n"
	require.NoError(t, os.WriteFile(included, []byte(original), 0600))
	mainText := "Include \"" + filepath.ToSlash(included) + "\"\nHost outside\n HostName outside.example\n"
	require.NoError(t, os.WriteFile(main, []byte(mainText), 0600))
	userConfig = &tsshConfig{configPath: main}
	p.config = userConfig
	p.hosts = getAllHosts()
	p.filterHosts()
	require.Equal(t, "included-host", p.currentHost().Alias)
	p.openEditor(false)
	require.NotNil(t, p.editor)
	resolved, err := filepath.EvalSymlinks(included)
	require.NoError(t, err)
	assert.Equal(t, resolved, p.editor.document.path)
	setEditorField(t, p.editor, "User", "ubuntu")
	p.saveEditor()
	require.Nil(t, p.editor)
	assert.Equal(t, "ubuntu", getConfig("included-host", "User"))
	data, err := os.ReadFile(main)
	require.NoError(t, err)
	assert.Equal(t, mainText, string(data))
}

func TestPromptPasswordPreservedReplacedAndRemoved(t *testing.T) {
	encoded, err := encodeSecret([]byte("old-password"))
	require.NoError(t, err)
	p, path := editorTestPrompt(t, "Host secret-host\n  encPassword "+encoded+"\n  Password old-plain\n  PasswordCommand never-executed\n")
	p.openEditor(false)
	assert.NotContains(t, p.editorView().Content, encoded)
	assert.NotContains(t, p.editorView().Content, "old-plain")
	assert.NotContains(t, p.editorView().Content, "never-executed")
	setEditorField(t, p.editor, "User", "ubuntu")
	p.saveEditor()
	require.Nil(t, p.editor)
	data, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Contains(t, string(data), encoded)
	p.openEditor(false)
	password := " pass#with'\"symbols "
	setEditorField(t, p.editor, "Password", password)
	assert.NotContains(t, p.editorView().Content, password)
	p.saveEditor()
	require.Nil(t, p.editor)
	config := userConfig.config
	raw, err := config.Get("secret-host", "encPassword")
	require.NoError(t, err)
	decoded, err := decodeSecret(raw)
	require.NoError(t, err)
	assert.Equal(t, password, decoded)
	plain, err := config.Get("secret-host", "Password")
	require.NoError(t, err)
	assert.Empty(t, plain)
	p.openEditor(false)
	setEditorField(t, p.editor, "Password", "")
	p.saveEditor()
	require.Nil(t, p.editor)
	assert.Empty(t, getConfig("secret-host", "encPassword"))
	assert.Equal(t, "never-executed", getConfig("secret-host", "PasswordCommand"))
}

func TestPromptEditorRejectsInvalidInputAndExternalChanges(t *testing.T) {
	for _, invalid := range []struct{ key, value string }{
		{"Host", "invalid alias"}, {"Host", "wild*"}, {"Port", "65536"}, {"Port", "0"},
		{"ConnectTimeout", "-1"}, {"ServerAliveInterval", "oops"}, {"ForwardAgent", "maybe"},
		{"RequestTTY", "sometimes"}, {"StrictHostKeyChecking", "trust"},
	} {
		t.Run(invalid.key+invalid.value, func(t *testing.T) {
			original := "Host server\n HostName example.com\n"
			p, path := editorTestPrompt(t, original)
			p.openEditor(false)
			setEditorField(t, p.editor, invalid.key, invalid.value)
			p.saveEditor()
			require.NotNil(t, p.editor)
			assert.NotEmpty(t, p.editor.err)
			data, err := os.ReadFile(path)
			require.NoError(t, err)
			assert.Equal(t, original, string(data))
		})
	}
	p, path := editorTestPrompt(t, "Host server\n User root\n")
	p.openEditor(false)
	setEditorField(t, p.editor, "User", "ubuntu")
	concurrent := "Host server\n User external\n"
	require.NoError(t, os.WriteFile(path, []byte(concurrent), 0600))
	p.saveEditor()
	require.NotNil(t, p.editor)
	assert.Contains(t, p.editor.err, "其他程序修改")
	data, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Equal(t, concurrent, string(data))
}

func TestPromptEditorUnicodeKeysPasteCancelAndLayout(t *testing.T) {
	p, path := editorTestPrompt(t, "Host server\n User root\n")
	pressPrompt(p, "n")
	require.NotNil(t, p.editor)
	pressPrompt(p, "中文q/?")
	pressPrompt(p, "left")
	pressPrompt(p, "backspace")
	assert.Equal(t, "中文q?", p.editor.values[0])
	assert.False(t, p.search)
	assert.False(t, p.quit)
	assert.Nil(t, p.selected)
	setEditorField(t, p.editor, "IdentityFile", "first")
	pressPrompt(p, "ctrl+n")
	p.Update(tea.PasteMsg{Content: "second\r\nthird\x1b\x00"})
	assert.Equal(t, "first\nsecond\nthird", p.editor.values[p.editor.focus])
	for _, size := range [][2]int{{120, 30}, {80, 24}, {40, 12}, {8, 4}, {1, 1}} {
		p.Update(tea.WindowSizeMsg{Width: size[0], Height: size[1]})
		for i := range hostEditorFields {
			p.editor.focus, p.editor.caret = i, len([]rune(p.editor.values[i]))
			view := p.View()
			assert.True(t, view.AltScreen)
			lines := strings.Split(view.Content, "\n")
			assert.LessOrEqual(t, len(lines), size[1])
			for _, line := range lines {
				assert.Equal(t, size[0], ansi.StringWidth(line))
			}
			if size[0] >= 40 {
				assert.Contains(t, ansi.Strip(view.Content), hostEditorFields[i].label)
			}
		}
	}
	pressPrompt(p, "esc")
	assert.Nil(t, p.editor)
	data, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Equal(t, "Host server\n User root\n", string(data))
	pressPrompt(p, "e")
	require.NotNil(t, p.editor)
	setEditorField(t, p.editor, "User", "ubuntu")
	pressPrompt(p, "ctrl+s")
	require.Nil(t, p.editor)
	assert.Equal(t, "ubuntu", getConfig("server", "User"))
}

func TestHostConfigSymlinkAndNoOp(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink creation requires Windows privileges")
	}
	p, path := editorTestPrompt(t, "Host one two\n  User root\n")
	link := filepath.Join(filepath.Dir(path), "config-link")
	require.NoError(t, os.Symlink(path, link))
	p.config.configPath = link
	p.openEditor(false)
	require.NotNil(t, p.editor)
	content, err := p.editor.content()
	require.NoError(t, err)
	assert.Equal(t, "Host one two\n  User root\n", string(content))
	setEditorField(t, p.editor, "User", "ubuntu")
	p.saveEditor()
	require.Nil(t, p.editor)
	target, err := os.Readlink(link)
	require.NoError(t, err)
	assert.Equal(t, path, target)
	assert.Equal(t, "ubuntu", getConfig("one", "User"))
	assert.Equal(t, "root", getConfig("two", "User"))
}

func TestPromptRenameAndClearRevertsToInheritedValue(t *testing.T) {
	p, _ := editorTestPrompt(t, "Host old\n User root\nHost taken\n User another\nHost *\n User inherited\n")
	p.openEditor(false)
	setEditorField(t, p.editor, "Host", "taken")
	p.saveEditor()
	require.NotNil(t, p.editor)
	assert.Contains(t, p.editor.err, "别名已存在")
	setEditorField(t, p.editor, "Host", "renamed")
	setEditorField(t, p.editor, "User", "")
	p.saveEditor()
	require.Nil(t, p.editor)
	assert.Equal(t, "renamed", p.currentHost().Alias)
	assert.Equal(t, "inherited", getConfig("renamed", "User"))
	_, err := ssh_config.Decode(strings.NewReader(p.config.config.String()))
	require.NoError(t, err)
}

func TestPromptLetterShortcutsRespectInputAndHelp(t *testing.T) {
	p, _ := editorTestPrompt(t, "Host server\n User root\n")
	for _, key := range []string{"n", "N"} {
		pressPrompt(p, key)
		require.NotNil(t, p.editor)
		assert.Equal(t, -1, p.editor.block)
		pressPrompt(p, "esc")
	}
	p.keywords = "server"
	p.filterHosts()
	for _, key := range []string{"e", "E"} {
		pressPrompt(p, key)
		require.NotNil(t, p.editor)
		assert.Equal(t, "server", p.editor.alias)
		assert.Equal(t, "server", p.keywords)
		pressPrompt(p, "esc")
	}
	pressPrompt(p, "v")
	assert.True(t, p.detailFocus)
	pressPrompt(p, "V")
	assert.False(t, p.detailFocus)
	pressPrompt(p, "a")
	assert.True(t, p.showAllConfig)
	pressPrompt(p, "A")
	assert.False(t, p.showAllConfig)
	pressPrompt(p, "?")
	for _, key := range []string{"n", "e", "v", "a"} {
		pressPrompt(p, key)
	}
	assert.Nil(t, p.editor)
	assert.False(t, p.detailFocus)
	assert.False(t, p.showAllConfig)
	pressPrompt(p, "esc")
	pressPrompt(p, "/")
	for _, key := range []string{"n", "e", "v", "a", "N", "E", "V", "A"} {
		pressPrompt(p, key)
	}
	assert.Equal(t, "nevaNEVA", p.query)
	assert.Equal(t, "server", p.keywords)
	assert.Nil(t, p.editor)
	assert.False(t, p.detailFocus)
	assert.False(t, p.showAllConfig)
	pressPrompt(p, "ctrl+e")
	assert.Empty(t, p.keywords)
	assert.Empty(t, p.query)
}
