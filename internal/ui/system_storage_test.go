package ui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

// TestSystemStorageConfirmationDefaultsToNo는 두 목록에서 명시적 동의 전 실행을 차단한다.
func TestSystemStorageConfirmationDefaultsToNo(t *testing.T) {
	for _, origin := range []screen{scrDiskList, scrNoDisks} {
		for _, cancelKey := range []tea.KeyType{tea.KeyEnter, tea.KeyEsc} {
			model := New()
			model.screen = origin
			_, command := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'p'}})

			if command != nil || model.screen != scrSystemStorageConfirm || model.yesNoCursor != 1 {
				t.Fatal("시스템 폴더 선택이 작업을 실행하거나 기본 선택을 예로 설정했습니다")
			}

			_, command = model.Update(tea.KeyMsg{Type: cancelKey})

			if command != nil || model.screen != origin || model.isSystemStorage || len(model.pendingSteps) != 0 {
				t.Fatal("취소가 원래 목록으로 복귀하지 않거나 작업을 시작했습니다")
			}
		}
	}
}

// TestSystemStorageConfirmationSchedulesOnlyPreparation는 동의 후 폴더 준비 하나만 예약한다.
func TestSystemStorageConfirmationSchedulesOnlyPreparation(t *testing.T) {
	model := New()
	model.screen = scrNoDisks
	model.startSystemStorageConfirm()
	_, command := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'y'}})

	if command == nil || model.screen != scrRunning || !model.isSystemStorage || len(model.pendingSteps) != 1 {
		t.Fatal("폴더 준비 단일 단계가 예약되지 않았습니다")
	}

	if model.pendingSteps[0].target != "/data/uploads" || model.selectedPartPath != "" || model.fstabChoice {
		t.Fatal("시스템 폴더 모드가 디스크 작업 상태를 포함합니다")
	}

	if model.afterRun != scrSummary {
		t.Fatal("폴더 준비 완료 화면이 지정되지 않았습니다")
	}

	model.screen = scrSummary
	view := model.View()

	if !strings.Contains(view, "시스템 디스크 폴더: /data/uploads") || strings.Contains(view, "파티션:") {
		t.Fatal("완료 화면이 폴더 준비를 파티션 마운트로 표시합니다")
	}
}
