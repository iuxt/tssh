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
	"bytes"
	"fmt"
	"slices"
	"strings"
	"unicode"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/trzsz/ssh_config"
)

type hostEditorField struct {
	key, label, group, hint string
	multi, secret           bool
}

var hostEditorFields = []hostEditorField{
	{key: "Host", label: "主机别名", group: "基本连接", hint: "必填，支持中文；不能包含空格或通配符"},
	{key: "HostName", label: "主机地址", group: "基本连接", hint: "IP 或域名；留空使用别名"},
	{key: "User", label: "登录用户", group: "基本连接", hint: "例如 root、ubuntu；留空继承配置或使用本地用户"},
	{key: "Port", label: "SSH 端口", group: "基本连接", hint: "1–65535；默认 22"},
	{key: "GroupLabels", label: "分组标签", group: "基本连接", hint: "以空格分隔，例如 生产 华东"},
	{key: "IdentityFile", label: "私钥路径", group: "认证", hint: "例如 ~/.ssh/id_ed25519；Ctrl+N 添加多把私钥", multi: true},
	{key: "Password", label: "登录密码", group: "认证", hint: "输入替换已存密码；Ctrl+U 清除此 Host 的密码；保存为 encPassword", secret: true},
	{key: "PasswordCommand", label: "密码命令", group: "认证", hint: "连接时执行的外部密码命令；编辑时不会执行", secret: true},
	{key: "IdentitiesOnly", label: "仅指定密钥", group: "认证", hint: "yes / no；留空继承或使用默认值"},
	{key: "PreferredAuthentications", label: "认证顺序", group: "认证", hint: "例如 publickey,keyboard-interactive,password"},
	{key: "PasswordAuthentication", label: "密码认证", group: "认证", hint: "yes / no"},
	{key: "PubkeyAuthentication", label: "公钥认证", group: "认证", hint: "yes / no"},
	{key: "ProxyJump", label: "跳板机", group: "代理与转发", hint: "例如 bastion 或 user@host:22；多跳用逗号分隔"},
	{key: "ProxyCommand", label: "代理命令", group: "代理与转发", hint: "例如 ssh -W %h:%p bastion；与 ProxyJump 二选一"},
	{key: "ForwardAgent", label: "转发认证代理", group: "代理与转发", hint: "yes / no"},
	{key: "LocalForward", label: "本地端口转发", group: "代理与转发", hint: "例如 8080 localhost:80；Ctrl+N 添加多条", multi: true},
	{key: "RemoteForward", label: "远程端口转发", group: "代理与转发", hint: "例如 9000 localhost:3000；Ctrl+N 添加多条", multi: true},
	{key: "DynamicForward", label: "SOCKS 代理", group: "代理与转发", hint: "例如 127.0.0.1:1080；Ctrl+N 添加多条", multi: true},
	{key: "ExitOnForwardFailure", label: "转发失败退出", group: "代理与转发", hint: "yes / no"},
	{key: "ConnectTimeout", label: "连接超时", group: "连接行为", hint: "非负整数，单位秒"},
	{key: "ServerAliveInterval", label: "保活间隔", group: "连接行为", hint: "非负整数，单位秒；0 关闭"},
	{key: "ServerAliveCountMax", label: "保活重试次数", group: "连接行为", hint: "非负整数"},
	{key: "StrictHostKeyChecking", label: "主机密钥检查", group: "连接行为", hint: "yes / ask / accept-new / no"},
	{key: "Compression", label: "启用压缩", group: "连接行为", hint: "yes / no"},
	{key: "RequestTTY", label: "终端分配", group: "连接行为", hint: "auto / yes / no / force"},
	{key: "RemoteCommand", label: "登录后命令", group: "连接行为", hint: "例如 tmux attach；按需配置 RequestTTY force"},
	{key: "EnableTrzsz", label: "trzsz 传输", group: "扩展功能", hint: "yes / no；默认 yes"},
	{key: "EnableZmodem", label: "Zmodem 传输", group: "扩展功能", hint: "yes / no；默认 yes"},
	{key: "EnableOSC52", label: "远程剪贴板", group: "扩展功能", hint: "yes / no；默认 no"},
	{key: "DefaultUploadPath", label: "默认上传目录", group: "扩展功能", hint: "例如 ~/Downloads；含空格路径使用双引号"},
	{key: "DefaultDownloadPath", label: "默认下载目录", group: "扩展功能", hint: "例如 ~/Downloads；含空格路径使用双引号"},
}

type hostConfigEditor struct {
	document        *hostConfigDocument
	block           int // -1 means a new host
	alias           string
	values, initial []string
	focus, caret    int
	passwordChanged bool
	err             string
}

func (p *sshPrompt) openEditor(create bool) {
	p.notice = ""
	if p.config == nil || p.config.configPath == "" {
		p.notice = "没有可写的主配置路径"
		return
	}
	if !create && p.currentHost() == nil {
		p.notice = "请先选择要编辑的主机"
		return
	}
	documents, err := hostConfigDocuments(p.config.configPath)
	if err != nil {
		p.notice = "打开配置失败：" + err.Error()
		return
	}
	e := &hostConfigEditor{document: documents[0], block: -1, values: make([]string, len(hostEditorFields))}
	if !create {
		e.alias = p.currentHost().Alias
		for _, d := range documents {
			for index, block := range d.blocks {
				if slices.Contains(block.aliases, e.alias) {
					e.document, e.block = d, index
					break
				}
			}
			if e.block >= 0 {
				break
			}
		}
		if e.block < 0 {
			p.notice = "没有找到该别名的 Host 配置块；请使用 n 新增独立配置"
			return
		}
		e.values[0] = e.alias
		block := e.document.blocks[e.block]
		for i, field := range hostEditorFields[1:] {
			if field.key == "Password" {
				continue
			}
			e.values[i+1] = strings.Join(block.values[strings.ToLower(field.key)], "\n")
		}
	}
	e.initial = append([]string(nil), e.values...)
	e.caret = len([]rune(e.values[0]))
	p.editor = e
}

func (e *hostConfigEditor) value(key string) (string, string) {
	for i, field := range hostEditorFields {
		if field.key == key {
			return e.values[i], e.initial[i]
		}
	}
	return "", ""
}

func (e *hostConfigEditor) insert(text string) {
	field := hostEditorFields[e.focus]
	text = strings.Map(func(r rune) rune {
		if r == '\n' && field.multi {
			return r
		}
		if unicode.IsControl(r) {
			if unicode.IsSpace(r) {
				return ' '
			}
			return -1
		}
		return r
	}, strings.ReplaceAll(text, "\r\n", "\n"))
	if text == "" {
		return
	}
	chars := []rune(e.values[e.focus])
	e.values[e.focus] = string(chars[:e.caret]) + text + string(chars[e.caret:])
	e.caret += len([]rune(text))
	if field.key == "Password" {
		e.passwordChanged = true
	}
	e.err = ""
}

func (p *sshPrompt) updateEditor(msg tea.Msg) (tea.Model, tea.Cmd) {
	e := p.editor
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		p.width, p.height = max(1, msg.Width), max(1, msg.Height)
	case tea.PasteMsg:
		e.insert(msg.Content)
	case tea.MouseWheelMsg:
		p.wheelEditor(msg)
	case tea.KeyPressMsg:
		e.err = ""
		key := msg.String()
		switch key {
		case "ctrl+c", "ctrl+q":
			p.quit = true
			return p, tea.Quit
		case "esc":
			p.editor = nil
			p.notice = "已取消，配置未保存"
		case "ctrl+s":
			p.saveEditor()
		case "tab", "down", "enter", "shift+tab", "up":
			delta := 1
			if key == "shift+tab" || key == "up" {
				delta = -1
			}
			e.focus = (e.focus + delta + len(hostEditorFields)) % len(hostEditorFields)
			e.caret = len([]rune(e.values[e.focus]))
		case "left":
			e.caret = max(0, e.caret-1)
		case "right":
			e.caret = min(len([]rune(e.values[e.focus])), e.caret+1)
		case "home":
			e.caret = 0
		case "end":
			e.caret = len([]rune(e.values[e.focus]))
		case "ctrl+u":
			e.values[e.focus], e.caret = "", 0
			if hostEditorFields[e.focus].key == "Password" {
				e.passwordChanged = true
			}
		case "ctrl+n":
			if hostEditorFields[e.focus].multi {
				e.insert("\n")
			}
		case "backspace", "delete":
			chars := []rune(e.values[e.focus])
			index := e.caret
			if key == "backspace" {
				index--
			}
			if index >= 0 && index < len(chars) {
				e.values[e.focus] = string(chars[:index]) + string(chars[index+1:])
				e.caret = index
				if hostEditorFields[e.focus].key == "Password" {
					e.passwordChanged = true
				}
			}
		default:
			if msg.Text != "" {
				e.insert(msg.Text)
			}
		}
	}
	return p, nil
}

func (e *hostConfigEditor) content() ([]byte, error) {
	alias := strings.TrimSpace(e.values[0])
	if err := validateHostAlias(alias); err != nil {
		e.focus = 0
		e.caret = len([]rune(e.values[0]))
		return nil, err
	}
	changed := map[string][]string{}
	for i, field := range hostEditorFields[1:] {
		index := i + 1
		if field.key == "Password" {
			if !e.passwordChanged {
				continue
			}
			changed["password"], changed["encpassword"] = nil, nil
			if e.values[index] != "" {
				encoded, err := encodeSecret([]byte(e.values[index]))
				if err != nil {
					return nil, err
				}
				changed["encpassword"] = []string{encoded}
			}
			continue
		}
		value := strings.TrimSpace(e.values[index])
		if value == strings.TrimSpace(e.initial[index]) {
			continue
		}
		if err := validateHostField(field.key, value); err != nil {
			e.focus, e.caret = index, len([]rune(e.values[index]))
			return nil, err
		}
		if !field.multi && strings.Contains(value, "\n") {
			return nil, fmt.Errorf("%s 只能填写一行", field.key)
		}
		lower := strings.ToLower(field.key)
		changed[lower] = nil
		for _, line := range strings.Split(value, "\n") {
			if line = strings.TrimSpace(line); line != "" {
				changed[lower] = append(changed[lower], line)
			}
		}
	}
	// Only reject proxy conflicts introduced by the form; existing configurations
	// can intentionally contain both directives and are preserved on unrelated edits.
	jump, initialJump := e.value("ProxyJump")
	command, initialCommand := e.value("ProxyCommand")
	if (jump != initialJump || command != initialCommand) &&
		strings.TrimSpace(jump) != "" && strings.TrimSpace(command) != "" {
		return nil, fmt.Errorf("ProxyJump 与 ProxyCommand 请只填写一项")
	}
	if e.block >= 0 && len(changed) == 0 && alias == e.alias {
		return e.document.original, nil
	}
	var block []string
	if e.block >= 0 {
		source := e.document.blocks[e.block]
		block = append([]string(nil), e.document.lines[source.start:source.end]...)
	} else {
		block = []string{"Host " + alias}
	}
	header := block[0]
	if alias != e.alias || e.block < 0 || len(e.document.blocks[e.block].aliases) > 1 {
		header = "Host " + alias
		if _, comment, ok := strings.Cut(block[0], "#"); ok {
			header += " #" + comment
		}
	}
	updated := []string{header}
	for _, field := range hostEditorFields[1:] {
		lower := strings.ToLower(field.key)
		if field.key == "Password" {
			for _, value := range changed["encpassword"] {
				updated = append(updated, "    encPassword "+value)
			}
		} else {
			for _, value := range changed[lower] {
				updated = append(updated, "    "+field.key+" "+value)
			}
		}
	}
	for index, line := range block[1:] {
		key, _ := configLineParts(line)
		if _, ok := changed[key]; !ok {
			updated = append(updated, line)
		} else if e.block >= 0 {
			if comment := e.document.comments[e.document.blocks[e.block].start+index+1]; comment != "" {
				updated = append(updated, "    #"+comment)
			}
		}
	}
	var lines []string
	if e.block < 0 {
		lines = append(updated, "", "Host *")
		lines = append(lines, e.document.lines...)
	} else {
		source := e.document.blocks[e.block]
		lines = append(lines, e.document.lines[:source.start]...)
		lines = append(lines, updated...)
		if len(source.aliases) > 1 {
			// Give the selected alias its own copy; other aliases retain the original.
			remaining := []string{}
			for _, old := range source.aliases {
				if old != e.alias {
					remaining = append(remaining, old)
				}
			}
			lines = append(lines, "Host "+strings.Join(remaining, " "))
			lines = append(lines, e.document.lines[source.start+1:source.end]...)
		}
		lines = append(lines, e.document.lines[source.end:]...)
	}
	content := []byte(strings.Join(lines, e.document.newline))
	if len(content) > 0 && !bytes.HasSuffix(content, []byte(e.document.newline)) {
		content = append(content, e.document.newline...)
	}
	if _, err := ssh_config.Decode(bytes.NewReader(content)); err != nil {
		return nil, fmt.Errorf("配置格式不正确：%w", err)
	}
	return content, nil
}

func (p *sshPrompt) saveEditor() {
	e := p.editor
	content, err := e.content()
	if err != nil {
		e.err = err.Error()
		return
	}
	alias := strings.TrimSpace(e.values[0])
	documents, err := hostConfigDocuments(p.config.configPath)
	if err != nil {
		e.err = err.Error()
		return
	}
	if alias != e.alias {
		for _, d := range documents {
			for _, block := range d.blocks {
				if slices.Contains(block.aliases, alias) {
					e.err = "别名已存在，请使用其他名称"
					return
				}
			}
		}
	}
	if err := e.document.write(content); err != nil {
		e.err = "保存失败：" + err.Error()
		return
	}
	refreshed := &tsshConfig{configPath: p.config.configPath, exConfigPath: p.config.exConfigPath}
	// Connection helpers read the global configuration, so reload it along with
	// the prompt's host list and detail cache after a successful write.
	userConfig = refreshed
	p.config = refreshed
	p.hosts = getAllHosts()
	p.detailCache = nil
	p.clearSearch()
	for i, host := range p.visible {
		if host.Alias == alias {
			p.cursor = i
			break
		}
	}
	p.editor = nil
	p.detailFocus = false
	p.notice = "已保存：" + alias
}

func (e *hostConfigEditor) displayValue(index, width int) string {
	field := hostEditorFields[index]
	value := e.values[index]
	if field.secret {
		value = strings.Repeat("•", len([]rune(value)))
	}
	if value == "" {
		value = "（未设置）"
		if field.key == "Password" && !e.passwordChanged && e.block >= 0 {
			block := e.document.blocks[e.block]
			if len(block.values["password"]) > 0 || len(block.values["encpassword"]) > 0 {
				value = "（已设置，输入替换）"
			}
		}
		if index == e.focus {
			return "▏" + fitPromptText(value, max(0, width-1))
		}
		return promptColor(promptMuted, fitPromptText(value, width))
	}
	if index != e.focus {
		return fitPromptText(strings.ReplaceAll(value, "\n", " ⏎ "), width)
	}
	chars := []rune(value)
	before := strings.ReplaceAll(string(chars[:e.caret]), "\n", " ⏎ ")
	after := strings.ReplaceAll(string(chars[e.caret:]), "\n", " ⏎ ")
	// Reserve a cell for the caret, including at the end of a long CJK path.
	if ansi.StringWidth(before) >= width {
		before = ansi.TruncateLeft(before, ansi.StringWidth(before)-max(0, (width-2)/2), "…")
	}
	before = ansi.Truncate(before, max(0, width-1), "")
	return fitPromptText(ansi.Truncate(before+"▏"+after, width, ""), width)
}

type editorFormRow struct {
	text, group string
	field       int // -1 for headings and blank rows
}

func (e *hostConfigEditor) formLayout(width, height int) []editorFormRow {
	labelWidth, keyWidth := 0, 0
	for _, field := range hostEditorFields {
		labelWidth = max(labelWidth, ansi.StringWidth(field.label))
		if width >= 90 {
			keyWidth = max(keyWidth, ansi.StringWidth(field.key))
		}
	}
	valueWidth := width - 2 - labelWidth - 3
	if keyWidth > 0 {
		valueWidth -= keyWidth + 2
	}
	var all []editorFormRow
	group, focusRow := "", 0
	groupHeading := func(group string) string {
		text := " " + group + " "
		return promptColor(promptAccent, text+strings.Repeat("─", max(0, width-ansi.StringWidth(text))))
	}
	for i, field := range hostEditorFields {
		if field.group != group {
			group = field.group
			all = append(all, editorFormRow{groupHeading(group), group, -1})
		}
		prefix := "  "
		if i == e.focus {
			prefix = "› "
			focusRow = len(all)
		}
		line := prefix + fitPromptText(field.label, labelWidth) + " │ "
		if keyWidth > 0 {
			line += promptColor(promptMuted, fitPromptText(field.key, keyWidth)) + "  "
		}
		line += e.displayValue(i, valueWidth)
		if i == e.focus {
			// Strip nested styles so the focus background spans every column.
			line = promptColor(promptActive, ansi.Strip(line))
		}
		all = append(all, editorFormRow{line, group, i})
	}
	if height == 1 {
		return []editorFormRow{all[focusRow]}
	}
	start := max(0, min(focusRow-height/2, len(all)-height))
	var lines []editorFormRow
	if all[start].field >= 0 {
		// Keep a group heading visible when the viewport begins within a group.
		start = max(start, focusRow-height+2)
		if all[start].field >= 0 {
			lines = append(lines, editorFormRow{groupHeading(all[start].group), all[start].group, -1})
		}
	}
	for i := start; i < len(all) && len(lines) < height; i++ {
		if all[i].field < 0 && len(lines) == height-1 {
			break
		}
		lines = append(lines, all[i])
	}
	for len(lines) < height {
		lines = append(lines, editorFormRow{field: -1})
	}
	return lines
}

func (e *hostConfigEditor) formRows(width, height int) []string {
	layout := e.formLayout(width, height)
	lines := make([]string, len(layout))
	for i, row := range layout {
		lines[i] = row.text
	}
	return lines
}

func (p *sshPrompt) editorView() tea.View {
	e := p.editor
	width, height := max(1, p.width), max(1, p.height)
	title := "新增主机配置"
	if e.block >= 0 {
		title = "编辑主机配置 · " + e.alias
	}
	field := hostEditorFields[e.focus]
	hint, hintStyle := field.hint, promptMuted
	if e.err != "" {
		hint, hintStyle = e.err, "1;31"
	}
	var lines []string
	margin := 0
	if width < 24 || height < 12 {
		label := "› " + field.label + "："
		lines = []string{promptColor(promptAccent, title),
			label + e.displayValue(e.focus, max(1, width-ansi.StringWidth(label))),
			promptColor(hintStyle, hint), "Ctrl+S保存 Esc取消 Tab切换"}
	} else {
		panelWidth := min(width, 112)
		margin = (width - panelWidth) / 2
		innerWidth := panelWidth - 4
		position := fmt.Sprintf("%d / %d", e.focus+1, len(hostEditorFields))
		titleWidth := panelWidth - ansi.StringWidth(position) - 2
		lines = []string{promptColor(promptAccent, fitPromptText(title, titleWidth)) + "  " + promptColor(promptMuted, position),
			promptColor(promptMuted, "文件："+truncatePromptMiddle(e.document.path, panelWidth-6)),
			promptColor(promptMuted, "仅修改此 Host · 留空继承配置 · 扩展 password 文件优先")}
		border := func(left, right string) string {
			return promptColor(promptMuted, left+strings.Repeat("─", panelWidth-2)+right)
		}
		lines = append(lines, border("╭", "╮"))
		for _, row := range e.formRows(innerWidth, height-10) {
			lines = append(lines, promptColor(promptMuted, "│ ")+fitPromptText(row, innerWidth)+promptColor(promptMuted, " │"))
		}
		lines = append(lines, border("╰", "╯"), promptColor(promptAccent, field.group+" · "+field.label)+promptColor(promptMuted, "  "+field.key))
		hints := strings.Split(ansi.Hardwrap(hint, panelWidth, true), "\n")
		for i := 0; i < 2; i++ {
			line := ""
			if i < len(hints) {
				line = hints[i]
			}
			lines = append(lines, promptColor(hintStyle, line))
		}
		lines = append(lines, promptShortcut("Ctrl+S", "保存")+"  "+promptShortcut("Esc", "取消"))
		keys := "Tab/↑↓ 切换  ←→ 移动  Ctrl+U 清空"
		if field.multi {
			keys = "Tab/↑↓ 切换  Ctrl+U 清空  Ctrl+N 添加一行"
			if panelWidth < 50 {
				keys = "Tab切换 Ctrl+U清空 Ctrl+N换行"
			}
		}
		lines = append(lines, promptColor(promptMuted, keys))
	}
	lines = lines[:min(len(lines), height)]
	for i := range lines {
		lines[i] = fitPromptText(strings.Repeat(" ", margin)+lines[i], width)
	}
	view := tea.NewView(strings.Join(lines, "\n"))
	view.AltScreen = true
	view.MouseMode = promptMouseMode()
	return view
}
