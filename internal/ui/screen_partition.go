package ui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

// totalPartitionItems is len(m.parts) existing partitions plus one virtual
// "format the whole disk" entry that always appears last.
func (m *Model) totalPartitionItems() int {
	return len(m.parts) + 1
}

func (m *Model) updatePartitionChoice(msg tea.Msg) (tea.Model, tea.Cmd) {
	keyMsg, ok := msg.(tea.KeyMsg)
	if !ok {
		return m, nil
	}
	total := m.totalPartitionItems()
	switch keyMsg.String() {
	case "up", "k":
		if m.partCur > 0 {
			m.partCur--
		}
	case "down", "j":
		if m.partCur < total-1 {
			m.partCur++
		}
	case "b", "esc":
		m.screen = scrDiskList
		return m, nil
	case "q":
		m.quitting = true
		return m, tea.Quit
	case "u":
		m.partErr = ""
		if m.partCur < len(m.parts) && m.parts[m.partCur].IsMounted() {
			p := m.parts[m.partCur]
			m.unmountTargetPath = p.DevPath()
			m.yesNoCursor = 1
			m.screen = scrUnmountConfirm
			return m, nil
		}
		m.partErr = "마운트 해제할 수 있는 파티션이 아닙니다 (선택한 파티션이 마운트되어 있지 않습니다)."
		return m, nil
	case "enter":
		m.partErr = ""
		if m.partCur == len(m.parts) {
			// 전체 디스크 포맷 선택
			m.yesNoCursor = 1 // 기본값은 "아니오" 쪽에 두어 실수로 enter 연타해도 안전
			m.confirmMsg = ""
			m.screen = scrFormatAllConfirm
			return m, nil
		}

		chosen := m.parts[m.partCur]
		if chosen.IsMounted() {
			m.partErr = fmt.Sprintf("이 파티션은 이미 %s 에 마운트되어 있습니다.", chosen.MountPoint)
			return m, nil
		}
		m.selectedPartPath = chosen.DevPath()
		if !chosen.HasFS() {
			m.yesNoCursor = 1
			m.screen = scrNoFSConfirm
			return m, nil
		}
		m.mountPoint = ""
		m.prepareMountTree = false
		resetTextInput(m, "/data/hdd_"+strings.ToLower(m.selectedDisk.Label))
		m.screen = scrMountPoint
		return m, nil
	}
	return m, nil
}

func (m *Model) viewPartitionChoice() string {
	var b strings.Builder
	b.WriteString(titleStyle.Render("파티션 선택") + "\n\n")
	b.WriteString(fmt.Sprintf("디스크: %s (%s, %s)\n\n", m.selectedDisk.DevPath(), m.selectedDisk.Label, m.selectedDisk.Model))

	if len(m.parts) == 0 {
		b.WriteString(subtleStyle.Render("이 디스크에는 파티션이 없습니다.") + "\n\n")
	}

	for i, p := range m.parts {
		fs := p.FSType
		if fs == "" {
			fs = "없음"
		}
		mp := p.MountPoint
		if mp == "" {
			mp = "없음"
		}
		line := fmt.Sprintf("%-12s  FSTYPE=%-8s  MOUNTPOINT=%s", p.DevPath(), fs, mp)
		if i == m.partCur {
			b.WriteString(cursorStyle.Render("› " + line))
		} else {
			b.WriteString("  " + line)
		}
		b.WriteString("\n")
	}

	formatLine := "⚠ 디스크 전체를 새로 파티션/포맷 (기존 데이터 전부 삭제)"
	if m.partCur == len(m.parts) {
		b.WriteString(cursorStyle.Render("› " + formatLine))
	} else {
		b.WriteString(warnStyle.Render("  " + formatLine))
	}
	b.WriteString("\n")

	if m.partErr != "" {
		b.WriteString("\n" + errorStyle.Render(m.partErr) + "\n")
	}

	b.WriteString("\n" + helpStyle.Render("↑/↓: 이동   enter: 선택   u: 마운트 해제   b: 뒤로   q: 종료"))
	return boxStyle.Render(b.String())
}
