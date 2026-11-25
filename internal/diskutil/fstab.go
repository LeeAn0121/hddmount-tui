package diskutil

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const fstabPath = "/etc/fstab"

// VerifyFstab는 마운트 없이 fstab의 구문과 사용 가능성을 검사한다.
// filename은 검사할 fstab 파일 경로다. 성공 시 진단 출력을 반환한다.
// 파일·도구·구문·장치 오류를 반환한다. findmnt 경고만 발생하면 성공한다.
func VerifyFstab(filename string) (string, error) {
	return runCmd("findmnt", "--verify", "--verbose", "--tab-file", filename)
}

// SetupFstab은 파티션 UUID의 자동 마운트 항목을 백업 후 등록한다.
// part는 파티션 경로, mountPoint는 이미 마운트된 절대경로다.
// 작업 로그와 UUID·파일·검사 오류를 반환한다. 검사 실패 시 원본을 복구한다.
func SetupFstab(part, mountPoint string) (string, error) {
	uuid, err := GetUUID(part)

	if err != nil || uuid == "" {
		return "", fmt.Errorf("UUID를 확인할 수 없어 fstab 등록을 건너뜁니다")
	}

	return setupFstabAt(fstabPath, uuid, mountPoint)
}

// setupFstabAt은 지정한 파일에 ext4 UUID 항목을 추가하고 검증한다.
// filename은 기존 fstab, uuid는 파일시스템 UUID, mountPoint는 대상 경로다.
// 로그와 파일·검증·복구 오류를 반환한다. 원본 백업은 성공·실패 모두 보존한다.
func setupFstabAt(filename, uuid, mountPoint string) (string, error) {
	existing, err := os.ReadFile(filename)

	if err != nil {
		return "", fmt.Errorf("fstab 읽기 실패: %w", err)
	}

	if strings.Contains(string(existing), uuid) {
		return "이미 fstab에 등록된 UUID 입니다. 건너뜁니다.", nil
	}

	backup, err := os.CreateTemp(filepath.Dir(filename), filepath.Base(filename)+".bak.*")

	if err != nil {
		return "", fmt.Errorf("fstab 백업 생성 실패: %w", err)
	}

	_, writeErr := backup.Write(existing)
	closeErr := backup.Close()

	if err := errors.Join(writeErr, closeErr); err != nil {
		return "", fmt.Errorf("fstab 백업 실패: %w", err)
	}

	logs := []string{"fstab 백업 생성: " + backup.Name()}
	entry := fmt.Sprintf("UUID=%s  %s  ext4  defaults,nofail  0  2\n", uuid, escapeFstabPath(mountPoint))
	content := string(existing)

	if content != "" && !strings.HasSuffix(content, "\n") {
		content += "\n"
	}

	if err := os.WriteFile(filename, []byte(content+entry), 0o644); err != nil {
		return strings.Join(logs, "\n"), restoreFstab(filename, existing, backup.Name(), err)
	}

	logs = append(logs, "fstab 항목 추가 완료")
	out, err := VerifyFstab(filename)

	if err != nil {
		logs = append(logs, "fstab 검사 실패, 백업으로 복구합니다:", out)

		return strings.Join(logs, "\n"), restoreFstab(filename, existing, backup.Name(), err)
	}

	logs = append(logs, "fstab 검증 통과 (findmnt --verify)")

	return strings.Join(logs, "\n"), nil
}

// restoreFstab은 원본 바이트를 복구하고 원래 오류와 복구 오류를 반환한다.
// filename은 복구 대상, original은 원본, backup은 보존한 백업 경로다.
// cause는 등록·검사 실패 원인이다. 복구 실패 시 백업 경로를 오류에 포함한다.
func restoreFstab(filename string, original []byte, backup string, cause error) error {
	if err := os.WriteFile(filename, original, 0o644); err != nil {
		return errors.Join(cause, fmt.Errorf("fstab 복구 실패, 백업 보존: %s: %w", backup, err))
	}

	return fmt.Errorf("fstab 등록 실패로 원본을 복구했습니다: %w", cause)
}

// escapeFstabPath는 경로의 구분 문자와 역슬래시를 fstab 이스케이프로 변환한다.
// value는 마운트 경로다. 변환한 문자열을 반환하며 파일은 변경하지 않는다.
func escapeFstabPath(value string) string {
	return strings.NewReplacer("\\", `\134`, " ", `\040`, "\t", `\011`, "\n", `\012`).Replace(value)
}
