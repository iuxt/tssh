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

import "strings"

type promptGroup struct {
	label string
	hosts []*sshHost
}

type promptRow struct {
	host  *sshHost // nil denotes a group heading
	group string
	count int
}

func (p *sshPrompt) buildGroups() {
	p.groups = nil
	// Keep an entirely unlabelled configuration as a flat list.
	grouped := false
	for _, host := range p.hosts {
		if len(strings.Fields(host.GroupLabels)) > 0 {
			grouped = true
			break
		}
	}
	if !grouped {
		return
	}
	indices := make(map[string]int)
	for _, host := range p.visible {
		labels := strings.Fields(host.GroupLabels)
		if len(labels) == 0 {
			labels = []string{""}
		}
		seen := make(map[string]bool)
		for _, label := range labels {
			if seen[label] {
				continue
			}
			seen[label] = true
			index, ok := indices[label]
			if !ok {
				index = len(p.groups)
				indices[label] = index
				p.groups = append(p.groups, promptGroup{label: label})
			}
			p.groups[index].hosts = append(p.groups[index].hosts, host)
		}
	}
}

func (p *sshPrompt) filtering() bool {
	return p.search || strings.TrimSpace(p.keywords+" "+p.query) != ""
}

func (p *sshPrompt) groupCollapsed(label string) bool {
	return !p.filtering() && p.collapsed[label]
}

func (p *sshPrompt) rebuildRows() {
	p.rows = nil
	if len(p.groups) == 0 {
		for _, host := range p.visible {
			p.rows = append(p.rows, promptRow{host: host})
		}
		return
	}
	for _, group := range p.groups {
		p.rows = append(p.rows, promptRow{group: group.label, count: len(group.hosts)})
		if !p.groupCollapsed(group.label) {
			for _, host := range group.hosts {
				p.rows = append(p.rows, promptRow{host: host, group: group.label})
			}
		}
	}
}

func (p *sshPrompt) onGroupHeading() bool {
	return len(p.groups) > 0 && p.cursor >= 0 && p.cursor < len(p.rows) && p.rows[p.cursor].host == nil
}

func (p *sshPrompt) toggleGroup() {
	if p.cursor >= 0 && p.cursor < len(p.rows) {
		p.setGroupCollapsed(!p.groupCollapsed(p.rows[p.cursor].group))
	}
}

func (p *sshPrompt) setGroupCollapsed(collapsed bool) {
	if len(p.groups) == 0 || p.cursor < 0 || p.cursor >= len(p.rows) || p.filtering() {
		return
	}
	row := p.rows[p.cursor]
	if p.collapsed == nil {
		p.collapsed = make(map[string]bool)
	}
	p.collapsed[row.group] = collapsed
	p.rebuildRows()
	// Collapsing a selected host moves focus to its heading. Expanding keeps
	// focus stable; the next Down selects the first child.
	for i, candidate := range p.rows {
		if candidate.group == row.group && (candidate.host == nil || candidate.host == row.host) {
			p.cursor = i
			if collapsed || row.host == nil || candidate.host == row.host {
				break
			}
		}
	}
	p.detailOffset = 0
}

func (p *sshPrompt) selectAlias(alias string) {
	for _, group := range p.groups {
		for _, host := range group.hosts {
			if host.Alias == alias {
				delete(p.collapsed, group.label)
				break
			}
		}
	}
	p.rebuildRows()
	for i, row := range p.rows {
		if row.host != nil && row.host.Alias == alias {
			p.cursor, p.detailOffset = i, 0
			return
		}
	}
}

func promptGroupName(label string) string {
	if label == "" {
		return "未分组"
	}
	return label
}
