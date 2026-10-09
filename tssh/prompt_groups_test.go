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
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPromptGroupMembership(t *testing.T) {
	hosts := []*sshHost{
		{Alias: "a", GroupLabels: "生产 华东 生产"},
		{Alias: "b", GroupLabels: "生产"},
		{Alias: "c"},
		{Alias: "d", GroupLabels: "未分组"},
	}
	p := newSSHPrompt(hosts, "")
	require.Len(t, p.groups, 4)
	assert.Equal(t, []promptGroup{
		{"生产", hosts[:2]}, {"华东", hosts[:1]}, {"", hosts[2:3]}, {"未分组", hosts[3:]},
	}, p.groups)
	assert.Len(t, p.rows, 9)
	assert.Same(t, hosts[0], p.currentHost())
	assert.Contains(t, ansi.Strip(p.View().Content), "4 / 4 台主机")
	assert.Contains(t, ansi.Strip(p.View().Content), "[-] 生产 (2)")
	// A real label called 未分组 must not share the unlabelled group's state.
	p.collapsed = map[string]bool{"": true}
	p.rebuildRows()
	assert.Len(t, p.rows, 8)
	p.selectAlias("c")
	assert.Same(t, hosts[2], p.currentHost())
	assert.False(t, p.collapsed[""])
}

func TestPromptGroupKeyboardAndSearch(t *testing.T) {
	p := newSSHPrompt(promptTestHosts(), "")
	pressPrompt(p, "left")
	assert.True(t, p.collapsed["开发"])
	assert.True(t, p.onGroupHeading())
	assert.Nil(t, p.currentHost())
	assert.Contains(t, ansi.Strip(p.View().Content), "[+] 开发 (1)")
	assert.Nil(t, pressPrompt(p, "enter"))
	assert.Nil(t, p.selected)
	assert.False(t, p.collapsed["开发"])
	assert.True(t, p.onGroupHeading())
	pressPrompt(p, "down")
	assert.Equal(t, "开发环境", p.currentHost().Alias)
	pressPrompt(p, "right")
	assert.Equal(t, "开发环境", p.currentHost().Alias, "expanding an open group keeps the selected host")
	pressPrompt(p, "space")
	assert.True(t, p.collapsed["开发"])
	pressPrompt(p, "right")
	assert.False(t, p.collapsed["开发"])
	pressPrompt(p, "space")
	pressPrompt(p, "/")
	pressPrompt(p, "开发")
	require.Len(t, p.visible, 1)
	assert.Equal(t, "开发环境", p.currentHost().Alias)
	assert.False(t, p.groupCollapsed("开发"))
	assert.True(t, p.collapsed["开发"], "search must not discard fold preferences")
	// Even a locked filter shows all matching hosts.
	assert.Nil(t, pressPrompt(p, "enter"))
	assert.False(t, p.search)
	pressPrompt(p, "left")
	assert.False(t, p.groupCollapsed("开发"))
	assert.True(t, p.onGroupHeading(), "filter mode keeps arrow-key pagination")
	pressPrompt(p, "down")
	assert.Equal(t, "开发环境", p.currentHost().Alias)
	pressPrompt(p, "ctrl+e")
	assert.True(t, p.groupCollapsed("开发"))
	p.selectAlias("开发环境")
	assert.False(t, p.collapsed["开发"])
	assert.NotNil(t, pressPrompt(p, "enter"))
	assert.Equal(t, "开发环境", p.selected.Alias)
}

func TestPromptCollapsedGroupNavigationAndResize(t *testing.T) {
	hosts := make([]*sshHost, 30)
	for i := range hosts {
		hosts[i] = &sshHost{Alias: fmt.Sprintf("主机%d", i), GroupLabels: fmt.Sprintf("分组%d", i/5)}
	}
	p := newSSHPrompt(hosts, "")
	p.Update(tea.WindowSizeMsg{Width: 100, Height: 12})
	pressPrompt(p, "pgdown")
	assert.Equal(t, 6, p.cursor)
	pressPrompt(p, "enter")
	assert.True(t, p.collapsed["分组1"])
	assert.Equal(t, "分组1", p.rows[p.cursor].group)
	assert.Len(t, p.rows, 31)
	pressPrompt(p, "end")
	assert.Equal(t, "主机29", p.currentHost().Alias)
	for _, group := range p.groups {
		p.collapsed[group.label] = true
	}
	p.rebuildRows()
	p.jump(true)
	assert.Equal(t, 5, p.cursor)
	assert.Nil(t, p.currentHost())
	for _, size := range [][2]int{{120, 30}, {76, 24}, {40, 12}, {24, 8}, {8, 4}, {1, 1}} {
		p.Update(tea.WindowSizeMsg{Width: size[0], Height: size[1]})
		lines := strings.Split(p.View().Content, "\n")
		assert.LessOrEqual(t, len(lines), size[1])
		for _, line := range lines {
			assert.Equal(t, size[0], ansi.StringWidth(line))
		}
	}
}

func TestPromptGroupEmptyFilterAndCancel(t *testing.T) {
	p := newSSHPrompt(promptTestHosts(), "")
	pressPrompt(p, "left")
	pressPrompt(p, "/")
	pressPrompt(p, "不存在")
	assert.Empty(t, p.visible)
	assert.Empty(t, p.rows)
	assert.Nil(t, pressPrompt(p, "enter"))
	assert.NotPanics(t, func() { p.View() })
	pressPrompt(p, "esc")
	assert.True(t, p.collapsed["开发"])
	assert.Len(t, p.visible, 3)
}

func TestPromptSaveHostIntoCollapsedGroup(t *testing.T) {
	p, _ := editorTestPrompt(t, "Host old\n GroupLabels 生产 华东\n")
	p.collapsed = map[string]bool{"生产": true, "华东": true}
	p.filterHosts()
	p.openEditor(true)
	require.NotNil(t, p.editor)
	setEditorField(t, p.editor, "Host", "new")
	setEditorField(t, p.editor, "GroupLabels", "生产 华东")
	p.saveEditor()
	require.Nil(t, p.editor)
	require.NotNil(t, p.currentHost())
	assert.Equal(t, "new", p.currentHost().Alias)
	assert.False(t, p.collapsed["生产"])
	assert.False(t, p.collapsed["华东"])
}
