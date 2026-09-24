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
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"sync"

	"github.com/google/shlex"
	"github.com/mitchellh/go-homedir"
	"github.com/trzsz/ssh_config"
)

var userHomeDir string

func resolveHomeDir(path string) string {
	if strings.HasPrefix(path, "~/") || strings.HasPrefix(path, "~\\") {
		return filepath.Join(userHomeDir, path[2:])
	}
	return path
}

type sshHost struct {
	Alias         string
	Host          string
	Port          string
	User          string
	IdentityFile  string
	ProxyCommand  string
	ProxyJump     string
	RemoteCommand string
	GroupLabels   string
}

type tsshConfig struct {
	configPath       string
	exConfigPath     string
	loadConfig       sync.Once
	loadExConfig     sync.Once
	loadHosts        sync.Once
	config           *ssh_config.Config
	exConfig         *ssh_config.Config
	allHosts         []*sshHost
	wildcardPatterns []*ssh_config.Pattern
}

var userConfig *tsshConfig

func initUserConfig(configFile string) (err error) {
	userConfig = &tsshConfig{}
	userHomeDir, err = os.UserHomeDir()
	if err != nil {
		debug("user home dir failed: %v", err)
		if userHomeDir, err = homedir.Dir(); err != nil {
			debug("obtain home dir failed: %v", err)
		}
	}
	if userHomeDir == "" {
		warning("Failed to obtain the home directory. Using the current directory as the home directory.")
	}

	userConfig.configPath = filepath.Join(userHomeDir, ".tssh", "config")
	if configFile != "" {
		userConfig.configPath = resolveHomeDir(configFile)
	}
	userConfig.exConfigPath = filepath.Join(userHomeDir, ".tssh", "password")

	return nil
}

func loadConfig(path string) *ssh_config.Config {
	file, err := os.Open(path)
	if err != nil {
		warning("open config [%s] failed: %v", path, err)
		return nil
	}
	defer func() { _ = file.Close() }()
	debug("open config [%s] success", path)

	config, err := ssh_config.Decode(file)
	if err != nil {
		warning("decode config [%s] failed: %v", path, err)
		return nil
	}
	debug("decode config [%s] success", path)
	return config
}

func (c *tsshConfig) doLoadConfig() {
	c.loadConfig.Do(func() {
		ssh_config.SetDefault("LogLevel", "")

		if c.configPath == "" {
			debug("no ssh configuration file path")
			return
		}
		c.config = loadConfig(c.configPath)
	})
}

func (c *tsshConfig) doLoadExConfig() {
	c.loadExConfig.Do(func() {
		if c.exConfigPath == "" {
			debug("no extended configuration file path")
			return
		}
		if !isFileExist(c.exConfigPath) {
			debug("extended config [%s] does not exist", c.exConfigPath)
			return
		}
		c.exConfig = loadConfig(c.exConfigPath)
	})
}

func getConfig(alias, key string) string {
	userConfig.doLoadConfig()

	if userConfig.config != nil {
		value, err := userConfig.config.Get(alias, key)
		if err != nil {
			warning("get user config [%s] for [%s] failed: %v", key, alias, err)
		} else if value != "" {
			return value
		}
	}

	return ssh_config.Default(key)
}

func getConfigSplits(alias, key string) []string {
	userConfig.doLoadConfig()

	if userConfig.config != nil {
		values, err := userConfig.config.GetSplits(alias, key)
		if err != nil {
			warning("get user config splits [%s] for [%s] failed: %v", key, alias, err)
		} else if len(values) > 0 {
			return values
		}
	}

	if value := ssh_config.Default(key); value != "" {
		values, err := shlex.Split(value)
		if err != nil {
			warning("split default [%s] value [%s] failed: %v", key, value, err)
		} else if len(values) > 0 {
			return values
		}
	}

	return nil
}

func getAllConfig(alias, key string) []string {
	userConfig.doLoadConfig()

	var values []string
	if userConfig.config != nil {
		vals, err := userConfig.config.GetAll(alias, key)
		if err != nil {
			warning("get all user config [%s] for [%s] failed: %v", key, alias, err)
		} else if len(vals) > 0 {
			values = append(values, vals...)
		}
	}
	if len(values) > 0 {
		return values
	}

	if value := ssh_config.Default(key); value != "" {
		values = append(values, value)
	}
	return values
}

func getAllConfigSplits(alias, key string) []string {
	userConfig.doLoadConfig()

	var values []string
	if userConfig.config != nil {
		vals, err := userConfig.config.GetAllSplits(alias, key)
		if err != nil {
			warning("get all user config splits [%s] for [%s] failed: %v", key, alias, err)
		} else if len(vals) > 0 {
			values = append(values, vals...)
		}
	}
	if len(values) > 0 {
		return values
	}

	if value := ssh_config.Default(key); value != "" {
		vals, err := shlex.Split(value)
		if err != nil {
			warning("split default [%s] value [%s] failed: %v", key, value, err)
		} else if len(vals) > 0 {
			values = append(values, vals...)
		}
	}
	return values
}

func getExConfig(alias, key string) string {
	userConfig.doLoadExConfig()

	if userConfig.exConfig != nil {
		value, err := userConfig.exConfig.Get(alias, key)
		if err != nil {
			warning("get extended config [%s] for [%s] failed: %v", key, alias, err)
		} else if value != "" {
			debug("get extended config [%s] for [%s] success", key, alias)
			return value
		}
	}

	if value := getConfig(alias, key); value != "" {
		debug("get extended config [%s] for [%s] success", key, alias)
		return value
	}

	debug("no extended config [%s] for [%s]", key, alias)
	return ""
}

func getAllExConfig(alias, key string) []string {
	userConfig.doLoadExConfig()

	var values []string
	if userConfig.exConfig != nil {
		vals, err := userConfig.exConfig.GetAll(alias, key)
		if err != nil {
			warning("get all extended config [%s] for [%s] failed: %v", key, alias, err)
		} else if len(vals) > 0 {
			values = append(values, vals...)
		}
	}
	if vals := getAllConfig(alias, key); len(vals) > 0 {
		values = append(values, vals...)
	}

	return values
}

func getAllHosts() []*sshHost {
	userConfig.loadHosts.Do(func() {
		userConfig.doLoadConfig()
		if userConfig.config != nil {
			userConfig.allHosts = append(userConfig.allHosts, recursiveGetHosts(userConfig.config.Hosts)...)
		}
		addAfterLoginFunc(func() { userConfig.allHosts = nil; userConfig.wildcardPatterns = nil })
	})

	return userConfig.allHosts
}

// recursiveGetHosts recursive get hosts (contains include file's hosts)
func recursiveGetHosts(cfgHosts []*ssh_config.Host) []*sshHost {
	var hosts []*sshHost
	for _, host := range cfgHosts {
		for _, node := range host.Nodes {
			if include, ok := node.(*ssh_config.Include); ok && include != nil {
				for _, config := range include.GetFiles() {
					if config != nil {
						hosts = append(hosts, recursiveGetHosts(config.Hosts)...)
					}
				}
			}
		}
		hosts = appendPromptHosts(hosts, host)
	}
	return hosts
}

func appendPromptHosts(hosts []*sshHost, cfgHosts ...*ssh_config.Host) []*sshHost {
	for _, host := range cfgHosts {
		for _, pattern := range host.Patterns {
			alias := pattern.String()
			if strings.ContainsRune(alias, '*') || strings.ContainsRune(alias, '?') {
				if alias != "*" && !pattern.Not() {
					userConfig.wildcardPatterns = append(userConfig.wildcardPatterns, pattern)
				}
				continue
			}
			if strings.ToLower(getConfig(alias, "HideHost")) == "yes" { // treat as not extended config
				continue
			}
			hosts = append(hosts, &sshHost{
				Alias:         alias,
				Host:          getConfig(alias, "HostName"),
				Port:          getConfig(alias, "Port"),
				User:          getConfig(alias, "User"),
				IdentityFile:  getConfig(alias, "IdentityFile"),
				ProxyCommand:  getConfig(alias, "ProxyCommand"),
				ProxyJump:     getConfig(alias, "ProxyJump"),
				RemoteCommand: getConfig(alias, "RemoteCommand"),
				GroupLabels:   getGroupLabels(alias),
			})
		}
	}
	return hosts
}

func getGroupLabels(alias string) string {
	var groupLabels []string
	addGroupLabel := func(groupLabel string) {
		if slices.Contains(groupLabels, groupLabel) {
			return
		}
		groupLabels = append(groupLabels, groupLabel)
	}
	for _, groupLabel := range getAllExConfig(alias, "GroupLabels") {
		for label := range strings.FieldsSeq(groupLabel) {
			addGroupLabel(label)
		}
	}
	return strings.Join(groupLabels, " ")
}

func getOptionConfig(args *sshArgs, option string) string {
	if value := args.Option.get(option); value != "" {
		return value
	}
	return getConfig(args.Destination, option)
}

func getOptionConfigSplits(args *sshArgs, option string) []string {
	if value := args.Option.get(option); value != "" {
		values, err := shlex.Split(value)
		if err != nil {
			warning("split option [%s] value [%s] failed: %v", option, value, err)
		}
		return values
	}
	return getConfigSplits(args.Destination, option)
}

func getAllOptionConfig(args *sshArgs, option string) []string {
	return append(args.Option.getAll(option), getAllConfig(args.Destination, option)...)
}

func getAllOptionConfigSplits(args *sshArgs, option string) []string {
	var all []string
	for _, value := range args.Option.getAll(option) {
		values, err := shlex.Split(value)
		if err != nil {
			warning("split option [%s] value [%s] failed: %v", option, value, err)
		} else if len(values) > 0 {
			all = append(all, values...)
		}
	}
	values := getAllConfigSplits(args.Destination, option)
	if len(values) > 0 {
		all = append(all, values...)
	}
	return all
}

func getExOptionConfig(args *sshArgs, option string) string {
	if value := args.Option.get(option); value != "" {
		return value
	}
	return getExConfig(args.Destination, option)
}

func getAllExOptionConfig(args *sshArgs, option string) []string {
	return append(args.Option.getAll(option), getAllExConfig(args.Destination, option)...)
}

var secretEncodeKey = []byte("THE_UNSAFE_KEY_FOR_ENCODING_ONLY")

func encodeSecret(secret []byte) (string, error) {
	aesCipher, err := aes.NewCipher(secretEncodeKey)
	if err != nil {
		return "", err
	}
	aesGCM, err := cipher.NewGCM(aesCipher)
	if err != nil {
		return "", err
	}
	nonce := make([]byte, aesGCM.NonceSize())
	if _, err = io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}
	return fmt.Sprintf("%x", aesGCM.Seal(nonce, nonce, secret, nil)), nil
}

func decodeSecret(secret string) (string, error) {
	cipherSecret, err := hex.DecodeString(secret)
	if err != nil {
		return "", err
	}
	aesCipher, err := aes.NewCipher(secretEncodeKey)
	if err != nil {
		return "", err
	}
	aesGCM, err := cipher.NewGCM(aesCipher)
	if err != nil {
		return "", err
	}
	nonceSize := aesGCM.NonceSize()
	if len(cipherSecret) < nonceSize {
		return "", fmt.Errorf("too short")
	}
	plainSecret, err := aesGCM.Open(nil, cipherSecret[:nonceSize], cipherSecret[nonceSize:], nil)
	if err != nil {
		return "", err
	}
	return string(plainSecret), nil
}

func execSecretCommand(param *sshParam, command string) string {
	expanded, err := expandTokens(command, param, "%hnpr")
	if err != nil {
		warning("expand secret command [%s] failed: %v", command, err)
		return ""
	}

	argv, err := splitCommandLine(expanded)
	if err != nil || len(argv) == 0 {
		warning("split secret command [%s] failed: %v", expanded, err)
		return ""
	}
	if enableDebugLogging {
		for i, arg := range argv {
			debug("secret command argv[%d] = %s", i, arg)
		}
	}
	cmd := exec.Command(argv[0], argv[1:]...)
	var outBuf, errBuf bytes.Buffer
	cmd.Stdout = &outBuf
	cmd.Stderr = &errBuf
	if err := cmd.Run(); err != nil {
		if errBuf.Len() > 0 {
			warning("exec secret command [%s] failed: %v, %s", expanded, err, strings.TrimSpace(errBuf.String()))
		} else {
			warning("exec secret command [%s] failed: %v", expanded, err)
		}
		return ""
	}
	if enableDebugLogging && errBuf.Len() > 0 {
		debug("secret command stderr output: %s", errBuf.String())
	}
	return strings.TrimSpace(outBuf.String())
}

func getSecretConfig(param *sshParam, key string) string {
	alias := param.args.Destination
	if value := getExConfig(alias, "enc"+key); value != "" {
		secret, err := decodeSecret(value)
		if err == nil && secret != "" {
			return secret
		}
		warning("decode encrypted configuration [enc%s] for [%s] failed: %v", key, alias, err)
	}
	if command := getExConfig(alias, key+"Command"); command != "" {
		if secret := execSecretCommand(param, command); secret != "" {
			return secret
		}
	}
	return getExConfig(alias, key)
}
