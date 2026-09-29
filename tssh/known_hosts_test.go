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
	"crypto/ed25519"
	"crypto/rand"
	"net"
	"os"
	"path/filepath"
	"testing"

	"github.com/skeema/knownhosts"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/ssh"
)

func testPublicKey(t *testing.T) ssh.PublicKey {
	t.Helper()
	pub, _, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)
	key, err := ssh.NewPublicKey(pub)
	require.NoError(t, err)
	return key
}

func TestHostKeyAliasUsesPinnedAlias(t *testing.T) {
	previous := userConfig
	defer func() { userConfig = previous }()
	userConfig = &tsshConfig{}

	aliasKey, actualKey := testPublicKey(t), testPublicKey(t)
	path := filepath.Join(t.TempDir(), "known_hosts")
	content := knownhosts.Line([]string{"pinned-alias"}, aliasKey) + "\n" +
		knownhosts.Line([]string{"actual-host"}, actualKey) + "\n"
	require.NoError(t, os.WriteFile(path, []byte(content), 0600))
	args := &sshArgs{Destination: "actual-host", Option: sshOption{options: map[string][]string{
		"hostkeyalias":          {"pinned-alias"},
		"userknownhostsfile":    {path},
		"globalknownhostsfile":  {"none"},
		"stricthostkeychecking": {"yes"},
	}}}
	param := &sshParam{args: args, addr: "actual-host:22"}
	callback, algorithms, err := getHostKeyCallback(param)
	require.NoError(t, err)
	assert.Contains(t, algorithms, ssh.KeyAlgoED25519)
	remote := &net.TCPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 22}
	assert.NoError(t, callback("actual-host:22", remote, aliasKey))
}

func TestHostKeyAliasIsSavedUnderAlias(t *testing.T) {
	previous := userConfig
	defer func() { userConfig = previous }()
	userConfig = &tsshConfig{}

	path := filepath.Join(t.TempDir(), "known_hosts")
	require.NoError(t, os.WriteFile(path, nil, 0600))
	args := &sshArgs{Destination: "actual-host", Option: sshOption{options: map[string][]string{
		"hostkeyalias":          {"pinned-alias"},
		"userknownhostsfile":    {path},
		"globalknownhostsfile":  {"none"},
		"stricthostkeychecking": {"accept-new"},
	}}}
	param := &sshParam{args: args, addr: "actual-host:22"}
	callback, _, err := getHostKeyCallback(param)
	require.NoError(t, err)
	remote := &net.TCPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 22}
	assert.NoError(t, callback("actual-host:22", remote, testPublicKey(t)))
	content, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Contains(t, string(content), "pinned-alias ")
	assert.NotContains(t, string(content), "actual-host")
}

func TestHostKeyAcceptanceIsPerLogin(t *testing.T) {
	previous := userConfig
	defer func() { userConfig = previous }()
	userConfig = &tsshConfig{}

	key := testPublicKey(t)
	path := filepath.Join(t.TempDir(), "known_hosts")
	require.NoError(t, os.WriteFile(path, nil, 0600))
	args := &sshArgs{Destination: "new-host", Option: sshOption{options: map[string][]string{
		"userknownhostsfile":    {path},
		"globalknownhostsfile":  {"none"},
		"stricthostkeychecking": {"accept-new"},
	}}}
	remote := &net.TCPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 22}
	previousLogin := &sshParam{args: args, addr: "new-host:22"}
	previousLogin.loginComplete.Store(true)
	callback, _, err := getHostKeyCallback(previousLogin)
	require.NoError(t, err)
	assert.Error(t, callback("new-host:22", remote, key))

	newLogin := &sshParam{args: args, addr: "new-host:22"}
	callback, _, err = getHostKeyCallback(newLogin)
	require.NoError(t, err)
	assert.NoError(t, callback("new-host:22", remote, key))
	content, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Contains(t, string(content), "new-host")
}
