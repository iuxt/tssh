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
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/trzsz/ssh_config"
)

func TestInitUserConfigDoesNotLoadGlobalConfig(t *testing.T) {
	originalConfig, originalHome := userConfig, userHomeDir
	defer func() { userConfig, userHomeDir = originalConfig, originalHome }()

	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	xdgConfigHome := filepath.Join(home, "xdg")
	t.Setenv("XDG_CONFIG_HOME", xdgConfigHome)
	require.NoError(t, os.WriteFile(filepath.Join(home, ".tssh.conf"),
		[]byte("ConfigPath = /legacy/config\nPromptThemeLayout = table\n"), 0600))
	require.NoError(t, os.MkdirAll(filepath.Join(xdgConfigHome, "tssh"), 0700))
	require.NoError(t, os.WriteFile(filepath.Join(xdgConfigHome, "tssh", "tssh.conf"),
		[]byte("ConfigPath = /xdg/config\nExConfigPath = /xdg/password\n"), 0600))

	require.NoError(t, initUserConfig(""))
	assert.Equal(t, filepath.Join(home, ".tssh", "config"), userConfig.configPath)
	assert.Equal(t, filepath.Join(home, ".tssh", "password"), userConfig.exConfigPath)

	require.NoError(t, initUserConfig("~/custom-config"))
	assert.Equal(t, filepath.Join(home, "custom-config"), userConfig.configPath)
}

func TestSplitOptionConfigValueWindowsKnownHostsPaths(t *testing.T) {
	paths, err := splitOptionConfigValue(`D:\a\tssh\known_hosts "C:\Program Files\ssh\known_hosts"`,
		"UserKnownHostsFile", "windows")
	require.NoError(t, err)
	assert.Equal(t, []string{"D:/a/tssh/known_hosts", "C:/Program Files/ssh/known_hosts"}, paths)

	paths, err = splitOptionConfigValue(`D:\a\tssh\known_hosts`, "GlobalKnownHostsFile", "windows")
	require.NoError(t, err)
	assert.Equal(t, []string{"D:/a/tssh/known_hosts"}, paths)
}

func TestWindowsKnownHostsPathFromConfig(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("requires native Windows path handling")
	}
	previous := userConfig
	defer func() { userConfig = previous }()

	config, err := ssh_config.Decode(strings.NewReader("Host test-host\n  UserKnownHostsFile C:\\Users\\test\\known_hosts\n"))
	require.NoError(t, err)
	userConfig = &tsshConfig{config: config}
	args := &sshArgs{Destination: "test-host"}
	assert.Equal(t, []string{"C:/Users/test/known_hosts"}, getOptionConfigSplits(args, "UserKnownHostsFile"))
}
