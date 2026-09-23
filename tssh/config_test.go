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
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestInitUserConfigDoesNotLoadGlobalConfig(t *testing.T) {
	originalConfig, originalHome := userConfig, userHomeDir
	defer func() { userConfig, userHomeDir = originalConfig, originalHome }()

	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, "xdg"))
	require.NoError(t, os.WriteFile(filepath.Join(home, ".tssh.conf"),
		[]byte("ConfigPath = /legacy/config\nPromptThemeLayout = table\n"), 0600))

	require.NoError(t, initUserConfig(""))
	assert.Equal(t, filepath.Join(home, ".tssh", "config"), userConfig.configPath)
	assert.Equal(t, filepath.Join(home, ".tssh", "password"), userConfig.exConfigPath)

	require.NoError(t, initUserConfig("~/custom-config"))
	assert.Equal(t, filepath.Join(home, "custom-config"), userConfig.configPath)
}
