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
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/trzsz/ssh_config"
	"golang.org/x/crypto/ssh"
)

func promptEntryMap(entries []promptConfigEntry) map[string]promptConfigEntry {
	result := make(map[string]promptConfigEntry)
	for _, entry := range entries {
		result[strings.ToLower(entry.key)] = entry
	}
	return result
}

func TestPromptConfigDefaults(t *testing.T) {
	entries := promptEntryMap(getPromptConfigEntries(&sshHost{Alias: "example"}, nil))
	for key := range ssh_config.Defaults() {
		assert.Contains(t, entries, key)
	}
	assert.Equal(t, "example", entries["hostname"].value)
	assert.Equal(t, "22", entries["port"].value)
	assert.Equal(t, "默认", entries["port"].source)
	assert.NotEmpty(t, entries["user"].value)
	assert.Equal(t, "10", entries["connecttimeout"].value)
	assert.Equal(t, "yes", entries["enablezmodem"].value)
	assert.Equal(t, "no", entries["enableosc52"].value)
	assert.Equal(t, "1", entries["consoleescapetime"].value)
	assert.Equal(t, "30", entries["expecttimeout"].value)
	assert.Equal(t, "未设置", entries["proxycommand"].value)
	assert.Contains(t, entries["identityfile"].value, "~/.ssh/id_ed25519")
	crypto := ssh.Config{}
	crypto.SetDefaults()
	assert.Equal(t, strings.Join(crypto.Ciphers, ","), entries["ciphers"].value)
}

func TestPromptConfigInheritanceIncludesAndExtensions(t *testing.T) {
	included := filepath.Join(t.TempDir(), "hosts.conf")
	require.NoError(t, os.WriteFile(included, []byte(`Host target
    IdentityFile ~/.ssh/second
    CustomIncluded included-value
Host other
    OnlyOther hidden-value
`), 0600))
	main, err := ssh_config.Decode(strings.NewReader(`Host target
    HostName target.example.com
    Port 2222
    IdentityFile ~/.ssh/first
    GroupLabels production
    Password local-secret
    EnableZmodem yes
Include "` + filepath.ToSlash(included) + `"
Host *
    User deploy
    Port 22
    IdentityFile ~/.ssh/common
    LocalForward 8080 localhost:80
    LocalForward 9090 localhost:90
    GroupLabels east
    CustomOption main-value
`))
	require.NoError(t, err)
	extra, err := ssh_config.Decode(strings.NewReader(`Host target
    Password extension-secret
    EnableZmodem no
    GroupLabels database
    CustomOption extension-value
    PasswordCommand touch should-never-run
    Port 9999
`))
	require.NoError(t, err)
	config := &tsshConfig{config: main, exConfig: extra}
	entries := promptEntryMap(getPromptConfigEntries(&sshHost{Alias: "target"}, config))
	assert.Equal(t, "target.example.com", entries["hostname"].value)
	assert.Equal(t, "2222", entries["port"].value)
	assert.Equal(t, "配置", entries["port"].source)
	assert.Equal(t, "deploy", entries["user"].value)
	assert.Equal(t, "~/.ssh/first\n~/.ssh/second\n~/.ssh/common", entries["identityfile"].value)
	assert.Equal(t, "8080 localhost:80\n9090 localhost:90", entries["localforward"].value)
	assert.Equal(t, "database\nproduction\neast", entries["grouplabels"].value)
	assert.Equal(t, "no", entries["enablezmodem"].value)
	assert.Equal(t, "扩展配置", entries["enablezmodem"].source)
	assert.Equal(t, "included-value", entries["customincluded"].value)
	assert.Equal(t, "extension-value", entries["customoption"].value)
	assert.NotContains(t, entries, "onlyother")
	assert.Equal(t, "••••••（已设置）", entries["password"].value)
	assert.Equal(t, "••••••（已设置）", entries["passwordcommand"].value)
	assert.Equal(t, "yes", entries["passwordauthentication"].value)
}

func TestPromptSecretFields(t *testing.T) {
	for _, key := range []string{"Password", "encPassword", "Passphrase", "QuestionAnswer1", "TotpSecret1", "ExpectSendText1", "CtrlExpectSendTotp1", "6e616d653a20", "totp636f64653a20"} {
		assert.True(t, promptSecretKey(key), key)
	}
	for _, key := range []string{"PasswordAuthentication", "NumberOfPasswordPrompts", "ExpectPassSleep", "IdentityFile", "User"} {
		assert.False(t, promptSecretKey(key), key)
	}
}
