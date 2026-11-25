//go:build linux

package diskutil

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// TestVerifyFstab는 실제 findmnt로 정상 항목과 구문 오류를 구분한다.
func TestVerifyFstab(t *testing.T) {
	directory := t.TempDir()
	filename := filepath.Join(directory, "fstab")
	valid := "tmpfs " + escapeFstabPath(directory) + " tmpfs defaults 0 0\n"

	for _, test := range []struct {
		name    string
		content string
		wantErr bool
	}{
		{name: "valid", content: valid},
		{name: "malformed", content: "broken\n", wantErr: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			if err := os.WriteFile(filename, []byte(test.content), 0o600); err != nil {
				t.Fatal(err)
			}

			output, err := VerifyFstab(filename)

			if (err != nil) != test.wantErr {
				t.Fatalf("findmnt 결과 불일치: %s, %v", output, err)
			}
		})
	}
}

// TestSetupFstabRestoresOriginal는 검사 실패 시 원본과 서로 다른 백업을 보존한다.
func TestSetupFstabRestoresOriginal(t *testing.T) {
	directory := t.TempDir()
	filename := filepath.Join(directory, "fstab")
	original := []byte("broken\n")

	if err := os.WriteFile(filename, original, 0o600); err != nil {
		t.Fatal(err)
	}

	for attempt := 0; attempt < 2; attempt++ {
		_, err := setupFstabAt(filename, "00000000-0000-0000-0000-000000000000", directory)

		if err == nil || !strings.Contains(err.Error(), "원본을 복구") {
			t.Fatalf("검사 실패와 복구 결과 누락: %v", err)
		}

		content, err := os.ReadFile(filename)

		if err != nil || string(content) != string(original) {
			t.Fatalf("원본 변경: %q, %v", content, err)
		}
	}

	backups, err := filepath.Glob(filename + ".bak.*")

	if err != nil || len(backups) != 2 {
		t.Fatalf("각 시도의 백업 보존 실패: %v, %v", backups, err)
	}

	for _, backup := range backups {
		content, err := os.ReadFile(backup)

		if err != nil || string(content) != string(original) {
			t.Fatalf("백업 내용 변경: %q, %v", content, err)
		}
	}
}

// TestRunCmdUsesStableLocale는 호스트 언어와 무관하게 C 로캘을 전달한다.
func TestRunCmdUsesStableLocale(t *testing.T) {
	t.Setenv("LC_ALL", "invalid-locale")
	t.Setenv("LANG", "invalid-locale")
	t.Setenv("LANGUAGE", "invalid-locale")
	output, err := runCmd("sh", "-c", `printf '%s/%s/%s' "$LC_ALL" "$LANG" "$LANGUAGE"`)

	if err != nil || output != "C/C/C" {
		t.Fatalf("로캘 정규화 실패: %q, %v", output, err)
	}
}

// TestLsblkColumns는 배포판의 실제 lsblk가 명시한 출력 열을 지원하는지 검사한다.
func TestLsblkColumns(t *testing.T) {
	for _, columns := range []string{"NAME,SIZE,TYPE,MODEL", "NAME,TYPE,FSTYPE,MOUNTPOINT"} {
		output, err := exec.Command("lsblk", "--pairs", "--bytes", "--nodeps", "--output", columns).CombinedOutput()

		if err != nil {
			t.Fatalf("lsblk %s 실패: %s, %v", columns, output, err)
		}
	}
}
