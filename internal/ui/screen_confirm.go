package ui

import (
	"path/filepath"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/koolsign/hddmount-tui/internal/diskutil"
)

// ---- generic yes/no confirm screens ----
// Used by: scrFormatAllConfirm, scrNoFSConfirm, scrMountPointWarnConfirm, scrFstabConfirm

func (m *Model) updateYesNo(msg tea.Msg) (tea.Model, tea.Cmd) {
	keyMsg, ok := msg.(tea.KeyMsg)
	if !ok {
		return m, nil
	}
	switch keyMsg.String() {
	case "left", "h", "tab":
		m.yesNoCursor = 0
	case "right", "l":
		m.yesNoCursor = 1
	case "y":
		m.yesNoCursor = 0
		return m.confirmYesNo(true)
	case "n":
		m.yesNoCursor = 1
		return m.confirmYesNo(false)
	case "enter":
		return m.confirmYesNo(m.yesNoCursor == 0)
	case "esc":
		return m.confirmYesNo(false)
	}
	return m, nil
}

// confirmYesNo dispatches to the correct next step depending on which
// yes/no screen we were on when the user confirmed.
func (m *Model) confirmYesNo(yes bool) (tea.Model, tea.Cmd) {
	switch m.screen {
	case scrFormatAllConfirm:
		if !yes {
			m.screen = scrPartitionChoice
			return m, nil
		}
		m.expectedText = m.selectedDisk.DevPath()
		resetTextInput(m, m.expectedText)
		m.screen = scrFormatAllDeviceType
		return m, nil

	case scrNoFSConfirm:
		if !yes {
			m.screen = scrPartitionChoice
			return m, nil
		}
		m.expectedText = m.selectedPartPath
		resetTextInput(m, m.expectedText)
		m.screen = scrNoFSDeviceType
		return m, nil

	case scrMountPointWarnConfirm:
		if !yes {
			m.screen = scrMountPoint
			resetTextInput(m, m.mountPoint)
			return m, nil
		}
		m.yesNoCursor = 0
		m.screen = scrFstabConfirm
		return m, nil

	case scrUnmountConfirm:
		if !yes {
			m.screen = scrPartitionChoice
			return m, nil
		}
		target := m.unmountTargetPath
		steps := []step{
			{label: "마운트 해제", target: target, run: func() (string, error) {
				return diskutil.Unmount(target)
			}},
		}
		return m.startRun(steps, scrLoadingPartitions)

	case scrFstabConfirm:
		m.fstabChoice = yes
		label := diskutil.SanitizeLabel(filepath.Base(m.mountPoint))
		steps := []step{
			{label: "라벨 설정", target: m.selectedPartPath, run: func() (string, error) {
				// Best-effort: not every filesystem/driver supports e2label,
				// so a failure here shouldn't abort the mount.
				out, err := diskutil.SetLabel(m.selectedPartPath, label)
				if err != nil {
					return "라벨 설정 건너뜀: " + err.Error(), nil
				}
				return out, nil
			}},
			{label: "마운트", target: m.selectedPartPath, run: func() (string, error) {
				return diskutil.MountPartition(m.selectedPartPath, m.mountPoint)
			}},
			{label: "표준 폴더/권한 설정", target: m.mountPoint, run: func() (string, error) {
				return diskutil.PrepareContentTree(m.mountPoint)
			}},
		}
		if m.fstabChoice {
			steps = append(steps, step{label: "fstab 등록", target: m.mountPoint, run: func() (string, error) {
				return diskutil.SetupFstab(m.selectedPartPath, m.mountPoint)
			}})
		}
		return m.startRun(steps, scrSummary)
	}
	return m, nil
}

func (m *Model) viewYesNo(prompt string) string {
	var b strings.Builder
	b.WriteString(titleStyle.Render("확인") + "\n\n")
	b.WriteString(prompt + "\n\n")

	yes, no := "  예  ", "  아니오  "
	if m.yesNoCursor == 0 {
		b.WriteString(selectedStyle.Render("[" + yes + "]"))
		b.WriteString("   " + no)
	} else {
		b.WriteString("  " + yes)
		b.WriteString("   " + selectedStyle.Render("["+no+"]"))
	}

	b.WriteString("\n\n" + helpStyle.Render("←/→: 이동   enter: 확정   y/n: 단축키"))
	return boxStyle.Render(b.String())
}

// ---- generic "type to confirm" text-input screens ----
// Used by: scrFormatAllDeviceType, scrFormatAllFinalYes, scrNoFSDeviceType, scrMountPoint

func (m *Model) updateTextInput(msg tea.Msg) (tea.Model, tea.Cmd) {
	keyMsg, ok := msg.(tea.KeyMsg)
	if ok {
		switch keyMsg.String() {
		case "esc":
			return m.cancelTextInput()
		case "enter":
			return m.submitTextInput()
		}
	}
	var cmd tea.Cmd
	m.textInput, cmd = m.textInput.Update(msg)
	return m, cmd
}

func (m *Model) cancelTextInput() (tea.Model, tea.Cmd) {
	switch m.screen {
	case scrFormatAllDeviceType, scrFormatAllFinalYes:
		m.screen = scrPartitionChoice
	case scrNoFSDeviceType:
		m.screen = scrPartitionChoice
	case scrMountPoint:
		m.screen = scrPartitionChoice
	case scrRaidConfirmType:
		m.screen = scrRaidLevelChoice
	}
	return m, nil
}

func (m *Model) submitTextInput() (tea.Model, tea.Cmd) {
	value := strings.TrimSpace(m.textInput.Value())

	switch m.screen {
	case scrFormatAllDeviceType:
		if value != m.expectedText {
			m.inputErr = "입력이 장치 경로와 일치하지 않습니다. 다시 입력해주세요."
			return m, nil
		}
		resetTextInput(m, "yes")
		m.screen = scrFormatAllFinalYes
		return m, nil

	case scrFormatAllFinalYes:
		if value != "yes" {
			m.inputErr = `취소하려면 esc, 진행하려면 정확히 "yes" 를 입력하세요.`
			return m, nil
		}
		dev := m.selectedDisk.DevPath()
		steps := []step{
			{label: "기존 서명 제거", target: dev, run: func() (string, error) { return diskutil.WipeSignatures(dev) }},
			{label: "GPT 파티션 생성", target: dev, run: func() (string, error) { return diskutil.CreatePartitionTable(dev) }},
			{label: "ext4 포맷", target: dev, run: func() (string, error) {
				m.selectedPartPath = diskutil.NewPartitionPath(dev)
				return diskutil.FormatExt4(m.selectedPartPath)
			}},
		}
		return m.startRun(steps, scrMountPoint)

	case scrNoFSDeviceType:
		if value != m.expectedText {
			m.inputErr = "입력이 파티션 경로와 일치하지 않습니다. 다시 입력해주세요."
			return m, nil
		}
		part := m.selectedPartPath
		steps := []step{
			{label: "ext4 포맷", target: part, run: func() (string, error) { return diskutil.FormatExt4(part) }},
		}
		return m.startRun(steps, scrMountPoint)

	case scrMountPoint:
		return m.submitMountPoint(value)

	case scrRaidConfirmType:
		return m.submitRaidConfirm(value)
	}
	return m, nil
}

func (m *Model) viewTextInput(prompt string) string {
	var b strings.Builder
	b.WriteString(titleStyle.Render("확인") + "\n\n")
	b.WriteString(prompt + "\n\n")
	b.WriteString(m.textInput.View())
	if m.inputErr != "" {
		b.WriteString("\n\n" + errorStyle.Render(m.inputErr))
	}
	b.WriteString("\n\n" + helpStyle.Render("enter: 확인   esc: 취소"))
	return boxStyle.Render(b.String())
}
