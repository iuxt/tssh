package tssh

import (
	"bytes"
	"fmt"
	"os"
	"slices"
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/trzsz/ssh_config"
)

type hostConfigDeletion struct {
	document *hostConfigDocument
	content  []byte
}

type hostDeleteDialog struct {
	alias     string
	documents []*hostConfigDocument
	changes   []hostConfigDeletion
	confirm   bool
	err       string
}

// Remove every explicit declaration, including duplicates, so reloading cannot
// bring the deleted alias back. Shared patterns and unrelated source lines stay.
func (d *hostConfigDocument) withoutHost(alias string) ([]byte, bool, error) {
	var lines []string
	start, changed := 0, false
	for _, block := range d.blocks {
		if !slices.Contains(block.aliases, alias) {
			continue
		}
		changed = true
		lines = append(lines, d.lines[start:block.start]...)
		remaining := slices.DeleteFunc(slices.Clone(block.aliases), func(value string) bool { return value == alias })
		if len(remaining) > 0 {
			header := "Host " + strings.Join(remaining, " ")
			if _, comment, ok := strings.Cut(d.lines[block.start], "#"); ok {
				header += " #" + comment
			}
			lines = append(lines, header)
			lines = append(lines, d.lines[block.start+1:block.end]...)
		} else {
			// Includes may declare other hosts. Retain their now-inactive scope
			// instead of dropping them or promoting them to global directives.
			var includes []string
			for _, inc := range d.includes {
				line := inc.directive.Pos().Line - 1
				if line > block.start && line < block.end {
					includes = append(includes, d.lines[line])
				}
			}
			if len(includes) > 0 {
				lines = append(lines, "Host * !* # tssh: 保留 Include 的非活动作用域")
				lines = append(lines, includes...)
			}
			// Comments immediately preceding the next Host belong to that
			// section, and must survive deleting the previous host.
			trailing := block.end
			for trailing > block.start+1 {
				line := strings.TrimSpace(d.lines[trailing-1])
				if line != "" && !strings.HasPrefix(line, "#") {
					break
				}
				trailing--
			}
			lines = append(lines, d.lines[trailing:block.end]...)
		}
		start = block.end
	}
	if !changed {
		return d.original, false, nil
	}
	lines = append(lines, d.lines[start:]...)
	content := []byte(strings.Join(lines, d.newline))
	if _, err := ssh_config.Decode(bytes.NewReader(content)); err != nil {
		return nil, false, fmt.Errorf("删除后的配置格式不正确：%w", err)
	}
	return content, true, nil
}

func (p *sshPrompt) openDeleteDialog() {
	p.notice = ""
	if p.currentHost() == nil {
		p.notice = "请先选择要删除的主机"
		return
	}
	if p.config == nil || p.config.configPath == "" {
		p.notice = "没有可写的主配置路径"
		return
	}
	documents, err := hostConfigDocuments(p.config.configPath)
	if err != nil {
		p.notice = "打开配置失败：" + err.Error()
		return
	}
	dialog := &hostDeleteDialog{alias: p.currentHost().Alias, documents: documents}
	for _, document := range documents {
		content, changed, err := document.withoutHost(dialog.alias)
		if err != nil {
			p.notice = err.Error()
			return
		}
		if changed {
			dialog.changes = append(dialog.changes, hostConfigDeletion{document, content})
		}
	}
	if len(dialog.changes) == 0 {
		p.notice = "没有找到该别名的 Host 配置块"
		return
	}
	p.deleteDialog = dialog
}

func (p *sshPrompt) updateDeleteDialog(msg tea.Msg) (tea.Model, tea.Cmd) {
	dialog := p.deleteDialog
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		p.width, p.height = max(1, msg.Width), max(1, msg.Height)
	case tea.KeyPressMsg:
		switch msg.String() {
		case "ctrl+c", "ctrl+q":
			p.deleteDialog, p.quit = nil, true
			return p, tea.Quit
		case "esc", "n", "N", "q", "Q":
			p.deleteDialog = nil
		case "tab", "shift+tab", "left", "right", "up", "down":
			dialog.confirm = !dialog.confirm
		case "y", "Y":
			p.deleteHost()
		case "enter":
			if dialog.confirm {
				p.deleteHost()
			} else {
				p.deleteDialog = nil
			}
		}
	}
	return p, nil
}

func (dialog *hostDeleteDialog) write() error {
	// Check the whole Include graph before touching any file: a changed parent
	// could have moved the host even when the source file itself is unchanged.
	for _, document := range dialog.documents {
		current, err := os.ReadFile(document.path)
		if err != nil {
			return err
		}
		if !bytes.Equal(current, document.original) {
			return fmt.Errorf("配置文件已被其他程序修改，请取消并重新打开删除提示")
		}
	}
	for i, change := range dialog.changes {
		if err := change.document.write(change.content); err != nil {
			// Each source is replaced atomically. Restore earlier sources if a
			// later write fails, checking for external edits during rollback too.
			for j := i - 1; j >= 0; j-- {
				previous := dialog.changes[j]
				rollback := *previous.document
				rollback.original = previous.content
				if rollbackErr := rollback.write(previous.document.original); rollbackErr != nil {
					return fmt.Errorf("%w；回退 %s 失败：%v，请检查配置文件", err, rollback.path, rollbackErr)
				}
			}
			return err
		}
	}
	return nil
}

func (p *sshPrompt) deleteHost() {
	dialog := p.deleteDialog
	if err := dialog.write(); err != nil {
		dialog.err, dialog.confirm = "删除失败："+err.Error(), false
		return
	}
	cursor := p.cursor
	refreshed := &tsshConfig{configPath: p.config.configPath, exConfigPath: p.config.exConfigPath}
	userConfig, p.config = refreshed, refreshed
	p.hosts = getAllHosts()
	p.detailCache = nil
	p.filterHosts()
	p.cursor = max(0, min(cursor, len(p.visible)-1))
	p.detailFocus = false
	p.deleteDialog = nil
	p.notice = "已删除：" + dialog.alias
}

func (p *sshPrompt) deleteDialogView(background string) string {
	dialog := p.deleteDialog
	width, height := max(1, p.width), max(1, p.height)
	panelWidth := min(76, width)
	innerWidth := max(1, panelWidth-4)
	cancel, confirm := " 取消 ", " 删除 "
	if dialog.confirm {
		confirm = promptColor("1;37;41", confirm)
	} else {
		cancel = promptColor(promptActive, cancel)
	}
	buttons := cancel + "    " + confirm
	rows := []string{promptColor("1;31", "删除主机？"), "主机：" + dialog.alias}
	for _, change := range dialog.changes {
		rows = append(rows, "文件："+truncatePromptMiddle(change.document.path, max(1, innerWidth-6)))
	}
	rows = append(rows, "将移除此别名的所有显式配置；共用块中的其他别名保留。", "此操作不可撤销。")
	if dialog.err != "" {
		rows = append(rows, promptColor("1;31", dialog.err))
	}
	keys := "Tab/←→ 选择 · Enter 确认 · y 删除 · Esc/n 取消"
	if innerWidth < 60 {
		keys = "Tab切换 Enter确认 y删除 Esc取消"
	}
	rows = append(rows, "", buttons, keys)
	if width < 32 || height < len(rows)+2 {
		// Keep the target, choices and cancellation visible on small terminals.
		rows = []string{"删除：" + dialog.alias, buttons, "y 删除  Esc 取消"}
		if dialog.err != "" {
			rows = append(rows, promptColor("1;31", dialog.err))
		}
	}
	var panel []string
	if width >= 32 && height >= len(rows)+2 {
		border := strings.Repeat("─", panelWidth-2)
		panel = append(panel, "╭"+border+"╮")
		for _, row := range rows {
			panel = append(panel, "│ "+fitPromptText(row, innerWidth)+" │")
		}
		panel = append(panel, "╰"+border+"╯")
	} else {
		panelWidth = width
		for _, row := range rows[:min(len(rows), height)] {
			panel = append(panel, fitPromptText(row, width))
		}
	}
	lines := strings.Split(background, "\n")
	for len(lines) < height {
		lines = append(lines, strings.Repeat(" ", width))
	}
	top, left := (height-len(panel))/2, (width-panelWidth)/2
	for i, row := range panel {
		line := fitPromptText(lines[top+i], width)
		lines[top+i] = ansi.Cut(line, 0, left) + row + ansi.Cut(line, left+panelWidth, width)
	}
	return strings.Join(lines[:height], "\n")
}
