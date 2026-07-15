package ui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

func (m *Model) updateTerminalScreen(msg tea.Msg) (tea.Model, tea.Cmd) {
	keyMsg, ok := msg.(tea.KeyMsg)
	if !ok {
		return m, nil
	}
	if m.screen == scrSmartDetail {
		switch keyMsg.String() {
		case "b", "esc", "enter":
			m.screen = scrDiskList
			return m, nil
		case "q", "ctrl+c":
			m.quitting = true
			return m, tea.Quit
		}
		return m, nil
	}
	switch keyMsg.String() {
	case "q", "esc", "ctrl+c":
		m.quitting = true
		return m, tea.Quit
	case "r":
		if m.screen == scrSummary {
			*m = *New()
			return m, m.Init()
		}
	}
	return m, nil
}

func (m *Model) viewRunning() string {
	var b strings.Builder
	b.WriteString(titleStyle.Render("작업 진행 중") + "\n\n")
	b.WriteString(logStyle.Render(joinLines(m.runLog)))
	if len(m.runLog) > 0 {
		b.WriteString("\n")
	}
	remaining := "완료 대기 중..."
	if len(m.pendingSteps) > 0 {
		remaining = m.pendingSteps[0].label + " 진행 중..."
	}
	b.WriteString(m.spin.View() + " " + remaining)
	return boxStyle.Render(b.String())
}

func (m *Model) viewRunError() string {
	var b strings.Builder
	b.WriteString(titleStyle.Render("작업 실패") + "\n\n")
	b.WriteString(logStyle.Render(joinLines(m.runLog)))
	b.WriteString("\n\n" + helpStyle.Render("q: 종료"))
	return boxStyle.Render(b.String())
}

func (m *Model) viewSummary() string {
	var b strings.Builder
	b.WriteString(okStyle.Render("완료되었습니다!") + "\n\n")
	b.WriteString("파티션: " + m.selectedPartPath + "\n")
	b.WriteString("마운트 포인트: " + m.mountPoint + "\n")
	if m.fstabChoice {
		b.WriteString("fstab: 등록됨 (재부팅 시 자동 마운트)\n")
	} else {
		b.WriteString("fstab: 등록하지 않음\n")
	}
	if m.dfOutput != "" {
		b.WriteString("\n" + logStyle.Render(m.dfOutput) + "\n")
	}
	b.WriteString("\n" + helpStyle.Render("r: 다른 디스크 마운트   q: 종료"))
	return boxStyle.Render(b.String())
}
