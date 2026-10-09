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

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
)

const (
	promptAccent = "1;36"
	promptMuted  = "2"
	promptActive = "1;30;46"
)

func promptColor(code, value string) string {
	if value == "" {
		return ""
	}
	return "\033[" + code + "m" + value + "\033[0m"
}

func promptPanelTitle(value string, width int, focused bool) string {
	style := promptMuted
	marker := "○ "
	if focused {
		style = promptAccent
		marker = "● "
	}
	return fitPromptText(promptColor(style, marker+value), width)
}

func promptShortcut(key, label string) string {
	return promptColor("1", key) + promptColor(promptMuted, " "+label)
}

func fitPromptText(value string, width int) string {
	if width <= 0 {
		return ""
	}
	value = ansi.Truncate(value, width, "…")
	return value + strings.Repeat(" ", max(0, width-ansi.StringWidth(value)))
}

func truncatePromptMiddle(value string, width int) string {
	valueWidth := ansi.StringWidth(value)
	if valueWidth <= width {
		return value
	}
	if width < 4 {
		return ansi.Truncate(value, max(0, width), "…")
	}
	firstWidth := width / 2
	lastWidth := width - firstWidth - 1
	return ansi.Truncate(value, firstWidth, "") + "…" + ansi.TruncateLeft(value, valueWidth-lastWidth, "")
}

func promptHostColumns(name, address string, width int) string {
	if width < 30 {
		return fitPromptText(name, width)
	}
	nameWidth := (width - 2) / 2
	return fitPromptText(truncatePromptMiddle(name, nameWidth), nameWidth) + "  " +
		fitPromptText(address, width-nameWidth-2)
}

func promptDetails(entries []promptConfigEntry, width int) []string {
	if width <= 0 {
		return nil
	}
	if len(entries) == 0 {
		return []string{promptColor(promptMuted, "选择主机后查看连接信息")}
	}
	var lines []string
	labelWidth := 0
	for _, entry := range entries {
		label := entry.key
		if entry.source != "" {
			label += "（" + entry.source + "）"
		}
		labelWidth = max(labelWidth, min(ansi.StringWidth(label), width/2))
	}
	for _, entry := range entries {
		label := entry.key
		if entry.source != "" {
			label += "（" + entry.source + "）"
		}
		// Keep short values in aligned columns; long and multiline values still
		// wrap in full beneath their labels so scrolling can reach every byte.
		if width >= 40 && ansi.StringWidth(label) <= labelWidth && !strings.Contains(entry.value, "\n") &&
			ansi.StringWidth(entry.value) <= width-labelWidth-2 {
			lines = append(lines, promptColor(promptMuted, fitPromptText(label, labelWidth))+"  "+entry.value)
			continue
		}
		// Every value is wrapped, never truncated; scrolling can reach its end.
		for _, line := range strings.Split(ansi.Hardwrap(label, width, true), "\n") {
			lines = append(lines, promptColor(promptMuted, line))
		}
		for _, line := range strings.Split(ansi.Hardwrap(entry.value, max(1, width-2), true), "\n") {
			lines = append(lines, "  "+line)
		}
	}
	return lines
}

func (p *sshPrompt) panelWidths() (int, int) {
	width := max(1, p.width)
	if width < 76 {
		return width, 0
	}
	left := (width - 3) * 55 / 100
	return left, width - left - 3
}

func (p *sshPrompt) detailLines() []string {
	host := p.currentHost()
	if p.detailCache == nil {
		p.detailCache = make(map[*sshHost][]promptConfigEntry)
	}
	entries, ok := p.detailCache[host]
	if !ok {
		entries = getPromptConfigEntries(host, p.config)
		p.detailCache[host] = entries
	}
	if !p.showAllConfig {
		configured := make([]promptConfigEntry, 0, len(entries))
		for _, entry := range entries {
			if entry.source != "默认" {
				configured = append(configured, entry)
			}
		}
		entries = configured
	}
	_, width := p.panelWidths()
	if width == 0 && p.detailFocus {
		width = max(1, p.width)
	}
	return promptDetails(entries, width)
}

func (p *sshPrompt) detailHeading(lines int) string {
	prefix := "连接详情"
	if p.showAllConfig {
		prefix += "·全部"
	} else {
		prefix += "·已配置"
	}
	if lines == 0 {
		return prefix
	}
	start := min(p.detailOffset, max(0, lines-p.pageSize()))
	return fmt.Sprintf("%s %d–%d/%d", prefix, start+1, min(start+p.pageSize(), lines), lines)
}

func promptHelp(width int) []string {
	entries := []string{
		"管理配置  n / N 新增主机；e / E 编辑选中主机（浏览时）；Ctrl+S 保存，Esc 取消",
		"填写表单  Tab / Shift+Tab 或 ↑↓ 切换字段；←→ / Home / End 移动输入光标；Ctrl+U 清空字段",
		"多值配置  私钥与端口转发可用 Ctrl+N 添加一行，也可粘贴多行；留空使用继承值或默认值",
		"切换面板  v / V 切换左右面板；亮色 ● 标题表示当前操作区域，另一侧显示 ○；窄屏时切换列表 / 详情页",
		"全部配置  a / A 切换已配置项 / 全部配置；默认只显示已配置项",
		"滚动详情  切到详情后，↑↓ 滚动、←→ / PageUp / PageDown 翻页、Home / End 首尾，Esc 返回",
		"配置来源  配置 / 扩展配置表示已填写的值；全部配置还包含程序默认值和未设置的字段",
		"确认连接  Enter（搜索时先锁定关键词，再按一次连接）",
		"退出界面  Ctrl+C / Ctrl+Q；浏览时也可按 q / Q",
		"上一主机  ↑ / Shift+Tab / Ctrl+K；浏览时也可按 k / K",
		"下一主机  ↓ / Tab / Ctrl+J；浏览时也可按 j / J",
		"向上翻页  ← / PageUp / Ctrl+H / Ctrl+U / Ctrl+B；浏览时也可按 h / u / b（含大写）",
		"向下翻页  → / PageDown / Ctrl+L / Ctrl+D / Ctrl+F；浏览时也可按 l / d / f（含大写）",
		"首尾主机  Home / End；浏览时也可按 g / G",
		"搜索主机  / 开始搜索；匹配别名、地址和分组标签，空格分隔多个关键词",
		"锁定筛选  Enter 锁定关键词；再按 / 可继续添加条件",
		"取消输入  Esc / / 取消当前输入，保留已锁定的筛选条件",
		"清空筛选  Ctrl+E",
		"关闭帮助  ? / Esc；帮助较长时用 ↑ / ↓ 或翻页键滚动",
	}
	return strings.Split(ansi.Hardwrap(strings.Join(entries, "\n\n"), max(1, width), true), "\n")
}

func (p *sshPrompt) View() tea.View {
	if p.quit || p.selected != nil {
		return tea.NewView("")
	}
	if p.editor != nil {
		return p.editorView()
	}
	width, rows := max(1, p.width), p.pageSize()
	title := promptColor(promptAccent, "tssh  主机连接")
	count := fmt.Sprintf("%d / %d 台主机", len(p.visible), len(p.hosts))
	gap := width - ansi.StringWidth(title) - ansi.StringWidth(count)
	if gap > 0 {
		title += strings.Repeat(" ", gap) + promptColor(promptMuted, count)
	}
	search := promptColor(promptMuted, "/ 搜索别名、地址或分组标签")
	if p.keywords != "" {
		search = "筛选：" + promptColor(promptAccent, p.keywords) + promptColor(promptMuted, "  Ctrl+E 清空")
	}
	if p.search {
		search = "搜索：" + p.query + promptColor(promptAccent, "▏")
		if p.keywords != "" {
			search += promptColor(promptMuted, "  已锁定："+p.keywords)
		}
	}
	lines := []string{title, search, ""}
	if p.notice != "" {
		lines[2] = p.notice
	}
	configShortcut := promptShortcut("a", "全部")
	if p.showAllConfig {
		configShortcut = promptShortcut("a", "已配置")
	}
	footer := strings.Join([]string{promptShortcut("n", "新增"), promptShortcut("e", "编辑"),
		promptShortcut("v", "切换面板"), configShortcut, promptShortcut("/", "搜索"),
		promptShortcut("Enter", "连接"), promptShortcut("?", "帮助"), promptShortcut("q", "退出")}, "  ")
	if width < 76 {
		footer = strings.Join([]string{promptShortcut("n", "新增"), promptShortcut("e", "编辑"),
			promptShortcut("v", "切换"), promptShortcut("/", "搜索"), promptShortcut("↵", "连接"),
			promptShortcut("?", "帮助")}, "  ")
		if width < 50 {
			footer = strings.Join([]string{promptShortcut("n", "新增"), promptShortcut("e", "编辑"),
				promptShortcut("v", "切换"), promptShortcut("/", "搜索"), promptShortcut("?", "帮助")}, "  ")
		}
	}
	if p.search {
		footer = "↑↓ 选择  Enter 锁定  Esc 取消  Ctrl+E 清空"
		if width < 76 {
			footer = "↵锁定 Esc取消 Ctrl+E清空"
		}
	}

	if p.showShortcuts {
		lines = append(lines, promptColor(promptAccent, "快捷键说明"), promptColor(promptMuted, strings.Repeat("─", width)))
		help := promptHelp(width)
		start := min(p.helpOffset, max(0, len(help)-rows))
		for i := 0; i < rows; i++ {
			line := ""
			if start+i < len(help) {
				line = help[start+i]
			}
			lines = append(lines, line)
		}
		footer = "↑↓ 滚动  ←→ 翻页  ? / Esc 返回"
	} else if p.detailFocus && width < 76 {
		details := p.detailLines()
		lines = append(lines, promptPanelTitle(p.detailHeading(len(details)), width, true), "")
		start := min(p.detailOffset, max(0, len(details)-rows))
		for row := 0; row < rows; row++ {
			line := ""
			if start+row < len(details) {
				line = details[start+row]
			}
			lines = append(lines, line)
		}
	} else {
		leftWidth, rightWidth := p.panelWidths()
		join := func(left, right string) string {
			if rightWidth == 0 {
				return fitPromptText(left, leftWidth)
			}
			return fitPromptText(left, leftWidth) + promptColor(promptMuted, " │ ") + fitPromptText(right, rightWidth)
		}
		details := p.detailLines()
		detailStart := min(p.detailOffset, max(0, len(details)-rows))
		lines = append(lines, join(promptPanelTitle("主机列表", leftWidth, !p.detailFocus),
			promptPanelTitle(p.detailHeading(len(details)), rightWidth, p.detailFocus)))
		heading := "   " + promptHostColumns("主机名称", "地址", leftWidth-3)
		lines = append(lines, join(promptColor(promptMuted, heading), promptColor(promptMuted, "配置项 / 值")))
		start := p.cursor / rows * rows
		for row := 0; row < rows; row++ {
			left, right := "", ""
			index := start + row
			if index < len(p.visible) {
				host := p.visible[index]
				prefix := "   "
				if index == p.cursor {
					prefix = fitPromptText(promptCursorIcon, 2) + " "
					if p.detailFocus {
						prefix = fitPromptText(promptSelectedIcon, 2) + " "
					}
				}
				left = prefix + promptHostColumns(host.Alias, host.Host, leftWidth-3)
				if index == p.cursor {
					if p.detailFocus {
						left = promptColor("1", left)
					} else {
						left = promptColor(promptAccent, left)
					}
				}
			} else if len(p.visible) == 0 {
				if row == 0 {
					left = "未找到匹配的主机"
					if len(p.hosts) == 0 {
						left = "暂无可用主机"
					}
				}
				if row == 1 {
					left = "按 Ctrl+E 清空筛选，或按 / 重新搜索"
					if len(p.hosts) == 0 {
						left = "按 n 新增主机配置"
					}
				}
			}
			if detailStart+row < len(details) {
				right = details[detailStart+row]
			}
			lines = append(lines, join(left, right))
		}
		if len(p.visible) > 0 && !p.search && width >= 76 {
			footer = fmt.Sprintf("%d/%d 页  ", p.cursor/rows+1, (len(p.visible)+rows-1)/rows) + footer
		}
	}
	if p.detailFocus && !p.showShortcuts {
		footer = strings.Join([]string{promptShortcut("v", "切换面板"), promptShortcut("Esc", "返回列表"),
			configShortcut, promptShortcut("↑↓", "滚动"), promptShortcut("←→", "翻页"), promptShortcut("Home/End", "首尾")}, "  ")
		if width < 76 {
			actions := []string{promptShortcut("v", "切换"), configShortcut, promptShortcut("↑↓", "滚动")}
			if width >= 50 {
				actions = append(actions, promptShortcut("←→", "翻页"))
			}
			footer = strings.Join(append(actions, promptShortcut("?", "帮助")), "  ")
		}
	}
	lines = append(lines, "", footer)
	// Extremely small terminals still receive a bounded, useful view.
	if p.height < promptChromeRows+1 {
		lines = []string{title, search}
		if host := p.currentHost(); host != nil {
			lines = append(lines, promptCursorIcon+" "+host.Alias+"  "+host.Host)
		}
		lines = append(lines, "Enter 连接  / 搜索  q 退出")
	}
	lines = lines[:min(len(lines), max(1, p.height))]
	for i := range lines {
		lines[i] = fitPromptText(lines[i], width)
	}
	view := tea.NewView(strings.Join(lines, "\n"))
	view.AltScreen = true
	return view
}
