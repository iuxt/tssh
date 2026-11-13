package tssh

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/charmbracelet/x/ansi"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func promptHostAliases(hosts []*sshHost) []string {
	aliases := make([]string, len(hosts))
	for i, host := range hosts {
		aliases[i] = host.Alias
	}
	return aliases
}

func TestPromptReorderPersistsWithoutChangingSSHConfig(t *testing.T) {
	original := "Host one\n User first\nHost two\n User second\nHost three\n"
	p, path := editorTestPrompt(t, original)
	host := p.currentHost()
	pressPrompt(p, "pgup") // Already first: do not create metadata.
	_, err := os.Stat(p.config.hostOrderPath())
	assert.True(t, os.IsNotExist(err))
	pressPrompt(p, "pgdown")
	assert.Equal(t, []string{"two", "one", "three"}, promptHostAliases(p.visible))
	assert.Equal(t, 1, p.cursor)
	assert.Same(t, host, p.currentHost())
	assert.Contains(t, p.notice, "已保存主机顺序")
	assert.Contains(t, ansi.Strip(p.View().Content), "PgUp/Dn")
	pressPrompt(p, "pgdown")
	pressPrompt(p, "pgdown") // Already last.
	assert.Equal(t, 2, p.cursor)
	assert.Same(t, host, p.currentHost())
	pressPrompt(p, "pgup")
	want := []string{"two", "one", "three"}
	assert.Equal(t, want, promptHostAliases(p.hosts))
	data, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Equal(t, original, string(data))
	userConfig = &tsshConfig{configPath: path}
	assert.Equal(t, want, promptHostAliases(getAllHosts()))
	assert.Equal(t, "first", getConfig("one", "User"))

	// Editing and deleting reload the list and must retain the saved order.
	p.config = userConfig
	p.openEditor(false)
	require.NotNil(t, p.editor)
	setEditorField(t, p.editor, "User", "changed")
	pressPrompt(p, "ctrl+s")
	require.Nil(t, p.editor)
	assert.Equal(t, want, promptHostAliases(p.hosts))
	pressPrompt(p, "delete")
	require.NotNil(t, p.deleteDialog)
	pressPrompt(p, "y")
	require.Nil(t, p.deleteDialog)
	assert.Equal(t, []string{"two", "three"}, promptHostAliases(p.hosts))
}

func TestPromptReorderWithLockedFilter(t *testing.T) {
	p, path := editorTestPrompt(t, "Host prod-one\nHost hidden\nHost prod-two\nHost tail\n")
	pressPrompt(p, "/")
	pressPrompt(p, "prod")
	pressPrompt(p, "enter")
	host := p.currentHost()
	pressPrompt(p, "pgdown")
	assert.Equal(t, []string{"prod-two", "prod-one"}, promptHostAliases(p.visible))
	assert.Equal(t, []string{"prod-two", "hidden", "prod-one", "tail"}, promptHostAliases(p.hosts))
	assert.Same(t, host, p.currentHost())
	pressPrompt(p, "ctrl+e")
	userConfig = &tsshConfig{configPath: path}
	assert.Equal(t, promptHostAliases(p.hosts), promptHostAliases(getAllHosts()))
}

func TestPromptReorderAcrossIncludeFiles(t *testing.T) {
	p, path := editorTestPrompt(t, "")
	included := filepath.Join(filepath.Dir(path), "included")
	mainText := "Include \"" + filepath.ToSlash(included) + "\"\nHost main\n User primary\n"
	includeText := "Host included\n User secondary\n"
	require.NoError(t, os.WriteFile(path, []byte(mainText), 0600))
	require.NoError(t, os.WriteFile(included, []byte(includeText), 0600))
	userConfig = &tsshConfig{configPath: path}
	p.config, p.hosts = userConfig, getAllHosts()
	p.filterHosts()
	assert.Equal(t, []string{"included", "main"}, promptHostAliases(p.hosts))
	pressPrompt(p, "pgdown")
	userConfig = &tsshConfig{configPath: path}
	assert.Equal(t, []string{"main", "included"}, promptHostAliases(getAllHosts()))
	for file, want := range map[string]string{path: mainText, included: includeText} {
		data, err := os.ReadFile(file)
		require.NoError(t, err)
		assert.Equal(t, want, string(data))
	}
}

func TestPromptPageKeysKeepNavigationInOtherPanels(t *testing.T) {
	p, _ := editorTestPrompt(t, "Host one\nHost two\nHost three\n")
	want := promptHostAliases(p.hosts)
	pressPrompt(p, "/")
	pressPrompt(p, "pgdown")
	assert.Equal(t, 2, p.cursor)
	pressPrompt(p, "pgup")
	assert.Equal(t, 0, p.cursor)
	pressPrompt(p, "esc")
	pressPrompt(p, "?")
	pressPrompt(p, "pgdown")
	assert.Greater(t, p.helpOffset, 0)
	pressPrompt(p, "pgup")
	assert.Zero(t, p.helpOffset)
	pressPrompt(p, "esc")
	pressPrompt(p, "v")
	pressPrompt(p, "a")
	pressPrompt(p, "pgdown")
	assert.Greater(t, p.detailOffset, 0)
	pressPrompt(p, "pgup")
	assert.Zero(t, p.detailOffset)
	assert.Equal(t, want, promptHostAliases(p.hosts))
	_, err := os.Stat(p.config.hostOrderPath())
	assert.True(t, os.IsNotExist(err))
}

func TestPromptReorderSaveFailureKeepsList(t *testing.T) {
	p, _ := editorTestPrompt(t, "Host one\nHost two\n")
	require.NoError(t, os.Mkdir(p.config.hostOrderPath(), 0700))
	pressPrompt(p, "pgdown")
	assert.Contains(t, p.notice, "保存主机顺序失败")
	assert.Equal(t, []string{"one", "two"}, promptHostAliases(p.hosts))
	assert.Equal(t, []string{"one", "two"}, promptHostAliases(p.visible))
	assert.Zero(t, p.cursor)
}

func TestHostOrderHandlesDuplicatesRemovedAndNewHosts(t *testing.T) {
	p, path := editorTestPrompt(t, "Host one\n User first\nHost new\nHost two\nHost one\n User second\n")
	require.NoError(t, os.WriteFile(p.config.hostOrderPath(), []byte(`["one", "missing", "two", "one"]`), 0600))
	userConfig = &tsshConfig{configPath: path}
	assert.Equal(t, []string{"one", "two", "one", "new"}, promptHostAliases(getAllHosts()))
	// Bad metadata must not hide otherwise usable SSH hosts.
	require.NoError(t, os.WriteFile(p.config.hostOrderPath(), []byte(`invalid`), 0600))
	userConfig = &tsshConfig{configPath: path}
	assert.Equal(t, []string{"one", "new", "two", "one"}, promptHostAliases(getAllHosts()))
}

func TestPromptReorderEmptyList(t *testing.T) {
	p := newSSHPrompt(nil, "")
	pressPrompt(p, "pgup")
	pressPrompt(p, "pgdown")
	assert.Empty(t, p.hosts)
	assert.Zero(t, p.cursor)
}
