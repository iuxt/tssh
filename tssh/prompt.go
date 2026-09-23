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
	"io"
	"os"
	"reflect"
	"strings"
	"time"

	"github.com/chzyer/readline"
	"github.com/trzsz/promptui"
)

var promptCursorIcon = "🧨"
var promptSelectedIcon = "🍺"

const (
	keyCtrlB = '\x02'
	keyCtrlC = '\x03'
	keyCtrlD = '\x04'
	keyCtrlE = '\x05'
	keyCtrlF = '\x06'
	keyCtrlH = '\x08'
	keyCtrlJ = '\x0a'
	keyCtrlK = '\x0b'
	keyCtrlL = '\x0c'
	keyCtrlQ = '\x11'
	keyCtrlU = '\x15'
	keyEnter = '\x0d'
	keyESC   = '\x1b'
)

type sshPrompt struct {
	selector      *promptui.Select
	pipeOut       io.WriteCloser
	hosts         []*sshHost
	showShortcuts bool
	search        bool
	quit          bool
}

type bellFilter struct {
	writer io.Writer
}

func (b *bellFilter) Write(p []byte) (int, error) {
	if len(p) == 1 && p[0] == readline.CharBell {
		return 1, nil
	}
	return b.writer.Write(p)
}

func (b *bellFilter) Close() error {
	return nil
}

type sshShortcuts struct {
	actionName    string
	globalKeys    []string
	searchKeys    []string
	nonSearchKeys []string
}

var normalShortcuts = []sshShortcuts{
	{actionName: "Confirm  ", globalKeys: []string{"Enter"}, nonSearchKeys: nil},
	{actionName: "Quit/Exit", globalKeys: []string{"Ctrl+C", "Ctrl+Q"}, nonSearchKeys: []string{"q", "Q"}},
	{actionName: "Move Prev", globalKeys: []string{"Ctrl+K", "Shift+Tab", "↑"}, nonSearchKeys: []string{"k", "K"}},
	{actionName: "Move Next", globalKeys: []string{"Ctrl+J", "Tab      ", "↓"}, nonSearchKeys: []string{"j", "J"}},
	{actionName: "Page   Up", globalKeys: []string{"Ctrl+H", "Ctrl+U", "Ctrl+B", "PageUp  ", "←"}, nonSearchKeys: []string{"h", "H", "u", "U", "b", "B"}},
	{actionName: "Page Down", globalKeys: []string{"Ctrl+L", "Ctrl+D", "Ctrl+F", "PageDown", "→"}, nonSearchKeys: []string{"l", "L", "d", "D", "f", "F"}},
	{actionName: "Goto Home", globalKeys: []string{"Home"}, nonSearchKeys: []string{"g"}},
	{actionName: "Goto  End", globalKeys: []string{"End "}, nonSearchKeys: []string{"G"}},
	{actionName: "EraseKeys", globalKeys: []string{"Ctrl+E"}, nonSearchKeys: []string{"e", "E"}},
	{actionName: "TglSearch", globalKeys: []string{"/"}, searchKeys: []string{"Esc", "Enter"}},
	{actionName: "Tgl  Help", globalKeys: []string{"?"}},
}

func (p *sshPrompt) getShortcuts() []string {
	if !p.showShortcuts {
		p.selector.HideHelp = false
		return nil
	}
	p.selector.HideHelp = true
	shortcuts := []string{"Shortcuts:"}
	addShortcuts := func(ss []sshShortcuts) {
		for _, s := range ss {
			keys := s.globalKeys
			if p.search {
				keys = append(keys, s.searchKeys...)
			} else {
				keys = append(keys, s.nonSearchKeys...)
			}
			shortcuts = append(shortcuts, fmt.Sprintf("  %s:  %s", s.actionName, strings.Join(keys, "  ")))
		}
	}
	addShortcuts(normalShortcuts)
	return shortcuts
}

func (p *sshPrompt) getPageCount() int {
	return (len(p.hosts)-1)/getPromptPageSize() + 1
}

func (p *sshPrompt) userQuit(buf []byte) bool {
	if len(buf) != 1 {
		return false
	}
	switch buf[0] {
	case keyCtrlC, keyCtrlQ:
		return true
	case 'q', 'Q':
		return !p.search
	default:
		return false
	}
}

func (p *sshPrompt) movePrev(buf []byte) bool {
	if len(buf) == 3 && buf[0] == '\x1b' && buf[1] == '\x5b' {
		switch buf[2] {
		case 'A', 'Z': // ↑Arrow-Up Shift-Tab
			return true
		}
	}
	if len(buf) != 1 {
		return false
	}
	switch buf[0] {
	case keyCtrlK:
		return true
	case 'k', 'K':
		return !p.search
	default:
		return false
	}
}

func (p *sshPrompt) moveNext(buf []byte) bool {
	if len(buf) == 3 && buf[0] == '\x1b' && buf[1] == '\x5b' {
		switch buf[2] {
		case 'B': // ↓Arrow-Down
			return true
		}
	}
	if len(buf) != 1 {
		return false
	}
	switch buf[0] {
	case '\t', keyCtrlJ:
		return true
	case 'j', 'J':
		return !p.search
	default:
		return false
	}
}

func (p *sshPrompt) pageUp(buf []byte) bool {
	if len(buf) == 3 && buf[0] == '\x1b' && buf[1] == '\x5b' {
		switch buf[2] {
		case 'D': // ←Arrow-Left
			return true
		}
	}
	if len(buf) == 4 && buf[0] == '\x1b' && buf[1] == '\x5b' && buf[3] == '~' {
		switch buf[2] {
		case '5': // PageUp
			return true
		}
	}
	if len(buf) != 1 {
		return false
	}
	switch buf[0] {
	case keyCtrlH, keyCtrlU, keyCtrlB:
		return true
	case 'h', 'H', 'u', 'U', 'b', 'B':
		return !p.search
	default:
		return false
	}
}

func (p *sshPrompt) pageDown(buf []byte) bool {
	if len(buf) == 3 && buf[0] == '\x1b' && buf[1] == '\x5b' {
		switch buf[2] {
		case 'C': // →Arrow-Right
			return true
		}
	}
	if len(buf) == 4 && buf[0] == '\x1b' && buf[1] == '\x5b' && buf[3] == '~' {
		switch buf[2] {
		case '6': // PageDown
			return true
		}
	}
	if len(buf) != 1 {
		return false
	}
	switch buf[0] {
	case keyCtrlL, keyCtrlD, keyCtrlF:
		return true
	case 'l', 'L', 'd', 'D', 'f', 'F':
		return !p.search
	default:
		return false
	}
}

func (p *sshPrompt) gotoHome(buf []byte) bool {
	if len(buf) == 3 && buf[0] == '\x1b' && buf[1] == '\x5b' {
		switch buf[2] {
		case 'H': // Home
			return true
		}
	}
	if len(buf) == 4 && buf[0] == '\x1b' && buf[1] == '\x5b' && buf[3] == '~' {
		switch buf[2] {
		case '1': // Fn-Arrow-Left
			return true
		}
	}
	if len(buf) != 1 {
		return false
	}
	switch buf[0] {
	case 'g':
		return !p.search
	default:
		return false
	}
}

func (p *sshPrompt) gotoEnd(buf []byte) bool {
	if len(buf) == 3 && buf[0] == '\x1b' && buf[1] == '\x5b' {
		switch buf[2] {
		case 'F': // End
			return true
		}
	}
	if len(buf) == 4 && buf[0] == '\x1b' && buf[1] == '\x5b' && buf[3] == '~' {
		switch buf[2] {
		case '4': // Fn-Arrow-Right
			return true
		}
	}
	if len(buf) != 1 {
		return false
	}
	switch buf[0] {
	case 'G':
		return !p.search
	default:
		return false
	}
}

func (p *sshPrompt) toggleSearch(buf []byte) bool {
	if len(buf) != 1 {
		return false
	}
	switch buf[0] {
	case '/':
		return true
	case keyESC:
		return p.search
	default:
		return false
	}
}

func (p *sshPrompt) toggleShortcuts(buf []byte) bool {
	if len(buf) != 1 {
		return false
	}
	switch buf[0] {
	case '?':
		return true
	default:
		return false
	}
}

func (p *sshPrompt) addKeywords(buf []byte) bool {
	if len(buf) != 1 {
		return false
	}
	switch buf[0] {
	case keyEnter:
		return p.search && p.selector.GetVisibleSize() > 0
	default:
		return false
	}
}

func (p *sshPrompt) eraseKeywords(buf []byte) bool {
	if len(buf) != 1 {
		return false
	}
	switch buf[0] {
	case keyCtrlE:
		return true
	case 'e', 'E':
		return !p.search
	default:
		return false
	}
}

func (p *sshPrompt) userConfirm(buf []byte) bool {
	return len(buf) == 1 && buf[0] == keyEnter && !p.search
}

func (p *sshPrompt) wrapStdin() {
	defer func() {
		_ = p.pipeOut.Close()
		_ = p.selector.Stdin.Close()
	}()
	buffer := make([]byte, 100)
	if strings.ToLower(userConfig.promptDefaultMode) == "search" {
		p.search = true
		_, _ = p.pipeOut.Write([]byte{'/'})
	}
	for {
		n, err := os.Stdin.Read(buffer)
		buf := buffer[:n]
		switch {
		case err != nil || p.userQuit(buf):
			p.quit = true
			return
		case p.movePrev(buf):
			buf = []byte{readline.CharPrev}
		case p.moveNext(buf):
			buf = []byte{readline.CharNext}
		case p.pageUp(buf):
			buf = []byte{readline.CharBackward}
		case p.pageDown(buf):
			buf = []byte{readline.CharForward}
		case p.gotoHome(buf):
			buf = bytes.Repeat([]byte{readline.CharBackward}, p.getPageCount())
		case p.gotoEnd(buf):
			buf = bytes.Repeat([]byte{readline.CharForward}, p.getPageCount())
		case p.toggleSearch(buf):
			p.search = !p.search
			buf = []byte{'/'}
		case p.toggleShortcuts(buf):
			p.showShortcuts = !p.showShortcuts
			buf = []byte{promptui.KeyRefresh}
		case p.addKeywords(buf):
			p.search = false
			buf = []byte{promptui.KeySoftEnter}
		case p.eraseKeywords(buf):
			p.search = false
			buf = []byte{promptui.KeyCtrlE}
		case p.userConfirm(buf):
			_, _ = p.pipeOut.Write([]byte{readline.CharEnter})
			return
		case len(buf) == 1 && buf[0] == '\x00':
			// avoid Ctrl+Space causing quit unexpectedly
			buf = []byte{promptui.KeyRefresh}
		}
		p.selector.Shortcuts = p.getShortcuts()
		_, _ = p.pipeOut.Write(buf)
	}
}

func matchHost(h *sshHost, keywords []string) bool {
	host := strings.ToLower(h.Host)
	alias := strings.ToLower(h.Alias)
	labels := strings.ToLower(h.GroupLabels)
	for _, keyword := range keywords {
		if !strings.Contains(host, keyword) &&
			!strings.Contains(alias, keyword) &&
			!strings.Contains(labels, keyword) {
			return false
		}
	}
	return true
}

func chooseAlias(keywords string) (string, bool, error) {
	if state, _ := makeStdinRaw(); state != nil {
		defer resetStdin(state)
	}

	hosts := getAllHosts()

	searcher := func(input string, index int) bool {
		return matchHost(hosts[index], strings.Fields(strings.ToLower(input)))
	}

	theme := getPromptTheme()
	funcMap := promptui.FuncMap
	funcMap["getExConfig"] = getExConfig
	funcMap["hasField"] = func(obj any, field string) bool {
		v := reflect.ValueOf(obj)
		if v.Kind() == reflect.Pointer {
			v = v.Elem()
		}
		return v.FieldByName(field).IsValid()
	}

	pipeIn, pipeOut := io.Pipe()
	prompt := sshPrompt{
		selector: &promptui.Select{
			Label: "SSH Alias",
			Items: hosts,
			Templates: &promptui.SelectTemplates{
				Help:            theme.Help,
				Label:           theme.Label,
				Active:          theme.Active,
				Inactive:        theme.Inactive,
				Details:         theme.Details,
				Shortcuts:       theme.Shortcuts,
				HideLabel:       theme.HideLabel,
				ItemsRenderer:   theme.ItemsRenderer,
				DetailsRenderer: theme.DetailsRenderer,
				FuncMap:         funcMap,
			},
			Size:         getPromptPageSize(),
			Searcher:     searcher,
			Stdin:        pipeIn,
			Stdout:       &bellFilter{os.Stderr},
			HideSelected: true,
			Keywords:     keywords,
		},
		pipeOut: pipeOut,
		hosts:   hosts,
	}

	if enableDebugLogging && tmuxDebugPaneWriter == nil {
		enableDebugLogging = false
		defer func() { enableDebugLogging = true }()
	}

	go prompt.wrapStdin()

	idx, _, err := prompt.selector.Run()
	if err != nil {
		return "", prompt.quit, fmt.Errorf("prompt choose alias failed: %v", err)
	}
	if prompt.quit {
		return "", true, nil
	}

	selectedHost := hosts[idx]
	fmt.Fprintf(os.Stderr, "\033[0;32m%s %s\033[0m\r\n", promptSelectedIcon, selectedHost.Alias)
	return selectedHost.Alias, false, nil
}

func predictDestination(dest string) (string, bool, error) {
	if !isTerminal || strings.ContainsAny(dest, ".:[]@") {
		return dest, false, nil
	}

	if userConfig.useOpenSSHConfig {
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
