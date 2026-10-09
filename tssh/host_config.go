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
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"unicode"

	"github.com/trzsz/ssh_config"
)

// Edits operate on source lines so unrelated directives, comments, unsupported
// Match blocks, and whitespace survive without a parser round trip.
type hostConfigBlock struct {
	start, end int
	aliases    []string
	values     map[string][]string
}

type hostConfigDocument struct {
	path     string
	original []byte
	lines    []string
	newline  string
	blocks   []hostConfigBlock
	parsed   *ssh_config.Config
	comments map[int]string
}

func configLineParts(line string) (string, string) {
	line = strings.TrimSpace(line)
	end := strings.IndexFunc(line, func(r rune) bool { return unicode.IsSpace(r) || r == '=' })
	if end < 0 {
		return strings.ToLower(line), ""
	}
	return strings.ToLower(line[:end]), strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(line[end:]), "="))
}

func readHostConfigDocument(path string, allowMissing bool) (*hostConfigDocument, error) {
	path, err := filepath.Abs(path)
	if err != nil {
		return nil, err
	}
	if resolved, err := filepath.EvalSymlinks(path); err == nil {
		path = resolved
	}
	data, err := os.ReadFile(path)
	if err != nil && !(allowMissing && os.IsNotExist(err)) {
		return nil, err
	}
	if err != nil {
		// A dangling link is not a missing configuration file we can safely create.
		if _, linkErr := os.Lstat(path); linkErr == nil {
			return nil, fmt.Errorf("配置路径不可读取：%s", path)
		}
	}
	parsed, err := ssh_config.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("解析配置失败：%w", err)
	}
	newline := "\n"
	if bytes.Contains(data, []byte("\r\n")) {
		newline = "\r\n"
	}
	lines := strings.Split(strings.ReplaceAll(string(data), "\r\n", "\n"), "\n")
	d := &hostConfigDocument{path: path, original: data, lines: lines, newline: newline, parsed: parsed, comments: map[int]string{}}
	for i, line := range lines {
		key, value := configLineParts(line)
		if key != "host" && key != "match" {
			continue
		}
		if len(d.blocks) > 0 {
			d.blocks[len(d.blocks)-1].end = i
		}
		block := hostConfigBlock{start: i, end: len(lines), values: map[string][]string{}}
		if key == "host" {
			value, _, _ = strings.Cut(value, "#")
			block.aliases = strings.Fields(value)
		}
		d.blocks = append(d.blocks, block)
	}
	for _, host := range parsed.Hosts {
		for _, node := range host.Nodes {
			kv, ok := node.(*ssh_config.KV)
			if !ok {
				continue
			}
			line := kv.Pos().Line - 1
			if kv.Comment != "" {
				d.comments[line] = kv.Comment
			}
			for i := range d.blocks {
				block := &d.blocks[i]
				if line > block.start && line < block.end {
					key := strings.ToLower(kv.Key)
					block.values[key] = append(block.values[key], kv.Value)
					break
				}
			}
		}
	}
	return d, nil
}

// Visit Includes in stable order, using the same resolved paths as the parser.
func hostConfigDocuments(path string) ([]*hostConfigDocument, error) {
	var documents []*hostConfigDocument
	seen := map[string]bool{}
	var visit func(string) error
	visit = func(path string) error {
		d, err := readHostConfigDocument(path, len(documents) == 0)
		if err != nil {
			return err
		}
		if seen[d.path] {
			return nil
		}
		seen[d.path] = true
		documents = append(documents, d)
		for _, host := range d.parsed.Hosts {
			for _, node := range host.Nodes {
				inc, ok := node.(*ssh_config.Include)
				if !ok {
					continue
				}
				paths := make([]string, 0, len(inc.GetFiles()))
				for path := range inc.GetFiles() {
					paths = append(paths, path)
				}
				slices.Sort(paths)
				for _, path := range paths {
					if err := visit(path); err != nil {
						return err
					}
				}
			}
		}
		return nil
	}
	if err := visit(path); err != nil {
		return nil, err
	}
	return documents, nil
}

func validateHostAlias(alias string) error {
	if alias == "" {
		return fmt.Errorf("请填写主机别名")
	}
	if strings.ContainsAny(alias, "*?!#=\"'[]\\") || strings.IndexFunc(alias, func(r rune) bool {
		return unicode.IsSpace(r) || unicode.IsControl(r)
	}) >= 0 {
		return fmt.Errorf("别名不能包含空格、通配符或配置分隔符")
	}
	return nil
}

func validateHostField(key, value string) error {
	if value == "" {
		return nil
	}
	if strings.IndexFunc(value, func(r rune) bool { return unicode.IsControl(r) && r != '\n' && r != '\t' }) >= 0 {
		return fmt.Errorf("%s 包含控制字符", key)
	}
	switch key {
	case "Port":
		n, err := strconv.Atoi(value)
		if err != nil || n < 1 || n > 65535 {
			return fmt.Errorf("端口必须为 1–65535")
		}
	case "ConnectTimeout", "ServerAliveInterval", "ServerAliveCountMax":
		n, err := strconv.Atoi(value)
		if err != nil || n < 0 {
			return fmt.Errorf("%s 必须为非负整数", key)
		}
	case "ForwardAgent", "IdentitiesOnly", "PasswordAuthentication", "PubkeyAuthentication", "Compression",
		"ExitOnForwardFailure", "EnableTrzsz", "EnableZmodem", "EnableOSC52":
		if !strings.EqualFold(value, "yes") && !strings.EqualFold(value, "no") {
			return fmt.Errorf("%s 请填写 yes 或 no", key)
		}
	case "StrictHostKeyChecking":
		if !slices.Contains([]string{"yes", "no", "ask", "accept-new", "off"}, strings.ToLower(value)) {
			return fmt.Errorf("主机密钥检查可选 yes / ask / accept-new / no")
		}
	case "RequestTTY":
		if !slices.Contains([]string{"yes", "no", "auto", "force"}, strings.ToLower(value)) {
			return fmt.Errorf("终端分配可选 auto / yes / no / force")
		}
	}
	return nil
}

func (d *hostConfigDocument) write(content []byte) error {
	current, err := os.ReadFile(d.path)
	if err != nil && !(os.IsNotExist(err) && d.original == nil) {
		return err
	}
	if (err == nil && d.original == nil) || !bytes.Equal(current, d.original) {
		return fmt.Errorf("配置文件已被其他程序修改，请取消并重新打开表单")
	}
	mode := os.FileMode(0600)
	if info, err := os.Stat(d.path); err == nil {
		if !info.Mode().IsRegular() {
			return fmt.Errorf("配置路径不是普通文件")
		}
		mode = info.Mode().Perm() & 0600
	}
	if err := os.MkdirAll(filepath.Dir(d.path), 0700); err != nil {
		return err
	}
	file, err := os.CreateTemp(filepath.Dir(d.path), ".tssh-config-*")
	if err != nil {
		return err
	}
	defer os.Remove(file.Name())
	defer file.Close()
	if err := file.Chmod(mode); err != nil {
		return err
	}
	if _, err := file.Write(content); err != nil {
		return err
	}
	if err := file.Sync(); err != nil {
		return err
	}
	if err := file.Close(); err != nil {
		return err
	}
	// Recheck immediately before replacing the file, including external creation.
	current, err = os.ReadFile(d.path)
	if err != nil && !(os.IsNotExist(err) && d.original == nil) {
		return err
	}
	if (err == nil && d.original == nil) || !bytes.Equal(current, d.original) {
		return fmt.Errorf("配置文件已被其他程序修改，请取消并重新打开表单")
	}
	if err := os.Rename(file.Name(), d.path); err != nil {
		return err
	}
	return nil
}
