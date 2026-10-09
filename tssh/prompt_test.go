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
	"github.com/trzsz/ssh_config"
)

func promptTestHosts() []*sshHost {
	return []*sshHost{
		{Alias: "开发环境", Host: "dev.example.com", User: "root", Port: "22", GroupLabels: "开发 华东"},
		{Alias: "数据库", Host: "db.example.com", User: "admin", Port: "2222", GroupLabels: "生产 华东"},
		{Alias: "production-europe-primary", Host: "192.168.100.22", GroupLabels: "生产 欧洲"},
	}
}

func pressPrompt(p *sshPrompt, key string) tea.Cmd {
	msg := tea.KeyPressMsg{}
	switch key {
	case "f2":
		msg.Code = tea.KeyF2
	case "f3":
		msg.Code = tea.KeyF3
	case "enter":
		msg.Code = tea.KeyEnter
	case "esc":
		msg.Code = tea.KeyEscape
	case "backspace":
		msg.Code = tea.KeyBackspace
	case "up":
		msg.Code = tea.KeyUp
	case "down":
		msg.Code = tea.KeyDown
	case "right":
		msg.Code = tea.KeyRight
	case "left":
		msg.Code = tea.KeyLeft
	case "end":
		msg.Code = tea.KeyEnd
	case "home":
		msg.Code = tea.KeyHome
	default:
		if strings.HasPrefix(key, "ctrl+") {
			msg.Code = []rune(key)[5]
			msg.Mod = tea.ModCtrl
		} else {
			msg.Text = key
			msg.Code = []rune(key)[0]
		}
	}
	_, cmd := p.Update(msg)
	return cmd
}

func TestPromptOnlyConfirmsWithEnter(t *testing.T) {
	p := newSSHPrompt(promptTestHosts(), "")
	for _, key := range []string{"p", "P", "t", "T", "w", "W", "ctrl+p", "ctrl+t", "ctrl+w"} {
		assert.Nil(t, pressPrompt(p, key))
		assert.Nil(t, p.selected)
	}
	pressPrompt(p, "/")
	assert.Nil(t, pressPrompt(p, "enter"))
	assert.Nil(t, p.selected)
	assert.False(t, p.search)
	assert.NotNil(t, pressPrompt(p, "enter"))
	assert.Equal(t, "开发环境", p.selected.Alias)
}

func TestPromptChineseSearchAndLockedKeywords(t *testing.T) {
	p := newSSHPrompt(promptTestHosts(), "生产")
	require.Len(t, p.visible, 2)
	pressPrompt(p, "/")
	pressPrompt(p, "华东")
	require.Len(t, p.visible, 1)
	assert.Equal(t, "数据库", p.currentHost().Alias)
	pressPrompt(p, "enter")
	assert.Equal(t, "生产 华东", p.keywords)
	pressPrompt(p, "/")
	pressPrompt(p, "不存在")
	assert.Empty(t, p.visible)
	assert.Nil(t, pressPrompt(p, "enter"))
	assert.True(t, p.search)
	pressPrompt(p, "esc")
	require.Len(t, p.visible, 1)
	assert.Equal(t, "生产 华东", p.keywords)
	pressPrompt(p, "ctrl+e")
	assert.Len(t, p.visible, 3)
	assert.Empty(t, p.keywords)
}

func TestPromptSearchEditingAndPaste(t *testing.T) {
	p := newSSHPrompt(promptTestHosts(), "")
	pressPrompt(p, "/")
	pressPrompt(p, "数据酷")
	pressPrompt(p, "backspace")
	assert.Equal(t, "数据", p.query)
	pressPrompt(p, "库")
	require.Len(t, p.visible, 1)
	pressPrompt(p, "ctrl+e")
	pressPrompt(p, "/")
	p.Update(tea.PasteMsg{Content: "生产\n华东\x00"})
	assert.Equal(t, "生产 华东", p.query)
	require.Len(t, p.visible, 1)
	assert.Nil(t, p.selected)
	pressPrompt(p, "q")
	assert.False(t, p.quit, "q must be entered as a search keyword")
	assert.NotNil(t, pressPrompt(p, "ctrl+q"))
	assert.True(t, p.quit)
}

func TestPromptNavigationAndResize(t *testing.T) {
	hosts := make([]*sshHost, 40)
	for i := range hosts {
		hosts[i] = &sshHost{Alias: fmt.Sprintf("主机%02d", i)}
	}
	p := newSSHPrompt(hosts, "")
	p.Update(tea.WindowSizeMsg{Width: 100, Height: 24})
	assert.Equal(t, 17, p.pageSize())
	pressPrompt(p, "right")
	assert.Equal(t, 17, p.cursor)
	pressPrompt(p, "G")
	assert.Equal(t, 39, p.cursor)
	pressPrompt(p, "down")
	assert.Equal(t, 39, p.cursor)
	p.Update(tea.WindowSizeMsg{Width: 40, Height: 12})
	assert.Equal(t, 39, p.cursor)
	assert.Contains(t, ansi.Strip(p.View().Content), "主机39")
	pressPrompt(p, "home")
	pressPrompt(p, "up")
	assert.Equal(t, 0, p.cursor)
}

func TestPromptResponsiveChineseLayout(t *testing.T) {
	for _, size := range [][2]int{{120, 30}, {100, 24}, {80, 24}, {60, 16}, {40, 12}, {20, 8}, {8, 4}, {1, 1}} {
		t.Run(fmt.Sprint(size), func(t *testing.T) {
			p := newSSHPrompt(promptTestHosts(), "")
			p.Update(tea.WindowSizeMsg{Width: size[0], Height: size[1]})
			for _, help := range []bool{false, true} {
				p.showShortcuts = help
				view := p.View()
				assert.True(t, view.AltScreen)
				lines := strings.Split(view.Content, "\n")
				assert.LessOrEqual(t, len(lines), size[1])
				for _, line := range lines {
					assert.Equal(t, size[0], ansi.StringWidth(line))
				}
			}
			p.showShortcuts = false
			plain := ansi.Strip(p.View().Content)
			if size[0] >= 76 {
				assert.Contains(t, plain, "连接详情")
				assert.Contains(t, plain, "root")
			}
			if size[0] >= 40 {
				assert.Contains(t, plain, "开发环境")
			}
			assert.NotContains(t, plain, "SSH Details")
		})
	}
}

func TestPromptEmptyAndHelpStates(t *testing.T) {
	p := newSSHPrompt(nil, "")
	assert.Contains(t, ansi.Strip(p.View().Content), "暂无可用主机")
	assert.Nil(t, pressPrompt(p, "enter"))
	p = newSSHPrompt(promptTestHosts(), "missing")
	assert.Contains(t, ansi.Strip(p.View().Content), "未找到匹配的主机")
	assert.Nil(t, pressPrompt(p, "enter"))
	pressPrompt(p, "ctrl+e")
	pressPrompt(p, "?")
	assert.Contains(t, ansi.Strip(p.View().Content), "快捷键说明")
	assert.Nil(t, pressPrompt(p, "enter"), "help must not start a connection")
	pressPrompt(p, "end")
	assert.Contains(t, ansi.Strip(p.View().Content), "关闭帮助")
	assert.Equal(t, 0, p.cursor)
	pressPrompt(p, "esc")
	assert.False(t, p.showShortcuts)
}

func TestPromptHostColumnsStayAligned(t *testing.T) {
	for _, width := range []int{38, 50, 70} {
		for _, host := range promptTestHosts() {
			row := promptHostColumns(host.Alias, host.Host, width)
			assert.Equal(t, width, ansi.StringWidth(row))
			assert.Contains(t, row, host.Host)
		}
	}
	first := truncatePromptMiddle("production-europe-primary", 15)
	second := truncatePromptMiddle("production-europe-backup", 15)
	assert.NotEqual(t, first, second)
	assert.Contains(t, first, "…")
}

func TestConsoleChineseAndResize(t *testing.T) {
	m := initMenuModel(60, 80)
	m.items = []*menuItem{{key: ".", label: getText("console/terminate")}}
	assert.Contains(t, ansi.Strip(m.View().Content), "会话控制台")
	assert.Contains(t, ansi.Strip(m.View().Content), "断开当前 SSH 会话")
	m.Update(tea.WindowSizeMsg{Width: 30, Height: 20})
	assert.Equal(t, 30, m.menuWidth)
	assert.Len(t, m.items, 1)
	assert.NotPanics(t, func() { initMenuModel(0, 0).View() })
}

func TestPromptHelpPreservesSearch(t *testing.T) {
	p := newSSHPrompt(promptTestHosts(), "")
	pressPrompt(p, "/")
	pressPrompt(p, "华东")
	pressPrompt(p, "?")
	pressPrompt(p, "esc")
	assert.False(t, p.showShortcuts)
	assert.True(t, p.search)
	assert.Equal(t, "华东", p.query)
	pressPrompt(p, "esc")
	assert.False(t, p.search)
	assert.Len(t, p.visible, 3)
}

func TestPromptDetailsScrolling(t *testing.T) {
	p := newSSHPrompt(promptTestHosts(), "")
	p.Update(tea.WindowSizeMsg{Width: 100, Height: 24})
	pressPrompt(p, "f3")
	lines := p.detailLines()
	require.Greater(t, len(lines), p.pageSize())
	pressPrompt(p, "v")
	assert.True(t, p.detailFocus)
	pressPrompt(p, "right")
	assert.Equal(t, p.pageSize(), p.detailOffset)
	assert.Equal(t, 0, p.cursor)
	pressPrompt(p, "end")
	assert.Equal(t, len(lines)-p.pageSize(), p.detailOffset)
	assert.Contains(t, ansi.Strip(p.View().Content), ansi.Strip(lines[len(lines)-1]))
	pressPrompt(p, "down")
	assert.Equal(t, len(lines)-p.pageSize(), p.detailOffset)
	pressPrompt(p, "home")
	assert.Zero(t, p.detailOffset)
	pressPrompt(p, "esc")
	pressPrompt(p, "down")
	assert.Equal(t, 1, p.cursor)
	assert.Zero(t, p.detailOffset)
	pressPrompt(p, "v")
	pressPrompt(p, "end")
	p.Update(tea.WindowSizeMsg{Width: 40, Height: 12})
	narrow := p.View().Content
	assert.Contains(t, ansi.Strip(narrow), "连接详情")
	for _, line := range strings.Split(narrow, "\n") {
		assert.Equal(t, 40, ansi.StringWidth(line))
	}
	pressPrompt(p, "home")
	assert.Contains(t, ansi.Strip(p.View().Content), "数据库")
	pressPrompt(p, "v")
	assert.False(t, p.detailFocus)
	assert.Contains(t, ansi.Strip(p.View().Content), "主机名称")
}

func TestPromptConfiguredDetailsAndToggle(t *testing.T) {
	main, err := ssh_config.Decode(strings.NewReader(`Host *
    User deploy
    Port 22
Host 开发环境
    HostName dev.example.com
`))
	require.NoError(t, err)
	extra, err := ssh_config.Decode(strings.NewReader(`Host *
    EnableZmodem no
    Password secret-value
`))
	require.NoError(t, err)
	for _, width := range []int{100, 40} {
		t.Run(fmt.Sprint(width), func(t *testing.T) {
			p := newSSHPrompt(promptTestHosts(), "")
			p.config = &tsshConfig{config: main, exConfig: extra}
			p.Update(tea.WindowSizeMsg{Width: width, Height: 24})
			pressPrompt(p, "f2")
			configured := ansi.Strip(strings.Join(p.detailLines(), "\n"))
			assert.Contains(t, configured, "主机别名")
			assert.Contains(t, configured, "Port（配置）")
			assert.Contains(t, configured, "User（配置）")
			assert.Contains(t, configured, "EnableZmodem（扩展配置）")
			assert.Contains(t, configured, "Password（扩展配置）")
			assert.NotContains(t, configured, "secret-value")
			assert.NotContains(t, configured, "默认")
			assert.NotContains(t, configured, "未设置")
			assert.Contains(t, ansi.Strip(p.View().Content), "a 全部")

			pressPrompt(p, "f3")
			all := ansi.Strip(strings.Join(p.detailLines(), "\n"))
			assert.Contains(t, all, "默认")
			assert.Contains(t, all, "未设置")
			pressPrompt(p, "end")
			require.Positive(t, p.detailOffset)
			pressPrompt(p, "f3")
			assert.Zero(t, p.detailOffset)
			assert.Equal(t, configured, ansi.Strip(strings.Join(p.detailLines(), "\n")))

			pressPrompt(p, "esc")
			pressPrompt(p, "down")
			assert.NotContains(t, ansi.Strip(strings.Join(p.detailLines(), "\n")), "默认")
			pressPrompt(p, "/")
			pressPrompt(p, "华东")
			pressPrompt(p, "f3")
			assert.True(t, p.showAllConfig)
			assert.True(t, p.search)
			assert.Equal(t, "华东", p.query)
			pressPrompt(p, "?")
			pressPrompt(p, "f3")
			assert.True(t, p.showAllConfig, "help must not toggle config visibility")
		})
	}
}

func TestPromptDetailsNeverTruncateValues(t *testing.T) {
	value := strings.Repeat("长路径/", 30) + "文件末尾.pem"
	entries := []promptConfigEntry{{"IdentityFile", value, "配置"}, {"UnsetOption", "未设置", "默认"}}
	lines := promptDetails(entries, 32)
	var values []string
	for _, line := range lines {
		assert.LessOrEqual(t, ansi.StringWidth(line), 32)
		if strings.HasPrefix(line, "  ") {
			values = append(values, strings.TrimPrefix(line, "  "))
		}
	}
	assert.Contains(t, strings.Join(values, ""), value)
	assert.Contains(t, ansi.Strip(strings.Join(lines, "\n")), "未设置")
}
