package ui

import (
	"fmt"
	"path"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/koolsign/hddmount-tui/internal/diskutil"
)

// submitMountPoint validates the path the person typed and decides whether
// we can move straight to the fstab question or need an extra confirmation
// for something unusual (already a mount point, or a non-empty directory).
func (m *Model) submitMountPoint(value string) (tea.Model, tea.Cmd) {
	value = path.Clean(value)
	if value == "" {
		m.inputErr = "경로를 입력해주세요."
		return m, nil
	}
	if !strings.HasPrefix(value, "/") {
		m.inputErr = "절대경로로 입력해주세요 (/ 로 시작)."
		return m, nil
	}
	if diskutil.IsDangerousMountpoint(value) {
		m.inputErr = fmt.Sprintf("시스템 핵심 경로(%s)는 마운트 포인트로 사용할 수 없습니다. /data 같은 운영용 데이터 경로를 지정해주세요.", value)
		return m, nil
	}

	m.mountPoint = value

	if diskutil.IsMountpoint(value) {
		m.mountWarn = fmt.Sprintf("%s 는 이미 다른 무언가가 마운트되어 있습니다.", value)
		m.yesNoCursor = 1
		m.screen = scrMountPointWarnConfirm
		return m, nil
	}
	if diskutil.DirHasContent(value) {
		m.mountWarn = fmt.Sprintf("%s 디렉토리가 이미 존재하고 내부에 파일이 있습니다.\n"+
			"마운트하면 기존 파일은 마운트 해제 전까지 가려지며(숨겨짐) 삭제되지는 않습니다.", value)
		m.yesNoCursor = 1
		m.screen = scrMountPointWarnConfirm
		return m, nil
	}

	m.yesNoCursor = 0
	m.screen = scrFstabConfirm
	return m, nil
}

func (m *Model) viewMountPoint() string {
	prompt := "마운트 포인트 경로를 입력하세요 (예: /data 또는 /data/hdd_1tb)\n" +
		subtleStyle.Render(fmt.Sprintf("대상 파티션: %s", m.selectedPartPath)) + "\n" +
		subtleStyle.Render("/ 자체와 /etc, /boot 등 시스템 핵심 경로는 사용할 수 없습니다. /data 같은 운영용 최상위 경로는 사용할 수 있습니다.")
	return m.viewTextInput(prompt)
}
