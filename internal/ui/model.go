// Package ui implements the Bubbletea-based terminal UI for hddmount-tui.
package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/koolsign/hddmount-tui/internal/diskutil"
)

type screen int

const (
	scrLoadingDisks screen = iota
	scrDiskList
	scrNoDisks
	scrLoadingPartitions
	scrPartitionChoice
	scrFormatAllConfirm
	scrFormatAllDeviceType
	scrFormatAllFinalYes
	scrNoFSConfirm
	scrNoFSDeviceType
	scrMountPoint
	scrMountPointWarnConfirm
	scrFstabConfirm
	scrUnmountConfirm
	scrRaidDiskSelect
	scrRaidLevelChoice
	scrRaidConfirmType
	scrRunning
	scrRunError
	scrSummary
	scrFatalError
	scrLoadingSmart
	scrSmartDetail
)

// step is one unit of work executed sequentially on the scrRunning screen.
type step struct {
	label  string
	target string // device/path this step acts on, recorded in the operations log
	run    func() (string, error)
}

// Model is the top-level Bubbletea model for the whole wizard.
type Model struct {
	screen screen

	width, height int

	// disk list
	disks   []diskutil.Disk
	diskCur int

	selectedDisk diskutil.Disk

	// partition choice (last entry is the virtual "format whole disk" option)
	parts   []diskutil.Partition
	partCur int
	partErr string

	selectedPartPath string // target /dev/... partition for this run

	// generic text input, reused by every "type to confirm" screen
	textInput    textinput.Model
	inputErr     string
	expectedText string

	// generic yes/no cursor, reused by every confirm screen
	yesNoCursor int // 0 = 예, 1 = 아니오
	confirmMsg  string

	// mount point
	mountPoint string
	mountWarn  string

	fstabChoice bool

	// unmount
	unmountTargetPath string

	// SMART detail view
	smartOutput string
	smartErr    error

	// RAID setup
	diskErr        string
	raidCandidates []diskutil.Disk
	raidSelected   map[int]bool
	raidCur        int
	raidLevels     []int
	raidLevelCur   int
	raidLevel      int
	raidDevs       []string // /dev/... device paths, selection order
	raidName       string

	// running screen
	spin         spinner.Model
	runLog       []string
	pendingSteps []step
	afterRun     screen

	fatalErr error
	dfOutput string

	quitting bool
}

// ---- messages ----

type disksLoadedMsg struct {
	disks []diskutil.Disk
	err   error
}

type partitionsLoadedMsg struct {
	parts []diskutil.Partition
	err   error
}

type stepResultMsg struct {
	log string
	err error
}

type smartLoadedMsg struct {
	output string
	err    error
}

// ---- constructors ----

func New() *Model {
	ti := textinput.New()
	ti.CharLimit = 200
	ti.Width = 50

	sp := spinner.New()
	sp.Spinner = spinner.Dot

	return &Model{
		screen:    scrLoadingDisks,
		textInput: ti,
		spin:      sp,
	}
}

func (m *Model) Init() tea.Cmd {
	return loadDisksCmd()
}

// ---- commands ----

func loadDisksCmd() tea.Cmd {
	return func() tea.Msg {
		disks, err := diskutil.ListDisks()
		return disksLoadedMsg{disks: disks, err: err}
	}
}

func loadPartitionsCmd(diskName string) tea.Cmd {
	return func() tea.Msg {
		parts, err := diskutil.GetPartitions(diskName)
		return partitionsLoadedMsg{parts: parts, err: err}
	}
}

func runStepCmd(s step) tea.Cmd {
	return func() tea.Msg {
		log, err := s.run()
		diskutil.LogEvent(s.label, s.target, log, err)
		return stepResultMsg{log: log, err: err}
	}
}

func loadSmartCmd(dev string) tea.Cmd {
	return func() tea.Msg {
		out, err := diskutil.SmartDetail(dev)
		return smartLoadedMsg{output: out, err: err}
	}
}

// ---- Update ----

func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		return m, nil

	case tea.KeyMsg:
		if msg.String() == "ctrl+c" {
			m.quitting = true
			return m, tea.Quit
		}

	case disksLoadedMsg:
		if msg.err != nil {
			m.fatalErr = msg.err
			m.screen = scrFatalError
			return m, nil
		}
		m.disks = msg.disks
		if len(m.disks) == 0 {
			m.screen = scrNoDisks
			return m, nil
		}
		m.screen = scrDiskList
		return m, nil

	case partitionsLoadedMsg:
		if msg.err != nil {
			m.fatalErr = msg.err
			m.screen = scrFatalError
			return m, nil
		}
		m.parts = msg.parts
		m.partCur = 0
		m.screen = scrPartitionChoice
		return m, nil

	case spinner.TickMsg:
		if m.screen == scrRunning || m.screen == scrLoadingSmart {
			var cmd tea.Cmd
			m.spin, cmd = m.spin.Update(msg)
			return m, cmd
		}
		return m, nil

	case stepResultMsg:
		return m.handleStepResult(msg)

	case smartLoadedMsg:
		m.smartOutput = msg.output
		m.smartErr = msg.err
		m.screen = scrSmartDetail
		return m, nil
	}

	switch m.screen {
	case scrDiskList:
		return m.updateDiskList(msg)
	case scrNoDisks:
		return m.updateNoDisks(msg)
	case scrPartitionChoice:
		return m.updatePartitionChoice(msg)
	case scrFormatAllConfirm, scrNoFSConfirm, scrMountPointWarnConfirm, scrFstabConfirm, scrUnmountConfirm:
		return m.updateYesNo(msg)
	case scrFormatAllDeviceType, scrFormatAllFinalYes, scrNoFSDeviceType, scrMountPoint, scrRaidConfirmType:
		return m.updateTextInput(msg)
	case scrRaidDiskSelect:
		return m.updateRaidDiskSelect(msg)
	case scrRaidLevelChoice:
		return m.updateRaidLevelChoice(msg)
	case scrRunError, scrSummary, scrFatalError, scrSmartDetail:
		return m.updateTerminalScreen(msg)
	}

	return m, nil
}

// ---- View ----

func (m *Model) View() string {
	if m.quitting {
		return ""
	}

	switch m.screen {
	case scrLoadingDisks:
		return boxStyle.Render(fmt.Sprintf("%s 디스크를 검색하는 중...", m.spin.View()))
	case scrDiskList:
		return m.viewDiskList()
	case scrNoDisks:
		return boxStyle.Render(warnStyle.Render("마운트 가능한 디스크가 없습니다 (시스템 루트 디스크 제외).") +
			"\n\n" + helpStyle.Render("q: 종료"))
	case scrLoadingPartitions:
		return boxStyle.Render(fmt.Sprintf("%s 파티션 정보를 불러오는 중...", m.spin.View()))
	case scrPartitionChoice:
		return m.viewPartitionChoice()
	case scrFormatAllConfirm:
		return m.viewYesNo(errorStyle.Render("!! 경고 !!") + "\n" +
			warnStyle.Render(fmt.Sprintf("%s 디스크 전체를 새 GPT 파티션 테이블로 초기화하고 포맷합니다.\n"+
				"해당 디스크의 모든 데이터가 영구적으로 삭제되며 복구할 수 없습니다.", m.selectedDisk.DevPath())))
	case scrFormatAllDeviceType:
		return m.viewTextInput(fmt.Sprintf("계속하려면 장치 경로를 정확히 입력하세요: %s", m.expectedText))
	case scrFormatAllFinalYes:
		return m.viewTextInput("정말로 진행하시겠습니까? 진행하려면 yes 를 입력하세요")
	case scrNoFSConfirm:
		return m.viewYesNo(fmt.Sprintf("%s 파티션에는 파일시스템이 없습니다.\next4로 포맷할까요?", m.selectedPartPath))
	case scrNoFSDeviceType:
		return m.viewTextInput(fmt.Sprintf("확인을 위해 파티션 경로를 다시 입력하세요: %s", m.expectedText))
	case scrMountPoint:
		return m.viewMountPoint()
	case scrMountPointWarnConfirm:
		return m.viewYesNo(warnStyle.Render(m.mountWarn) + "\n계속할까요?")
	case scrFstabConfirm:
		return m.viewYesNo("재부팅 시 자동 마운트되도록 /etc/fstab 에 등록할까요?")
	case scrUnmountConfirm:
		return m.viewYesNo(warnStyle.Render(fmt.Sprintf("%s 마운트를 해제할까요?", m.unmountTargetPath)))
	case scrRaidDiskSelect:
		return m.viewRaidDiskSelect()
	case scrRaidLevelChoice:
		return m.viewRaidLevelChoice()
	case scrRaidConfirmType:
		return m.viewRaidConfirmType()
	case scrLoadingSmart:
		return boxStyle.Render(fmt.Sprintf("%s SMART 정보를 읽는 중...", m.spin.View()))
	case scrSmartDetail:
		return m.viewSmartDetail()
	case scrRunning:
		return m.viewRunning()
	case scrRunError:
		return m.viewRunError()
	case scrSummary:
		return m.viewSummary()
	case scrFatalError:
		return boxStyle.Render(errorStyle.Render("오류: "+m.fatalErr.Error()) + "\n\n" + helpStyle.Render("q: 종료"))
	}
	return ""
}

// ---- shared helpers ----

// startRun switches to the running screen and kicks off the first step.
func (m *Model) startRun(steps []step, after screen) (tea.Model, tea.Cmd) {
	m.screen = scrRunning
	m.pendingSteps = steps
	m.runLog = nil
	m.afterRun = after
	if len(steps) == 0 {
		return m, nil
	}
	return m, tea.Batch(m.spin.Tick, runStepCmd(steps[0]))
}

func (m *Model) handleStepResult(msg stepResultMsg) (tea.Model, tea.Cmd) {
	if msg.log != "" {
		m.runLog = append(m.runLog, msg.log)
	}
	if msg.err != nil {
		m.runLog = append(m.runLog, errorStyle.Render("오류: "+msg.err.Error()))
		m.screen = scrRunError
		return m, nil
	}
	if len(m.pendingSteps) > 0 {
		m.pendingSteps = m.pendingSteps[1:]
	}
	if len(m.pendingSteps) == 0 {
		if m.afterRun == scrLoadingPartitions {
			m.screen = scrLoadingPartitions
			return m, loadPartitionsCmd(m.selectedDisk.Name)
		}
		m.screen = m.afterRun
		if m.afterRun == scrMountPoint {
			resetTextInput(m, "/data/hdd_"+strings.ToLower(m.selectedDisk.Label))
		}
		if m.afterRun == scrSummary {
			out, _ := diskutil.DiskFree(m.mountPoint)
			m.dfOutput = out
		}
		return m, nil
	}
	return m, runStepCmd(m.pendingSteps[0])
}

func resetTextInput(m *Model, placeholder string) {
	m.textInput.SetValue("")
	m.textInput.Placeholder = placeholder
	m.textInput.Focus()
	m.inputErr = ""
}

func joinLines(lines []string) string {
	return strings.Join(lines, "\n")
}
