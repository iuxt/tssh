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

import tea "charm.land/bubbletea/v2"

const promptBodyTop = 5

func promptMouseMode() tea.MouseMode {
	// The legacy Windows reader is a byte stream: native console mouse records
	// are unavailable there. Keep keyboard navigation usable in that fallback.
	if isRunningOnOldWindows.Load() {
		return tea.MouseModeNone
	}
	return tea.MouseModeCellMotion
}

func (p *sshPrompt) mouseInView(x, y int) bool {
	return x >= 0 && x < p.width && y >= 0 && y < p.height
}

func (p *sshPrompt) mousePanel(x int) (details, valid bool) {
	left, right := p.panelWidths()
	if right == 0 {
		return p.detailFocus, true
	}
	if x < left {
		return false, true
	}
	if x >= left+3 {
		return true, true
	}
	return false, false
}

func (p *sshPrompt) wheelPrompt(msg tea.MouseWheelMsg) {
	if !p.mouseInView(msg.X, msg.Y) || msg.Mod != 0 || p.height < promptChromeRows+1 ||
		promptMouseMode() == tea.MouseModeNone || msg.Y < 3 || msg.Y >= promptBodyTop+p.pageSize() {
		return
	}
	delta := 0
	switch msg.Button {
	case tea.MouseWheelUp:
		delta = -3
	case tea.MouseWheelDown:
		delta = 3
	default:
		return
	}
	if !p.showShortcuts {
		details, valid := p.mousePanel(msg.X)
		if !valid {
			return
		}
		p.detailFocus = details
	}
	p.move(delta)
}

func (p *sshPrompt) editorMouseInForm(x, y int) bool {
	panelWidth := min(p.width, 112)
	margin := (p.width - panelWidth) / 2
	return promptMouseMode() != tea.MouseModeNone && p.width >= 24 && p.height >= 12 &&
		x >= margin+2 && x < margin+panelWidth-2 && y >= 4 && y < p.height-6
}

func (p *sshPrompt) wheelEditor(msg tea.MouseWheelMsg) {
	if msg.Mod != 0 || !p.editorMouseInForm(msg.X, msg.Y) {
		return
	}
	delta := 0
	switch msg.Button {
	case tea.MouseWheelUp:
		delta = -3
	case tea.MouseWheelDown:
		delta = 3
	default:
		return
	}
	e := p.editor
	e.focus = max(0, min(e.focus+delta, len(hostEditorFields)-1))
	e.caret = len([]rune(e.values[e.focus]))
}
