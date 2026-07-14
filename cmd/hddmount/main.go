// Command hddmount is a small TUI for safely mounting HDDs on Ubuntu.
package main

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/koolsign/hddmount-tui/internal/ui"
)

func main() {
	if os.Geteuid() != 0 {
		fmt.Fprintln(os.Stderr, "이 프로그램은 root 권한이 필요합니다. sudo hddmount 로 실행해주세요.")
		os.Exit(1)
	}

	p := tea.NewProgram(ui.New(), tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Fprintln(os.Stderr, "실행 오류:", err)
		os.Exit(1)
	}
}
