package ui

import (
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/koolsign/hddmount-tui/internal/diskutil"
)

// startRaidSetup is entered from the disk list ("R"). It groups every disk
// that shares the currently-hovered disk's size label, since mdadm RAID1/10
// expects same-capacity members.
func (m *Model) startRaidSetup() (tea.Model, tea.Cmd) {
	m.diskErr = ""
	if len(m.disks) == 0 {
		return m, nil
	}
	label := m.disks[m.diskCur].Label
	var candidates []diskutil.Disk
	for _, d := range m.disks {
		if d.Label == label {
			candidates = append(candidates, d)
		}
	}
	if len(candidates) < 2 {
		m.diskErr = fmt.Sprintf("RAID 구성에는 동일 용량(%s) 디스크가 최소 2개 필요합니다 (현재 %d개).", label, len(candidates))
		return m, nil
	}
	m.raidCandidates = candidates
	m.raidSelected = map[int]bool{}
	m.raidCur = 0
	m.screen = scrRaidDiskSelect
	return m, nil
}

func (m *Model) raidSelectedCount() int {
	n := 0
	for _, v := range m.raidSelected {
		if v {
			n++
		}
	}
	return n
}

func (m *Model) updateRaidDiskSelect(msg tea.Msg) (tea.Model, tea.Cmd) {
	keyMsg, ok := msg.(tea.KeyMsg)
	if !ok {
		return m, nil
	}
	switch keyMsg.String() {
	case "up", "k":
		if m.raidCur > 0 {
			m.raidCur--
		}
	case "down", "j":
		if m.raidCur < len(m.raidCandidates)-1 {
			m.raidCur++
		}
	case " ":
		m.raidSelected[m.raidCur] = !m.raidSelected[m.raidCur]
	case "a":
		if m.raidSelectedCount() == len(m.raidCandidates) {
			m.raidSelected = map[int]bool{}
		} else {
			for i := range m.raidCandidates {
				m.raidSelected[i] = true
			}
		}
	case "b", "esc":
		m.screen = scrDiskList
		return m, nil
	case "q":
		m.quitting = true
		return m, tea.Quit
	case "enter":
		count := m.raidSelectedCount()
		if count < 2 {
			m.diskErr = "RAID 구성을 위해 최소 2개의 디스크를 선택하세요 (space: 선택/해제)."
			return m, nil
		}
		var devs []string
		for i, d := range m.raidCandidates {
			if m.raidSelected[i] {
				devs = append(devs, d.DevPath())
			}
		}
		m.raidDevs = devs
		m.raidLevels = []int{1}
		if count >= 4 && count%2 == 0 {
			m.raidLevels = append(m.raidLevels, 10)
		}
		m.raidLevelCur = 0
		m.diskErr = ""
		m.screen = scrRaidLevelChoice
		return m, nil
	}
	return m, nil
}

func (m *Model) viewRaidDiskSelect() string {
	var b strings.Builder
	b.WriteString(titleStyle.Render("RAID 구성 - 디스크 선택") + "\n\n")
	fmt.Fprintf(&b, "동일 용량(%s) 디스크 %d개 감지됨. RAID에 포함할 디스크를 선택하세요.\n\n",
		m.raidCandidates[0].Label, len(m.raidCandidates))

	for i, d := range m.raidCandidates {
		box := "[ ]"
		if m.raidSelected[i] {
			box = "[x]"
		}
		line := fmt.Sprintf("%s %-10s  %-6s  %-30s", box, d.DevPath(), d.Label, d.Model)
		if i == m.raidCur {
			b.WriteString(cursorStyle.Render("› " + line))
		} else {
			b.WriteString("  " + line)
		}
		b.WriteString("\n")
	}

	if m.diskErr != "" {
		b.WriteString("\n" + errorStyle.Render(m.diskErr) + "\n")
	}

	b.WriteString("\n" + helpStyle.Render("↑/↓: 이동   space: 선택/해제   a: 전체선택   enter: 다음   b: 뒤로   q: 종료"))
	return boxStyle.Render(b.String())
}

func (m *Model) updateRaidLevelChoice(msg tea.Msg) (tea.Model, tea.Cmd) {
	keyMsg, ok := msg.(tea.KeyMsg)
	if !ok {
		return m, nil
	}
	switch keyMsg.String() {
	case "up", "k":
		if m.raidLevelCur > 0 {
			m.raidLevelCur--
		}
	case "down", "j":
		if m.raidLevelCur < len(m.raidLevels)-1 {
			m.raidLevelCur++
		}
	case "b", "esc":
		m.screen = scrRaidDiskSelect
		return m, nil
	case "q":
		m.quitting = true
		return m, tea.Quit
	case "enter":
		m.raidLevel = m.raidLevels[m.raidLevelCur]
		names := make([]string, len(m.raidDevs))
		for i, d := range m.raidDevs {
			names[i] = strings.TrimPrefix(d, "/dev/")
		}
		m.expectedText = strings.Join(names, ",")
		resetTextInput(m, m.expectedText)
		m.screen = scrRaidConfirmType
		return m, nil
	}
	return m, nil
}

func raidLevelDesc(level int) string {
	switch level {
	case 1:
		return "RAID 1  (미러링, 용량 = 디스크 1개분, 디스크 1개 고장까지 허용)"
	case 10:
		return "RAID 10 (미러링+스트라이핑, 용량 = 전체의 절반, 속도/안정성 우수)"
	default:
		return fmt.Sprintf("RAID %d", level)
	}
}

func (m *Model) viewRaidLevelChoice() string {
	var b strings.Builder
	b.WriteString(titleStyle.Render("RAID 구성 - 레벨 선택") + "\n\n")
	fmt.Fprintf(&b, "선택된 디스크 %d개로 구성할 RAID 레벨을 선택하세요.\n\n", len(m.raidDevs))

	for i, level := range m.raidLevels {
		line := raidLevelDesc(level)
		if i == m.raidLevelCur {
			b.WriteString(cursorStyle.Render("› " + line))
		} else {
			b.WriteString("  " + line)
		}
		b.WriteString("\n")
	}

	b.WriteString("\n" + helpStyle.Render("↑/↓: 이동   enter: 선택   b: 뒤로   q: 종료"))
	return boxStyle.Render(b.String())
}

func (m *Model) viewRaidConfirmType() string {
	warn := errorStyle.Render("!! 경고 !!") + "\n" +
		warnStyle.Render(fmt.Sprintf(
			"%s 디스크 %d개(%s)를 RAID%d 배열로 구성합니다.\n"+
				"해당 디스크들의 모든 데이터가 영구적으로 삭제되며 복구할 수 없습니다.",
			m.raidCandidates[0].Label, len(m.raidDevs), strings.Join(m.raidDevs, ", "), m.raidLevel)) +
		"\n\n계속하려면 아래 장치 목록을 정확히 입력하세요: " + m.expectedText
	return m.viewTextInput(warn)
}

// submitRaidConfirm wipes every selected device, creates the mdadm array,
// persists it so the array survives a reboot, and formats it — then hands
// off to the ordinary mount-point flow, exactly like the whole-disk-format
// path does.
func (m *Model) submitRaidConfirm(value string) (tea.Model, tea.Cmd) {
	if value != m.expectedText {
		m.inputErr = "입력이 장치 목록과 일치하지 않습니다. 다시 입력해주세요."
		return m, nil
	}

	devs := append([]string(nil), m.raidDevs...)
	level := m.raidLevel
	m.prepareMountTree = true
	m.raidName = fmt.Sprintf("raid%d_%d", level, time.Now().Unix()%100000)
	name := m.raidName

	var steps []step
	for _, d := range devs {
		dev := d
		steps = append(steps, step{
			label:  "기존 서명 제거 (" + dev + ")",
			target: dev,
			run:    func() (string, error) { return diskutil.WipeSignatures(dev) },
		})
	}
	steps = append(steps,
		step{label: fmt.Sprintf("RAID%d 배열 생성", level), target: name, run: func() (string, error) {
			return diskutil.CreateRaidArray(level, name, devs)
		}},
		step{label: "mdadm.conf 갱신", target: name, run: func() (string, error) {
			return diskutil.PersistRaidConfig()
		}},
		step{label: "initramfs 갱신", target: name, run: func() (string, error) {
			return diskutil.UpdateInitramfs()
		}},
		step{label: "ext4 포맷", target: name, run: func() (string, error) {
			m.selectedPartPath = diskutil.RaidDevicePath(name)
			return diskutil.FormatExt4(m.selectedPartPath)
		}},
	)
	return m.startRun(steps, scrMountPoint)
}
