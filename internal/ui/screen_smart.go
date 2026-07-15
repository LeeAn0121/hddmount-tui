package ui

import (
	"strings"
)

func (m *Model) viewSmartDetail() string {
	var b strings.Builder
	b.WriteString(titleStyle.Render("SMART 상태 - "+m.selectedDisk.DevPath()) + "\n\n")

	if m.smartErr != nil {
		b.WriteString(errorStyle.Render(m.smartErr.Error()) + "\n")
	} else {
		b.WriteString(logStyle.Render(m.smartOutput) + "\n")
	}

	b.WriteString("\n" + helpStyle.Render("b/esc: 뒤로   q: 종료"))
	return boxStyle.Render(b.String())
}
