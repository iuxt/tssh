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
	"encoding/hex"
	"os/user"
	"slices"
	"strconv"
	"strings"

	"github.com/trzsz/ssh_config"
	"golang.org/x/crypto/ssh"
)

type promptConfigEntry struct {
	key, value, source string
}

// Values left empty by the parser can have defaults in the connection code.
// Keep those fallbacks here separate from the parser's OpenSSH defaults.
func promptRuntimeDefaults(alias string) map[string]string {
	username := "当前本地用户"
	if current, err := user.Current(); err == nil {
		username = current.Username
		if index := strings.LastIndexByte(username, '\\'); index >= 0 {
			username = username[index+1:]
		}
	}
	crypto := ssh.Config{}
	crypto.SetDefaults()
	return map[string]string{
		"HostName": alias, "User": username,
		"ConnectTimeout":    strconv.Itoa(int(kDefaultConnectTimeout.Seconds())),
		"Ciphers":           strings.Join(crypto.Ciphers, ","),
		"LogLevel":          "默认日志级别（显示警告）",
		"XAuthLocation":     "xauth（从 PATH 查找）",
		"ForwardX11Trusted": "自动（远程 SSH 会话 yes，本地会话 no）",
		"IdentityFile":      "~/.ssh/id_rsa\n~/.ssh/id_ecdsa\n~/.ssh/id_ecdsa_sk\n~/.ssh/id_ed25519\n~/.ssh/id_ed25519_sk\n~/.ssh/identity",
		"IdentityAgent":     "SSH_AUTH_SOCK / 系统默认代理",
		"ProxyCommand":      "", "ProxyJump": "", "RemoteCommand": "", "LocalCommand": "",
		"CertificateFile": "", "HostKeyAlias": "", "ControlPath": "",
		"Password": "", "Passphrase": "",
		"CanonicalDomains": "", "SendEnv": "", "SetEnv": "",
		"GroupLabels": "", "HideHost": "no",
		"LocalForward": "", "RemoteForward": "", "DynamicForward": "",
		"EnableTrzsz": "yes", "EnableZmodem": "yes", "EnableOSC52": "no", "EnableWaypipe": "no",
		"EnableDragFile": "no", "DragFileUploadCommand": "rz",
		"SetTerminalTitle": "no", "ConsoleEscapeTime": "1", "DnsSrvName": "",
		"DefaultUploadPath": "", "DefaultDownloadPath": "", "ProgressColorPair": "",
		"WaypipeClientPath": "waypipe", "WaypipeServerPath": "waypipe",
		"WaypipeClientOption": "", "WaypipeServerOption": "",
		"ExpectCount": "0", "CtrlExpectCount": "0",
		"ExpectTimeout": strconv.Itoa(kDefaultExpectTimeout), "CtrlExpectTimeout": strconv.Itoa(kDefaultExpectTimeout),
		"ExpectSleepMS": strconv.Itoa(kDefaultExpectSleepMS), "CtrlExpectSleepMS": strconv.Itoa(kDefaultExpectSleepMS),
		"ExpectPassSleep": "no", "CtrlExpectPassSleep": "no",
	}
}

// These directives use the extension file as well as the main configuration.
func promptExtendedKey(key string) bool {
	switch strings.ToLower(key) {
	case "grouplabels", "localforward", "remoteforward", "dynamicforward",
		"enabletrzsz", "enablezmodem", "enableosc52", "enablewaypipe", "enabledragfile", "dragfileuploadcommand",
		"setterminaltitle", "consoleescapetime",
		"dnssrvname", "defaultuploadpath", "defaultdownloadpath", "progresscolorpair",
		"waypipeclientpath", "waypipeserverpath", "waypipeclientoption", "waypipeserveroption":
		return true
	}
	lower := strings.ToLower(key)
	return strings.HasPrefix(lower, "expect") || strings.HasPrefix(lower, "ctrlexpect") || promptSecretKey(key)
}

func promptSecretKey(key string) bool {
	key = strings.ToLower(key)
	if strings.HasPrefix(key, "enc") || strings.HasPrefix(key, "questionanswer") || strings.HasPrefix(key, "totpsecret") ||
		strings.Contains(key, "passphrase") || key == "password" || key == "passwordcommand" {
		return true
	}
	if strings.Contains(key, "expectsend") || strings.Contains(key, "expectcasesend") {
		return true
	}
	// Answers can also use a hex-encoded question as their configuration key.
	for _, prefix := range []string{"", "totp", "otp"} {
		candidate := strings.TrimPrefix(key, prefix)
		if len(candidate) > 0 {
			if _, err := hex.DecodeString(candidate); err == nil {
				return true
			}
		}
	}
	return false
}

func promptMultiKey(key string) bool {
	switch strings.ToLower(key) {
	case "identityfile", "certificatefile", "sendenv", "localforward", "remoteforward", "dynamicforward", "grouplabels":
		return true
	}
	return strings.Contains(strings.ToLower(key), "expectcasesendtext")
}

func collectPromptConfigKeys(config *ssh_config.Config, alias string, keys map[string]string) {
	if config == nil {
		return
	}
	for _, host := range config.Hosts {
		if !host.Matches(alias) {
			continue
		}
		for _, node := range host.Nodes {
			switch node := node.(type) {
			case *ssh_config.KV:
				keys[strings.ToLower(node.Key)] = node.Key
			case *ssh_config.Include:
				for _, included := range node.GetFiles() {
					collectPromptConfigKeys(included, alias, keys)
				}
			}
		}
	}
}

func promptConfiguredValues(config *ssh_config.Config, alias, key string) []string {
	if config == nil {
		return nil
	}
	if promptMultiKey(key) {
		values, _ := config.GetAll(alias, key)
		return values
	}
	value, _ := config.Get(alias, key)
	if value == "" {
		return nil
	}
	return []string{value}
}

func getPromptConfigEntries(host *sshHost, config *tsshConfig) []promptConfigEntry {
	if host == nil {
		return nil
	}
	defaults := ssh_config.Defaults()
	keys := make(map[string]string, len(defaults))
	for key := range defaults {
		keys[key] = key
	}
	for key, value := range promptRuntimeDefaults(host.Alias) {
		lower := strings.ToLower(key)
		defaults[lower], keys[lower] = value, key
	}
	// A synthetic host (e.g. in a preview) can supply the basic fields directly.
	basic := map[string]string{"HostName": host.Host, "User": host.User, "Port": host.Port, "GroupLabels": host.GroupLabels,
		"IdentityFile": host.IdentityFile, "ProxyCommand": host.ProxyCommand, "ProxyJump": host.ProxyJump, "RemoteCommand": host.RemoteCommand}
	for key := range basic {
		keys[strings.ToLower(key)] = key
	}
	if config != nil {
		config.doLoadConfig()
		config.doLoadExConfig()
		collectPromptConfigKeys(config.config, host.Alias, keys)
		collectPromptConfigKeys(config.exConfig, host.Alias, keys)
	}
	order := []string{"hostname", "user", "port", "grouplabels", "identityfile", "proxycommand", "proxyjump", "remotecommand"}
	var rest []string
	for key := range keys {
		if !slices.Contains(order, key) {
			rest = append(rest, key)
		}
	}
	slices.Sort(rest)
	order = append(order, rest...)
	entries := []promptConfigEntry{{"主机别名", host.Alias, ""}}
	for _, lower := range order {
		key := keys[lower]
		var values []string
		source := "默认"
		if config != nil {
			values = promptConfiguredValues(config.config, host.Alias, key)
			if len(values) > 0 {
				source = "配置"
			}
			_, known := defaults[lower]
			if promptExtendedKey(key) || !known {
				extra := promptConfiguredValues(config.exConfig, host.Alias, key)
				if len(extra) > 0 {
					if promptMultiKey(key) {
						values = append(extra, values...)
					} else {
						values = extra
					}
					source = "扩展配置"
				}
			}
		} else {
			for basicKey, value := range basic {
				if strings.EqualFold(basicKey, key) && value != "" {
					values = []string{value}
					source = "配置"
				}
			}
		}
		if len(values) == 0 {
			values = []string{defaults[lower]}
		}
		value := strings.Join(values, "\n")
		if value == "" {
			value = "未设置"
		} else if promptSecretKey(key) {
			value = "••••••（已设置）"
		}
		entries = append(entries, promptConfigEntry{key, value, source})
	}
	return entries
}
