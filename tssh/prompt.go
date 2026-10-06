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
	"os"
	"strings"
	"time"
	"unicode"

	tea "charm.land/bubbletea/v2"
)

var promptCursorIcon = "›"
var promptSelectedIcon = "✓"

const (
	defaultPromptPageSize = 10
	defaultPromptWidth    = 80
	promptChromeRows      = 7
)

// sshPrompt owns selection and search state; rendering lives in prompt_style.go.
type sshPrompt struct {
	hosts                 []*sshHost
	visible               []*sshHost
	cursor                int
	helpOffset            int
	detailOffset          int
	detailFocus           bool
	showAllConfig         bool
	config                *tsshConfig
	detailCache           map[*sshHost][]promptConfigEntry
	width, height         int
	keywords, query       string
	search, showShortcuts bool
	selected              *sshHost
	quit                  bool
}

func newSSHPrompt(hosts []*sshHost, keywords string) *sshPrompt {
	p := &sshPrompt{hosts: hosts, keywords: keywords, width: defaultPromptWidth,
		height: defaultPromptPageSize + promptChromeRows}
	p.filterHosts()
	return p
}

func (p *sshPrompt) Init() tea.Cmd { return nil }

func calculatePromptPageSize(height int) int {
	if height <= 0 {
		return defaultPromptPageSize
	}
	return max(1, height-promptChromeRows)
}

func (p *sshPrompt) pageSize() int { return calculatePromptPageSize(p.height) }

func (p *sshPrompt) currentHost() *sshHost {
	if p.cursor < 0 || p.cursor >= len(p.visible) {
		return nil
	}
	return p.visible[p.cursor]
}

func matchHost(h *sshHost, keywords []string) bool {
	host, alias, labels := strings.ToLower(h.Host), strings.ToLower(h.Alias), strings.ToLower(h.GroupLabels)
	for _, keyword := range keywords {
		if !strings.Contains(host, keyword) && !strings.Contains(alias, keyword) && !strings.Contains(labels, keyword) {
			return false
		}
	}
	return true
}

func (p *sshPrompt) filterHosts() {
	keywords := strings.Fields(strings.ToLower(p.keywords + " " + p.query))
	p.visible = nil
	for _, host := range p.hosts {
		if matchHost(host, keywords) {
			p.visible = append(p.visible, host)
		}
	}
	p.cursor, p.detailOffset = 0, 0
}

func (p *sshPrompt) move(offset int) {
	if p.showShortcuts {
		p.helpOffset = max(0, min(p.helpOffset+offset, len(promptHelp(p.width))-p.pageSize()))
		return
	}
	if p.detailFocus {
		p.detailOffset = max(0, min(p.detailOffset+offset, len(p.detailLines())-p.pageSize()))
		return
	}
	previous := p.cursor
	p.cursor = max(0, min(p.cursor+offset, len(p.visible)-1))
	if previous != p.cursor {
		p.detailOffset = 0
	}
}

func (p *sshPrompt) jump(end bool) {
	if p.showShortcuts {
		p.helpOffset = 0
		if end {
			p.helpOffset = max(0, len(promptHelp(p.width))-p.pageSize())
		}
	} else if p.detailFocus {
		p.detailOffset = 0
		if end {
			p.detailOffset = max(0, len(p.detailLines())-p.pageSize())
		}
	} else {
		p.cursor, p.detailOffset = 0, 0
		if end {
			p.cursor = max(0, len(p.visible)-1)
		}
	}
}

func (p *sshPrompt) clearSearch() {
	p.search, p.keywords, p.query = false, "", ""
	p.filterHosts()
}

func (p *sshPrompt) toggleSearch() {
	p.detailFocus = false
	p.search = !p.search
	p.query = ""
	p.filterHosts()
}

func (p *sshPrompt) appendQuery(text string) {
	// Pasted newlines are search separators, never actions or terminal controls.
	p.query += strings.Map(func(r rune) rune {
		if unicode.IsSpace(r) {
			return ' '
		}
		if unicode.IsControl(r) {
			return -1
		}
		return r
	}, text)
	p.filterHosts()
}

func (p *sshPrompt) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		p.width, p.height = max(1, msg.Width), max(1, msg.Height)
	case tea.PasteMsg:
		if p.search {
			p.appendQuery(msg.Content)
		}
	case tea.KeyPressMsg:
		key := msg.String()
		switch key {
		case "ctrl+c", "ctrl+q":
			p.quit = true
			return p, tea.Quit
		case "up", "shift+tab", "ctrl+k":
			p.move(-1)
		case "down", "tab", "ctrl+j":
			p.move(1)
		case "left", "pgup", "ctrl+h", "ctrl+u", "ctrl+b":
			p.move(-p.pageSize())
		case "right", "pgdown", "ctrl+l", "ctrl+d", "ctrl+f":
			p.move(p.pageSize())
		case "home":
			p.jump(false)
		case "end":
			p.jump(true)
		case "f2":
			if !p.showShortcuts {
				p.detailFocus = !p.detailFocus
			}
		case "f3":
			if !p.showShortcuts {
				p.showAllConfig = !p.showAllConfig
				p.detailOffset = 0
			}
		case "ctrl+e":
			p.clearSearch()
		case "/":
			p.toggleSearch()
		case "?":
			p.showShortcuts = !p.showShortcuts
		case "esc":
			if p.showShortcuts {
				p.showShortcuts = false
			} else if p.detailFocus {
				p.detailFocus = false
			} else if p.search {
				p.toggleSearch()
			}
		case "enter":
			if p.showShortcuts {
				break
			}
			if p.currentHost() == nil {
				break
			}
			if p.search {
				p.keywords = strings.TrimSpace(p.keywords + " " + p.query)
				p.query, p.search = "", false
			} else {
				p.selected = p.currentHost()
				return p, tea.Quit
			}
		default:
			if p.search {
				if key == "backspace" {
					chars := []rune(p.query)
					if len(chars) > 0 {
						p.query = string(chars[:len(chars)-1])
						p.filterHosts()
					}
				} else if msg.Text != "" {
					p.appendQuery(msg.Text)
				}
			} else {
				switch key {
				case "q", "Q":
					p.quit = true
					return p, tea.Quit
				case "k", "K":
					p.move(-1)
				case "j", "J":
					p.move(1)
				case "h", "H", "u", "U", "b", "B":
					p.move(-p.pageSize())
				case "l", "L", "d", "D", "f", "F":
					p.move(p.pageSize())
				case "g":
					p.jump(false)
				case "G":
					p.jump(true)
				case "e", "E":
					p.clearSearch()
				}
			}
		}
	}
	return p, nil
}

func chooseAlias(keywords string) (string, bool, error) {
	// Keep the stty fallback for older Windows terminals, whose custom reader
	// cannot be detected as a TTY by Bubble Tea.
	if state, _ := makeStdinRaw(); state != nil {
		defer resetStdin(state)
	}
	prompt := newSSHPrompt(getAllHosts(), keywords)
	prompt.config = userConfig
	if enableDebugLogging && tmuxDebugPaneWriter == nil {
		enableDebugLogging = false
		defer func() { enableDebugLogging = true }()
	}
	opts, cancelReader := newTeaOptions(nil)
	defer cancelReader()
	program := tea.NewProgram(prompt, append(opts, tea.WithOutput(os.Stderr))...)
	if _, err := program.Run(); err != nil {
		return "", prompt.quit, fmt.Errorf("打开主机选择界面失败：%w", err)
	}
	if prompt.quit || prompt.selected == nil {
		return "", true, nil
	}
	fmt.Fprintf(os.Stderr, "%s 正在连接 %s…\r\n", promptSelectedIcon, prompt.selected.Alias)
	return prompt.selected.Alias, false, nil
}

func predictDestination(dest string) (string, bool, error) {
	if !isTerminal || strings.ContainsAny(dest, ".:[]@") {
		return dest, false, nil
	}

	hosts := getAllHosts()
	for _, host := range hosts {
		if host.Alias == dest {
			return dest, false, nil
		}
	}

	for _, pattern := range userConfig.wildcardPatterns {
		if pattern.Regex().MatchString(dest) {
			return dest, false, nil
		}
	}

	match := false
	keywords := strings.Fields(strings.ToLower(dest))
	for _, host := range hosts {
		if matchHost(host, keywords) {
			match = true
			break
		}
	}
	if !match {
		return dest, false, nil
	}

	if _, err := lookupHostWithTimeout(dest, 200*time.Millisecond); err == nil {
		return dest, false, nil
	}

	return chooseAlias(dest)
}
