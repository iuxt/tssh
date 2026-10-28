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
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"

	"github.com/skeema/knownhosts"
	"golang.org/x/crypto/ssh"
	xknownhosts "golang.org/x/crypto/ssh/knownhosts"
)

var acceptHostKeys []string
var addHostKeyMutex sync.Mutex

func ensureNewline(file *os.File) error {
	if _, err := file.Seek(-1, io.SeekEnd); err != nil {
		return nil
	}
	buf := make([]byte, 1)
	if n, err := file.Read(buf); err != nil || n != 1 || buf[0] == '\n' {
		return nil
	}
	if _, err := file.Write([]byte("\n")); err != nil {
		return err
	}
	return nil
}

func writeKnownHost(path, host string, key ssh.PublicKey) error {
	file, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR|os.O_APPEND, 0600)
	if err != nil {
		return err
	}
	defer func() { _ = file.Close() }()
	if err := ensureNewline(file); err != nil {
		return err
	}

	hostNormalized := knownhosts.Normalize(host)
	if strings.ContainsAny(hostNormalized, "\t ") {
		return fmt.Errorf("host '%s' contains spaces", hostNormalized)
	}
	line := knownhosts.Line([]string{hostNormalized}, key) + "\n"
	return writeAll(file, []byte(line))
}

func addHostKey(path, host string, key ssh.PublicKey, ask, loginComplete bool) error {
	addHostKeyMutex.Lock()
	defer addHostKeyMutex.Unlock()
	keyNormalizedLine := knownhosts.Line([]string{host}, key)
	if slices.Contains(acceptHostKeys, keyNormalizedLine) {
		if enableDebugLogging {
			debug("host key [%s] has been accepted", ssh.FingerprintSHA256(key))
		}
		return nil
	}

	if loginComplete {
		warning("The public key of the remote server has changed after login")
		return fmt.Errorf("host key changed")
	}

	// writing only during the login process with the user's permission
	if ask {
		fingerprint := ssh.FingerprintSHA256(key)
		fmt.Fprintf(os.Stderr, "The authenticity of host '%s' can't be established.\r\n"+
			"%s key fingerprint is %s.\r\n", host, shortKeyType(key.Type()), fingerprint)

		if err := confirmHostKey(key, "Are you sure you want to continue connecting (yes/no/[fingerprint])? "); err != nil {
			return err
		}
	}

	acceptHostKeys = append(acceptHostKeys, keyNormalizedLine)

	if err := writeKnownHost(path, host, key); err != nil {
		warning("Failed to add the host to the list of known hosts (%s): %v", path, err)
		return nil
	}

	warning("Permanently added '%s' (%s) to the list of known hosts.", host, shortKeyType(key.Type()))
	return nil
}

// confirmHostKey requires an explicit answer; empty input never accepts a key.
func confirmHostKey(key ssh.PublicKey, prompt string) error {
	stdin, closer, err := getKeyboardInput()
	if err != nil {
		return err
	}
	defer closer()
	reader := bufio.NewReader(stdin)
	_, _ = os.Stderr.WriteString(prompt)
	for {
		input, err := reader.ReadString('\n')
		if err != nil {
			return err
		}
		input = strings.TrimSpace(input)
		if input == ssh.FingerprintSHA256(key) || strings.EqualFold(input, "yes") {
			return nil
		}
		if strings.EqualFold(input, "no") {
			return fmt.Errorf("host key not trusted")
		}
		_, _ = os.Stderr.WriteString("Please type 'yes', 'no' or the fingerprint: ")
	}
}

// replaceKnownHost removes only this host's trust in the conflicting keys.
// Other aliases sharing a line keep their original key.
func replaceKnownHost(path, host string, key ssh.PublicKey, want []xknownhosts.KnownKey) error {
	original, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	lines := strings.SplitAfter(string(original), "\n")
	normalized := knownhosts.Normalize(host)
	for _, old := range want {
		if old.Line < 1 || old.Line > len(lines) {
			return fmt.Errorf("known hosts file changed: %s", path)
		}
		line := lines[old.Line-1]
		marker, hosts, oldKey, _, _, err := ssh.ParseKnownHosts([]byte(line))
		if err != nil || marker != "" || !bytes.Equal(oldKey.Marshal(), old.Key.Marshal()) {
			return fmt.Errorf("known hosts entry changed or is not a host key: %s:%d", path, old.Line)
		}
		var remaining []string
		for _, name := range hosts {
			if name == normalized || strings.HasPrefix(name, "|") {
				continue
			}
			remaining = append(remaining, name)
		}
		if len(remaining) == 0 {
			lines[old.Line-1] = ""
		} else {
			// Negation also excludes this host from wildcard entries, without
			// changing the key trusted by any other host matching the pattern.
			remaining = append([]string{"!" + normalized}, remaining...)
			trimmed := strings.TrimLeft(line, " \t")
			end := strings.IndexAny(trimmed, " \t")
			lines[old.Line-1] = strings.Join(remaining, ",") + trimmed[end:]
		}
	}
	content := strings.Join(lines, "")
	if content != "" && !strings.HasSuffix(content, "\n") {
		content += "\n"
	}
	content += knownhosts.Line([]string{normalized}, key) + "\n"

	// Follow symlinks and atomically replace the target, keeping its permissions.
	target, err := filepath.EvalSymlinks(path)
	if err != nil {
		return err
	}
	info, err := os.Stat(target)
	if err != nil {
		return err
	}
	file, err := os.CreateTemp(filepath.Dir(target), ".tssh-known-hosts-*")
	if err != nil {
		return err
	}
	defer os.Remove(file.Name())
	defer file.Close()
	if err := file.Chmod(info.Mode().Perm()); err != nil {
		return err
	}
	if err := writeAll(file, []byte(content)); err != nil {
		return err
	}
	if err := file.Sync(); err != nil {
		return err
	}
	if err := file.Close(); err != nil {
		return err
	}
	current, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	if !bytes.Equal(current, original) {
		return fmt.Errorf("known hosts file changed: %s", path)
	}
	return os.Rename(file.Name(), target)
}

func updateHostKey(files, userFiles []string, host string, remote net.Addr, key ssh.PublicKey, loginComplete bool) error {
	addHostKeyMutex.Lock()
	defer addHostKeyMutex.Unlock()
	if loginComplete {
		return fmt.Errorf("host key changed after login")
	}
	// Reload under the lock so simultaneous connections only prompt once.
	db, err := knownhosts.NewDB(files...)
	if err != nil {
		return err
	}
	err = db.HostKeyCallback()(host, remote, key)
	if err == nil {
		return nil
	}
	var keyErr *xknownhosts.KeyError
	if !errors.As(err, &keyErr) || len(keyErr.Want) == 0 {
		return err
	}
	conflicts := make(map[string][]xknownhosts.KnownKey)
	for _, old := range keyErr.Want {
		if !slices.Contains(userFiles, old.Filename) {
			return fmt.Errorf("host key conflict in global known hosts file %s:%d; update it manually", old.Filename, old.Line)
		}
		conflicts[old.Filename] = append(conflicts[old.Filename], old)
	}
	if err := confirmHostKey(key, "Replace the old host key and continue connecting (yes/no/[fingerprint])? "); err != nil {
		return err
	}
	for path, want := range conflicts {
		if err := replaceKnownHost(path, host, key, want); err != nil {
			return fmt.Errorf("update known hosts (%s): %w", path, err)
		}
	}
	warning("Updated the host key for '%s' (%s) in known hosts.", host, shortKeyType(key.Type()))
	return nil
}

func getHostKeyCallback(param *sshParam) (ssh.HostKeyCallback, []string, error) {
	primaryPath := ""
	var files, userFiles []string
	addKnownHostsFiles := func(key string, user bool) error {
		knownHostsFiles := getOptionConfigSplits(param.args, key)
		if len(knownHostsFiles) == 0 {
			debug("%s is empty", key)
			return nil
		}
		if len(knownHostsFiles) == 1 && strings.ToLower(knownHostsFiles[0]) == "none" {
			debug("%s is none", key)
			return nil
		}
		for _, path := range knownHostsFiles {
			var resolvedPath string
			if user {
				expandedPath, err := expandTokens(path, param, "%CdhijkLlnpru")
				if err != nil {
					return fmt.Errorf("expand UserKnownHostsFile [%s] failed: %v", path, err)
				}
				resolvedPath = resolveHomeDir(expandedPath)
				if primaryPath == "" {
					primaryPath = resolvedPath
				}
			} else {
				resolvedPath = path
			}
			if !isFileExist(resolvedPath) {
				debug("%s [%s] does not exist", key, resolvedPath)
				continue
			}
			if !canReadFile(resolvedPath) {
				if user {
					warning("%s [%s] can't be read", key, resolvedPath)
				} else {
					debug("%s [%s] can't be read", key, resolvedPath)
				}
				continue
			}
			debug("add %s: %s", key, resolvedPath)
			if !slices.Contains(files, resolvedPath) {
				files = append(files, resolvedPath)
			}
			if user {
				userFiles = append(userFiles, resolvedPath)
			}
		}
		return nil
	}
	if err := addKnownHostsFiles("UserKnownHostsFile", true); err != nil {
		return nil, nil, err
	}
	if err := addKnownHostsFiles("GlobalKnownHostsFile", false); err != nil {
		return nil, nil, err
	}

	khdb, err := knownhosts.NewDB(files...)
	if err != nil {
		return nil, nil, fmt.Errorf("new knownhosts failed: %v", err)
	}

	hostKeyName := param.addr
	if alias := getOptionConfig(param.args, "HostKeyAlias"); alias != "" {
		// Use the alias as the known_hosts identity even on nonstandard SSH ports.
		hostKeyName = net.JoinHostPort(alias, "22")
	}
	var callbackMutex sync.Mutex
	hostKeyCallback := func(_ string, remote net.Addr, key ssh.PublicKey) error {
		callbackMutex.Lock()
		defer callbackMutex.Unlock()
		err := khdb.HostKeyCallback()(hostKeyName, remote, key)
		if err == nil {
			return nil
		}
		strictHostKeyChecking := strings.ToLower(getOptionConfig(param.args, "StrictHostKeyChecking"))
		if knownhosts.IsHostKeyChanged(err) {
			path := primaryPath
			if path == "" {
				path = "~/.ssh/known_hosts"
			}
			fmt.Fprintf(os.Stderr, "\033[0;31m@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@\r\n"+
				"@    WARNING: REMOTE HOST IDENTIFICATION HAS CHANGED!     @\r\n"+
				"@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@\r\n"+
				"IT IS POSSIBLE THAT SOMEONE IS DOING SOMETHING NASTY!\r\n"+
				"Someone could be eavesdropping on you right now (man-in-the-middle attack)!\033[0m\r\n"+
				"It is also possible that a host key has just been changed.\r\n"+
				"The fingerprint for the %s key sent by the remote host is\r\n"+
				"%s\r\n"+
				"Please contact your system administrator.\r\n"+
				"Add correct host key in %s to get rid of this message.\r\n",
				shortKeyType(key.Type()), ssh.FingerprintSHA256(key), path)
			switch strictHostKeyChecking {
			case "yes", "true", "accept-new":
				return err
			default:
				if err := updateHostKey(files, userFiles, hostKeyName, remote, key, param.loginComplete.Load()); err != nil {
					return err
				}
				// Forget the old keys in this callback as soon as the file changes.
				updated, reloadErr := knownhosts.NewDB(files...)
				if reloadErr != nil {
					return reloadErr
				}
				khdb = updated
				return nil
			}
		} else if knownhosts.IsHostUnknown(err) && primaryPath != "" {
			ask := true
			switch strictHostKeyChecking {
			case "yes", "true":
				return err
			case "accept-new", "no", "off", "false":
				ask = false
			}
			return addHostKey(primaryPath, hostKeyName, key, ask, param.loginComplete.Load())
		}
		switch strictHostKeyChecking {
		case "no", "off", "false":
			return nil
		default:
			return err
		}
	}

	return hostKeyCallback, khdb.HostKeyAlgorithms(hostKeyName), err
}
