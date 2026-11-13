package tssh

import (
	"encoding/json"
	"os"
	"slices"
)

// Keep display order separate from SSH directives: moving Host blocks could
// change precedence, including when hosts come from different Include files.
func (c *tsshConfig) hostOrderPath() string { return c.configPath + ".order.json" }

func (c *tsshConfig) applyHostOrder() {
	if c.configPath == "" {
		return
	}
	data, err := os.ReadFile(c.hostOrderPath())
	if os.IsNotExist(err) {
		return
	}
	var aliases []string
	if err == nil {
		err = json.Unmarshal(data, &aliases)
	}
	if err != nil {
		warning("读取主机顺序失败：%v", err)
		return
	}
	byAlias := make(map[string][]*sshHost)
	for _, host := range c.allHosts {
		byAlias[host.Alias] = append(byAlias[host.Alias], host)
	}
	ordered := make([]*sshHost, 0, len(c.allHosts))
	used := make(map[*sshHost]bool)
	for _, alias := range aliases {
		if hosts := byAlias[alias]; len(hosts) > 0 {
			ordered = append(ordered, hosts[0])
			used[hosts[0]] = true
			byAlias[alias] = hosts[1:]
		}
	}
	// Ignore removed aliases and append new hosts in configuration order.
	for _, host := range c.allHosts {
		if !used[host] {
			ordered = append(ordered, host)
		}
	}
	c.allHosts = ordered
}

func (p *sshPrompt) reorderHost(direction int) {
	target := p.cursor + direction
	if p.currentHost() == nil || target < 0 || target >= len(p.visible) {
		return
	}
	if p.config == nil || p.config.configPath == "" {
		p.notice = "没有可保存主机顺序的配置路径"
		return
	}
	host, neighbor := p.currentHost(), p.visible[target]
	from, to := slices.Index(p.hosts, host), slices.Index(p.hosts, neighbor)
	if from < 0 || to < 0 {
		return
	}
	ordered := slices.Clone(p.hosts)
	// With a filter active, only exchange the two visible hosts. Hidden hosts
	// keep their positions in the full list.
	ordered[from], ordered[to] = ordered[to], ordered[from]
	aliases := make([]string, len(ordered))
	for i, h := range ordered {
		aliases[i] = h.Alias
	}
	data, err := json.MarshalIndent(aliases, "", "  ")
	if err == nil {
		var original []byte
		original, err = os.ReadFile(p.config.hostOrderPath())
		if os.IsNotExist(err) {
			err = nil
		}
		if err == nil {
			err = writeConfigFile(p.config.hostOrderPath(), original, append(data, '\n'))
		}
	}
	if err != nil {
		p.notice = "保存主机顺序失败：" + err.Error()
		return
	}
	copy(p.hosts, ordered)
	p.visible[p.cursor], p.visible[target] = p.visible[target], p.visible[p.cursor]
	p.cursor = target
	p.notice = "已保存主机顺序：" + host.Alias
}
