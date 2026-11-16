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
	xknownhosts "golang.org/x/crypto/ssh/knownhosts"
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

func withHostKeyInput(t *testing.T, input string) {
	t.Helper()
	stdin, terminal := os.Stdin, isTerminal
	file, err := os.CreateTemp(t.TempDir(), "stdin")
	require.NoError(t, err)
	_, err = file.WriteString(input)
	require.NoError(t, err)
	_, err = file.Seek(0, 0)
	require.NoError(t, err)
	os.Stdin, isTerminal = file, true
	t.Cleanup(func() { os.Stdin, isTerminal = stdin, terminal; file.Close() })
}

func TestChangedHostKeyConfirmation(t *testing.T) {
	previous := userConfig
	t.Cleanup(func() { userConfig = previous })
	userConfig = &tsshConfig{}
	for _, tc := range []struct {
		name, mode, input string
		accept            bool
	}{
		{"confirm", "ask", "yes\n", true},
		{"default", "", "yes\n", true},
		{"reject", "ask", "no\n", false},
		{"eof", "ask", "", false},
		{"empty then reject", "ask", "\nno\n", false},
		{"strict", "yes", "yes\n", false},
		{"accept new only", "accept-new", "yes\n", false},
		{"disabled still confirms change", "no", "no\n", false},
		{"disabled explicit confirmation", "no", "yes\n", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			withHostKeyInput(t, tc.input)
			oldKey, newKey := testPublicKey(t), testPublicKey(t)
			path := filepath.Join(t.TempDir(), "known_hosts")
			original := knownhosts.Line([]string{"changed-host"}, oldKey) + "\n"
			require.NoError(t, os.WriteFile(path, []byte(original), 0600))
			args := &sshArgs{Destination: "changed-host", Option: sshOption{options: map[string][]string{
				"userknownhostsfile": {path}, "globalknownhostsfile": {"none"}, "stricthostkeychecking": {tc.mode},
			}}}
			callback, _, err := getHostKeyCallback(&sshParam{args: args, addr: "changed-host:22"})
			require.NoError(t, err)
			remote := &net.TCPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 22}
			err = callback("changed-host:22", remote, newKey)
			if !tc.accept {
				require.Error(t, err)
				content, err := os.ReadFile(path)
				require.NoError(t, err)
				assert.Equal(t, original, string(content))
				return
			}
			require.NoError(t, err)
			// The original callback and a new login both accept the saved key.
			require.NoError(t, callback("changed-host:22", remote, newKey))
			require.Error(t, callback("changed-host:22", remote, oldKey))
			db, err := knownhosts.NewDB(path)
			require.NoError(t, err)
			assert.NoError(t, db.HostKeyCallback()("changed-host:22", remote, newKey))
			assert.Error(t, db.HostKeyCallback()("changed-host:22", remote, oldKey))
		})
	}
}

func TestReplaceKnownHostPreservesOtherHosts(t *testing.T) {
	for _, hosts := range [][]string{
		{"changed-host"},
		{"changed-host", "other-host"},
		{xknownhosts.HashHostname("changed-host")},
		{"*-host"},
		{"[changed-host]:2222", "other-host"},
	} {
		t.Run(hosts[0], func(t *testing.T) {
			oldKey, newKey, unrelated := testPublicKey(t), testPublicKey(t), testPublicKey(t)
			path := filepath.Join(t.TempDir(), "known_hosts")
			content := "# keep this comment\n" + knownhosts.Line([]string{"unrelated"}, unrelated) + "\n" + knownhosts.Line(hosts, oldKey) + " comment"
			require.NoError(t, os.WriteFile(path, []byte(content), 0640))
			// Compare the permissions reported by the platform; Windows does not preserve Unix mode bits.
			originalInfo, err := os.Stat(path)
			require.NoError(t, err)
			host := "changed-host:22"
			if hosts[0] == "[changed-host]:2222" {
				host = "changed-host:2222"
			}
			remote := &net.TCPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 22}
			db, err := knownhosts.NewDB(path)
			require.NoError(t, err)
			var keyErr *xknownhosts.KeyError
			require.ErrorAs(t, db.HostKeyCallback()(host, remote, newKey), &keyErr)
			require.NoError(t, replaceKnownHost(path, host, newKey, keyErr.Want))
			db, err = knownhosts.NewDB(path)
			require.NoError(t, err)
			assert.NoError(t, db.HostKeyCallback()(host, remote, newKey))
			assert.Error(t, db.HostKeyCallback()(host, remote, oldKey))
			assert.NoError(t, db.HostKeyCallback()("unrelated:22", remote, unrelated))
			if len(hosts) > 1 || hosts[0] == "*-host" {
				assert.NoError(t, db.HostKeyCallback()("other-host:22", remote, oldKey))
			}
			updated, err := os.ReadFile(path)
			require.NoError(t, err)
			assert.Contains(t, string(updated), "# keep this comment\n")
			info, err := os.Stat(path)
			require.NoError(t, err)
			assert.Equal(t, originalInfo.Mode().Perm(), info.Mode().Perm())
		})
	}
}

func TestChangedHostKeyAfterLoginIsRejected(t *testing.T) {
	withHostKeyInput(t, "yes\n")
	path := filepath.Join(t.TempDir(), "known_hosts")
	oldKey, newKey := testPublicKey(t), testPublicKey(t)
	original := knownhosts.Line([]string{"changed-host"}, oldKey) + "\n"
	require.NoError(t, os.WriteFile(path, []byte(original), 0600))
	remote := &net.TCPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 22}
	require.Error(t, updateHostKey([]string{path}, []string{path}, "changed-host:22", remote, newKey, true))
	content, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Equal(t, original, string(content))
}

func TestUpdateHostKeyMultipleFiles(t *testing.T) {
	newKey := testPublicKey(t)
	withHostKeyInput(t, ssh.FingerprintSHA256(newKey)+"\n")
	var files []string
	for range 2 {
		path := filepath.Join(t.TempDir(), "known_hosts")
		require.NoError(t, os.WriteFile(path, []byte(knownhosts.Line([]string{"changed-host"}, testPublicKey(t))+"\n"), 0600))
		files = append(files, path)
	}
	remote := &net.TCPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 22}
	require.NoError(t, updateHostKey(files, files, "changed-host:22", remote, newKey, false))
	for _, path := range files {
		db, err := knownhosts.NewDB(path)
		require.NoError(t, err)
		assert.NoError(t, db.HostKeyCallback()("changed-host:22", remote, newKey))
	}
}

func TestUpdateGlobalHostKeyIsRejected(t *testing.T) {
	withHostKeyInput(t, "yes\n")
	path := filepath.Join(t.TempDir(), "known_hosts")
	original := knownhosts.Line([]string{"changed-host"}, testPublicKey(t)) + "\n"
	require.NoError(t, os.WriteFile(path, []byte(original), 0600))
	remote := &net.TCPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 22}
	err := updateHostKey([]string{path}, nil, "changed-host:22", remote, testPublicKey(t), false)
	require.ErrorContains(t, err, "global known hosts file")
	content, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Equal(t, original, string(content))
}

func TestReplaceKnownHostFollowsSymlink(t *testing.T) {
	path := filepath.Join(t.TempDir(), "known_hosts")
	link := filepath.Join(t.TempDir(), "known_hosts_link")
	oldKey, newKey := testPublicKey(t), testPublicKey(t)
	require.NoError(t, os.WriteFile(path, []byte(knownhosts.Line([]string{"changed-host"}, oldKey)), 0600))
	if err := os.Symlink(path, link); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}
	want := []xknownhosts.KnownKey{{Key: oldKey, Filename: link, Line: 1}}
	require.NoError(t, replaceKnownHost(link, "changed-host:22", newKey, want))
	info, err := os.Lstat(link)
	require.NoError(t, err)
	assert.NotZero(t, info.Mode()&os.ModeSymlink)
	db, err := knownhosts.NewDB(path)
	require.NoError(t, err)
	remote := &net.TCPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 22}
	assert.NoError(t, db.HostKeyCallback()("changed-host:22", remote, newKey))
}
