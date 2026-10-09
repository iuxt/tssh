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
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPromptMouseClicksIgnored(t *testing.T) {
	p := newSSHPrompt(promptTestHosts(), "")
	p.Update(tea.WindowSizeMsg{Width: 100, Height: 24})
	cursor := p.cursor
	left, _ := p.panelWidths()
	for _, point := range [][2]int{{8, 5}, {8, 9}, {8, 1}, {left + 3, 6}} {
		for _, button := range []tea.MouseButton{tea.MouseLeft, tea.MouseRight, tea.MouseMiddle} {
			_, cmd := p.Update(tea.MouseClickMsg{X: point[0], Y: point[1], Button: button})
			assert.Nil(t, cmd)
			assert.Equal(t, cursor, p.cursor)
			assert.False(t, p.search)
			assert.False(t, p.detailFocus)
			assert.Nil(t, p.selected)
		}
	}
}

func TestPromptMouseWheelPanelsHelpAndSearch(t *testing.T) {
	p := newSSHPrompt(promptTestHosts(), "")
	p.Update(tea.WindowSizeMsg{Width: 100, Height: 12})
	assert.Equal(t, tea.MouseModeCellMotion, p.View().MouseMode)
	pressPrompt(p, "a")
	left, _ := p.panelWidths()
	p.Update(tea.MouseWheelMsg{X: left + 1, Y: 6, Button: tea.MouseWheelDown})
	assert.False(t, p.detailFocus, "the divider is not a panel")
	assert.Zero(t, p.detailOffset)
	p.Update(tea.MouseWheelMsg{X: left + 3, Y: 6, Button: tea.MouseWheelDown})
	assert.True(t, p.detailFocus)
	assert.Equal(t, 3, p.detailOffset)
	assert.Equal(t, "开发环境", p.currentHost().Alias)
	p.Update(tea.MouseWheelMsg{X: left + 3, Y: 6, Button: tea.MouseWheelUp})
	assert.Zero(t, p.detailOffset)
	p.Update(tea.MouseWheelMsg{X: 5, Y: 6, Button: tea.MouseWheelDown})
	assert.False(t, p.detailFocus)
	assert.Equal(t, "production-europe-primary", p.currentHost().Alias)
	pressPrompt(p, "?")
	cursor := p.cursor
	p.Update(tea.MouseWheelMsg{X: 5, Y: 6, Button: tea.MouseWheelDown})
	assert.Equal(t, 3, p.helpOffset)
	assert.Equal(t, cursor, p.cursor)
	pressPrompt(p, "esc")
	pressPrompt(p, "/")
	pressPrompt(p, "华东")
	p.Update(tea.WindowSizeMsg{Width: 40, Height: 12})
	pressPrompt(p, "f2")
	p.Update(tea.MouseWheelMsg{X: 5, Y: 6, Button: tea.MouseWheelDown})
	assert.True(t, p.detailFocus)
	assert.Positive(t, p.detailOffset)
	assert.Equal(t, "华东", p.query)
}

func TestPromptMouseWheelBounds(t *testing.T) {
	p := newSSHPrompt(promptTestHosts(), "")
	p.Update(tea.WindowSizeMsg{Width: 40, Height: 12})
	cursor := p.cursor
	for _, point := range [][2]int{{-1, 5}, {40, 5}, {4, -1}, {4, 12}, {4, 0}, {4, 11}} {
		p.Update(tea.MouseWheelMsg{X: point[0], Y: point[1], Button: tea.MouseWheelDown})
		assert.Equal(t, cursor, p.cursor)
	}
	p.Update(tea.MouseWheelMsg{X: 4, Y: 5, Button: tea.MouseWheelRight})
	p.Update(tea.MouseWheelMsg{X: 4, Y: 5, Button: tea.MouseWheelDown, Mod: tea.ModShift})
	assert.Equal(t, cursor, p.cursor)
	p.Update(tea.WindowSizeMsg{Width: 8, Height: 4})
	p.Update(tea.MouseWheelMsg{X: 1, Y: 2, Button: tea.MouseWheelDown})
	assert.Equal(t, cursor, p.cursor)
}

func TestPromptLegacyWindowsKeyboardFallback(t *testing.T) {
	previous := isRunningOnOldWindows.Load()
	isRunningOnOldWindows.Store(true)
	t.Cleanup(func() { isRunningOnOldWindows.Store(previous) })
	p := newSSHPrompt(promptTestHosts(), "")
	assert.Equal(t, tea.MouseModeNone, p.View().MouseMode)
	p.Update(tea.MouseWheelMsg{X: 5, Y: 6, Button: tea.MouseWheelDown})
	assert.Equal(t, "开发环境", p.currentHost().Alias)
	pressPrompt(p, "left")
	assert.Zero(t, p.cursor)
	pressPrompt(p, "right")
	assert.Equal(t, 2, p.cursor)
	assert.NotNil(t, pressPrompt(p, "enter"))
	assert.Equal(t, "production-europe-primary", p.selected.Alias)
}

func TestPromptMouseEditorWheelAndIgnoredClicks(t *testing.T) {
	p, _ := editorTestPrompt(t, "")
	p.openEditor(true)
	require.NotNil(t, p.editor)
	for _, width := range []int{40, 100, 140} {
		p.Update(tea.WindowSizeMsg{Width: width, Height: 24})
		e := p.editor
		e.focus, e.caret = 0, 0
		margin := (width - min(width, 112)) / 2
		p.Update(tea.MouseClickMsg{X: margin + 5, Y: 6, Button: tea.MouseLeft})
		assert.Zero(t, e.focus)
		assert.Zero(t, e.caret)
		p.Update(tea.MouseWheelMsg{X: margin, Y: 6, Button: tea.MouseWheelDown})
		assert.Zero(t, e.focus, "border / margin wheels must not change focus")
		p.Update(tea.MouseWheelMsg{X: margin + 5, Y: 6, Button: tea.MouseWheelDown})
		assert.Equal(t, 3, e.focus)
		assert.Equal(t, tea.MouseModeCellMotion, p.View().MouseMode)
	}
}
