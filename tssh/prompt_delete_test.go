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
)

func TestPromptDeleteConfirmationAndCancel(t *testing.T) {
	original := "Host 开发主机\n User root\nHost other\n User ubuntu\n"
	p, path := editorTestPrompt(t, original)
	for _, key := range []string{"esc", "n", "enter"} {
		pressPrompt(p, "delete")
		require.NotNil(t, p.deleteDialog)
		assert.False(t, p.deleteDialog.confirm)
		view := ansi.Strip(p.View().Content)
		assert.Contains(t, view, "删除主机？")
		assert.Contains(t, view, "开发主机")
		assert.Contains(t, view, "文件：")
		assert.Contains(t, view, "取消")
		pressPrompt(p, key)
		assert.Nil(t, p.deleteDialog)
		data, err := os.ReadFile(path)
		require.NoError(t, err)
		assert.Equal(t, original, string(data))
		assert.Nil(t, p.selected)
	}
	pressPrompt(p, "f6")
	p.Update(tea.MouseWheelMsg{Button: tea.MouseWheelDown})
	p.Update(tea.PasteMsg{Content: "y\n"})
	pressPrompt(p, "j")
	pressPrompt(p, "e")
	assert.Equal(t, "开发主机", p.currentHost().Alias)
	assert.False(t, p.deleteDialog.confirm)
	assert.Nil(t, p.editor)
	pressPrompt(p, "right")
	assert.True(t, p.deleteDialog.confirm)
	pressPrompt(p, "enter")
	require.Nil(t, p.deleteDialog)
	require.Len(t, p.hosts, 1)
	assert.Equal(t, "other", p.currentHost().Alias)
	assert.Contains(t, p.notice, "已删除：开发主机")
	data, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Equal(t, "Host other\n User ubuntu\n", string(data))
}

func TestPromptDeletePreservesSharedAliasesAndOtherSections(t *testing.T) {
	original := "# top\r\nHost one two prod* !prod-excluded # shared\r\n\tUser root\r\n\tCustomOption keep\r\n\r\n" +
		"# next section\r\nHost one\r\n User duplicate\r\n\r\n# match note\r\nMatch host two\r\n Port 2222\r\nHost *\r\n ServerAliveInterval 15\r\n"
	p, path := editorTestPrompt(t, original)
	pressPrompt(p, "f6")
	require.NotNil(t, p.deleteDialog)
	pressPrompt(p, "y")
	require.Nil(t, p.deleteDialog, p.notice)
	for _, host := range p.hosts {
		assert.NotEqual(t, "one", host.Alias)
	}
	assert.Equal(t, "root", getConfig("two", "User"))
	assert.Equal(t, "root", getConfig("prod-other", "User"))
	assert.Equal(t, "2222", getConfig("two", "Port"))
	data, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Contains(t, string(data), "Host two prod* !prod-excluded # shared\r\n\tUser root\r\n\tCustomOption keep\r\n")
	assert.Contains(t, string(data), "# match note\r\nMatch host two\r\n Port 2222\r\nHost *\r\n ServerAliveInterval 15\r\n")
	assert.NotContains(t, strings.ReplaceAll(string(data), "\r\n", ""), "\n")
}

func TestPromptDeleteIncludeAndDuplicateSources(t *testing.T) {
	p, main := editorTestPrompt(t, "")
	included := filepath.Join(filepath.Dir(main), "included")
	mainText := "Include \"" + filepath.ToSlash(included) + "\"\nHost target\n User main\nHost outside\n User outside\n"
	require.NoError(t, os.WriteFile(included, []byte("Host target other\n User inside\n"), 0600))
	require.NoError(t, os.WriteFile(main, []byte(mainText), 0600))
	userConfig = &tsshConfig{configPath: main}
	p.config = userConfig
	p.hosts = getAllHosts()
	p.filterHosts()
	pressPrompt(p, "f6")
	require.NotNil(t, p.deleteDialog)
	require.Len(t, p.deleteDialog.changes, 2)
	pressPrompt(p, "Y")
	require.Nil(t, p.deleteDialog, p.notice)
	for _, host := range p.hosts {
		assert.NotEqual(t, "target", host.Alias)
	}
	assert.Equal(t, "inside", getConfig("other", "User"))
	assert.Equal(t, "outside", getConfig("outside", "User"))
	data, err := os.ReadFile(included)
	require.NoError(t, err)
	assert.Equal(t, "Host other\n User inside\n", string(data))
}

func TestPromptDeleteRejectsExternalChanges(t *testing.T) {
	p, main := editorTestPrompt(t, "Host target\n User root\n")
	pressPrompt(p, "f6")
	require.NotNil(t, p.deleteDialog)
	concurrent := "Host target\n User external\nHost new\n User keep\n"
	require.NoError(t, os.WriteFile(main, []byte(concurrent), 0600))
	pressPrompt(p, "y")
	require.NotNil(t, p.deleteDialog)
	assert.Contains(t, p.deleteDialog.err, "其他程序修改")
	assert.False(t, p.deleteDialog.confirm)
	data, err := os.ReadFile(main)
	require.NoError(t, err)
	assert.Equal(t, concurrent, string(data))
	assert.Nil(t, p.selected)
	pressPrompt(p, "esc")
	assert.Nil(t, p.deleteDialog)
}

func TestPromptDeleteRollsBackWhenIncludeIsNotWritable(t *testing.T) {
	if runtime.GOOS == "windows" || os.Geteuid() == 0 {
		t.Skip("requires Unix directory permissions without root privileges")
	}
	p, main := editorTestPrompt(t, "")
	dir := filepath.Join(filepath.Dir(main), "readonly")
	require.NoError(t, os.Mkdir(dir, 0700))
	included := filepath.Join(dir, "included")
	mainText := "Include \"" + filepath.ToSlash(included) + "\"\nHost target\n User main\n"
	includeText := "Host target\n User included\n"
	require.NoError(t, os.WriteFile(included, []byte(includeText), 0600))
	require.NoError(t, os.WriteFile(main, []byte(mainText), 0600))
	userConfig = &tsshConfig{configPath: main}
	p.config = userConfig
	p.hosts = getAllHosts()
	p.filterHosts()
	pressPrompt(p, "f6")
	require.NotNil(t, p.deleteDialog)
	require.Len(t, p.deleteDialog.changes, 2)
	require.NoError(t, os.Chmod(dir, 0500))
	t.Cleanup(func() { require.NoError(t, os.Chmod(dir, 0700)) })
	pressPrompt(p, "y")
	require.NotNil(t, p.deleteDialog)
	assert.Contains(t, p.deleteDialog.err, "删除失败")
	for path, content := range map[string]string{main: mainText, included: includeText} {
		data, err := os.ReadFile(path)
		require.NoError(t, err)
		assert.Equal(t, content, string(data), path)
	}
}

func TestPromptDeleteLastHostAndPreservesFilter(t *testing.T) {
	p, _ := editorTestPrompt(t, "Host first\n User root\nHost last\n User root\n")
	p.keywords = "last"
	p.filterHosts()
	pressPrompt(p, "f6")
	pressPrompt(p, "y")
	assert.Equal(t, "last", p.keywords)
	assert.Empty(t, p.visible)
	assert.Nil(t, p.currentHost())
	pressPrompt(p, "ctrl+e")
	pressPrompt(p, "delete")
	pressPrompt(p, "y")
	assert.Empty(t, p.hosts)
	assert.Contains(t, ansi.Strip(p.View().Content), "暂无可用主机")
	pressPrompt(p, "f6")
	assert.Nil(t, p.deleteDialog)
	assert.Contains(t, p.notice, "请先选择")
}

func TestPromptDeleteKeysRespectInputAndHelp(t *testing.T) {
	p, _ := editorTestPrompt(t, "Host server\n User root\n")
	pressPrompt(p, "d")
	pressPrompt(p, "D")
	assert.Nil(t, p.deleteDialog)
	pressPrompt(p, "/")
	pressPrompt(p, "d")
	pressPrompt(p, "f6")
	pressPrompt(p, "delete")
	assert.Equal(t, "d", p.query)
	assert.Nil(t, p.deleteDialog)
	pressPrompt(p, "esc")
	pressPrompt(p, "?")
	pressPrompt(p, "f6")
	pressPrompt(p, "delete")
	assert.Nil(t, p.deleteDialog)
	pressPrompt(p, "esc")
	pressPrompt(p, "e")
	pressPrompt(p, "f6")
	assert.NotNil(t, p.editor)
	assert.Nil(t, p.deleteDialog)
}

func TestPromptDeleteDialogLayout(t *testing.T) {
	p, _ := editorTestPrompt(t, "Host 开发主机\n User root\n")
	pressPrompt(p, "f6")
	require.NotNil(t, p.deleteDialog)
	for _, size := range [][2]int{{120, 30}, {80, 24}, {40, 12}, {24, 8}, {8, 4}, {1, 1}} {
		p.Update(tea.WindowSizeMsg{Width: size[0], Height: size[1]})
		for _, errText := range []string{"", "删除失败：" + strings.Repeat("很长的错误路径/", 20)} {
			p.deleteDialog.err = errText
			view := p.View()
			assert.True(t, view.AltScreen)
			lines := strings.Split(view.Content, "\n")
			require.Len(t, lines, size[1])
			for _, line := range lines {
				assert.Equal(t, size[0], ansi.StringWidth(line))
			}
		}
	}
}

func TestPromptDeletePreservesScopedInclude(t *testing.T) {
	p, main := editorTestPrompt(t, "")
	included := filepath.Join(filepath.Dir(main), "included")
	require.NoError(t, os.WriteFile(included, []byte("Host child\n User child-user\n"), 0600))
	require.NoError(t, os.WriteFile(main, []byte("Host target\n User target-user\n Include \""+filepath.ToSlash(included)+"\"\nHost outside\n HostName outside.example\n"), 0600))
	userConfig = &tsshConfig{configPath: main}
	p.config = userConfig
	p.hosts = getAllHosts()
	p.keywords = "target"
	p.filterHosts()
	pressPrompt(p, "f6")
	pressPrompt(p, "y")
	require.Nil(t, p.deleteDialog, p.notice)
	assert.Equal(t, "outside.example", getConfig("outside", "HostName"))
	assert.Empty(t, getConfig("outside", "User"))
	assert.Empty(t, getConfig("child", "User"), "Include must retain its inactive scope")
	for _, host := range p.hosts {
		assert.NotEqual(t, "target", host.Alias)
	}
}
