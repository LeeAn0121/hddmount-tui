package ui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

func (m *Model) updateDiskList(msg tea.Msg) (tea.Model, tea.Cmd) {
	keyMsg, ok := msg.(tea.KeyMsg)
	if !ok {
		return m, nil
	}
	switch keyMsg.String() {
	case "up", "k":
		if m.diskCur > 0 {
			m.diskCur--
		}
	case "down", "j":
		if m.diskCur < len(m.disks)-1 {
			m.diskCur++
		}
	case "enter":
		m.selectedDisk = m.disks[m.diskCur]
		m.screen = scrLoadingPartitions
		return m, loadPartitionsCmd(m.selectedDisk.Name)
	case "s":
		m.selectedDisk = m.disks[m.diskCur]
		m.screen = scrLoadingSmart
		return m, tea.Batch(m.spin.Tick, loadSmartCmd(m.selectedDisk.DevPath()))
	case "q", "esc":
		m.quitting = true
		return m, tea.Quit
	}
	return m, nil
}

func (m *Model) updateNoDisks(msg tea.Msg) (tea.Model, tea.Cmd) {
	if keyMsg, ok := msg.(tea.KeyMsg); ok {
		if keyMsg.String() == "q" || keyMsg.String() == "esc" {
			m.quitting = true
			return m, tea.Quit
		}
	}
	return m, nil
}

func (m *Model) viewDiskList() string {
	var b strings.Builder
	b.WriteString(titleStyle.Render("HDD 마운트 도구") + "\n\n")
	b.WriteString("감지된 디스크 목록 (시스템 루트 디스크는 제외됨)\n\n")

	for i, d := range m.disks {
		line := fmt.Sprintf("%-10s  %-6s  %-30s  SMART=%s", d.DevPath(), d.Label, d.Model, d.SmartState)
		if d.SmartState == "위험(FAILED)" {
			line = errorStyle.Render(line)
		}
		if i == m.diskCur {
			b.WriteString(cursorStyle.Render("› " + line))
		} else {
			b.WriteString("  " + line)
		}
		b.WriteString("\n")
	}

	b.WriteString("\n" + helpStyle.Render("↑/↓: 이동   enter: 선택   s: SMART 상세   q: 종료"))
	return boxStyle.Render(b.String())
}
