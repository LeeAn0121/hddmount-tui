package diskutil

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const (
	webRoot     = "/var/www/html"
	dataUploads = "/data/uploads"
)

// LinkWebUploads는 두 CMS 업로드 경로를 /data/uploads에 연결한다.
//
// /data가 마운트되지 않으면 시스템 디스크에 업로드 폴더를 만들지 않는다.
// 빈 디렉터리와 다른 링크는 백업한 뒤 교체한다. 기존 데이터와 마운트는 변경하지 않는다.
// 반환값은 작업 로그와 오류다. 디스크를 마운트하거나 포맷하지 않는다.
func LinkWebUploads() (string, error) {
	if !IsMountpoint(filepath.Dir(dataUploads)) {
		return "", fmt.Errorf("/data가 마운트되어 있지 않습니다. 데이터 디스크를 /data에 먼저 마운트하세요")
	}

	return linkWebUploadsAt(webRoot, dataUploads, IsMountpoint)
}

type uploadLinkChange struct {
	path    string
	backup  string
	created bool
}

// checkUploadPath는 경로를 교체할 수 있는지 검사한다.
//
// 인자는 링크 경로와 목표 디렉터리, 마운트 검사 함수다.
// 올바른 링크이면 true, 변경 가능한 빈 경로이면 false를 반환한다.
// 기존 데이터·파일·마운트와 검사 오류는 오류로 반환한다.
func checkUploadPath(link, target string, isMountpoint func(string) bool) (bool, error) {
	info, err := os.Lstat(link)

	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}

	if err != nil {
		return false, err
	}

	if isMountpoint(link) {
		return false, fmt.Errorf("%s는 마운트된 경로입니다. 마운트를 정리한 뒤 다시 실행하세요", link)
	}

	if info.Mode()&os.ModeSymlink != 0 {
		resolved, linkErr := filepath.EvalSymlinks(link)
		resolvedTarget, targetErr := filepath.EvalSymlinks(target)

		if linkErr == nil && targetErr == nil && filepath.Clean(resolved) == filepath.Clean(resolvedTarget) {
			return true, nil
		}

		if linkErr != nil {
			if errors.Is(linkErr, os.ErrNotExist) {
				return false, nil
			}

			return false, linkErr
		}

		linkedInfo, err := os.Stat(link)

		if err != nil || !linkedInfo.IsDir() {
			return false, fmt.Errorf("%s의 기존 링크 대상을 확인하세요", link)
		}

		info = linkedInfo
	}

	if !info.IsDir() {
		return false, fmt.Errorf("%s는 디렉터리나 링크가 아닙니다. 기존 파일을 확인하세요", link)
	}

	entries, err := os.ReadDir(link)

	if err != nil {
		return false, err
	}

	if len(entries) != 0 {
		return false, fmt.Errorf("%s에 기존 데이터가 있습니다. 쓰기를 중지하고 %s로 백업·병합한 뒤 빈 디렉터리 상태에서 다시 실행하세요", link, target)
	}

	return false, nil
}

// linkWebUploadsAt은 두 경로를 모두 검사한 뒤 링크를 교체한다.
//
// 인자는 서로 다른 절대 웹 루트·대상 경로와 마운트 검사 함수다.
// 반환값은 로그와 생성·검사·복구 오류다. 호출 중 서비스의 업로드 쓰기를 중지해야 한다.
func linkWebUploadsAt(root, target string, isMountpoint func(string) bool) (string, error) {
	if !filepath.IsAbs(root) || !filepath.IsAbs(target) || filepath.Clean(root) == filepath.Clean(target) {
		return "", fmt.Errorf("웹 루트와 대상은 서로 다른 절대경로여야 합니다")
	}

	if err := os.MkdirAll(target, 0o755); err != nil {
		return "", fmt.Errorf("업로드 대상 생성 실패: %w", err)
	}

	info, err := os.Lstat(target)

	if err != nil || !info.IsDir() {
		return "", fmt.Errorf("업로드 대상은 실제 디렉터리여야 합니다: %s", target)
	}

	if err := os.MkdirAll(filepath.Join(root, "public"), 0o755); err != nil {
		return "", err
	}

	paths := []string{filepath.Join(root, "uploads"), filepath.Join(root, "public", "uploads")}
	var pending []string
	var logs []string

	for _, link := range paths {
		linked, err := checkUploadPath(link, target, isMountpoint)

		if err != nil {
			return "", err
		}

		if linked {
			logs = append(logs, "연결 확인: "+link+" -> "+target)
			continue
		}

		pending = append(pending, link)
	}

	var changes []uploadLinkChange

	for _, link := range pending {
		linked, err := checkUploadPath(link, target, isMountpoint)

		if err != nil {
			return strings.Join(logs, "\n"), errors.Join(err, restoreUploadLinks(changes))
		}

		if linked {
			continue
		}

		change := uploadLinkChange{path: link}

		if _, err := os.Lstat(link); err == nil {
			backup, err := os.MkdirTemp(filepath.Dir(link), ".uploads-before-data-*")

			if err != nil {
				return strings.Join(logs, "\n"), errors.Join(err, restoreUploadLinks(changes))
			}

			change.backup = filepath.Join(backup, "uploads")

			if err := os.Rename(link, change.backup); err != nil {
				return strings.Join(logs, "\n"), errors.Join(err, restoreUploadLinks(changes))
			}
		} else if !errors.Is(err, os.ErrNotExist) {
			return strings.Join(logs, "\n"), errors.Join(err, restoreUploadLinks(changes))
		}

		changes = append(changes, change)

		if err := os.Symlink(target, link); err != nil {
			return strings.Join(logs, "\n"), errors.Join(fmt.Errorf("%s 링크 생성 실패: %w", link, err), restoreUploadLinks(changes))
		}

		changes[len(changes)-1].created = true

		if change.backup != "" {
			logs = append(logs, "이전 경로 백업: "+change.backup)
		}

		logs = append(logs, "링크 생성: "+link+" -> "+target)
	}

	return strings.Join(logs, "\n"), nil
}

// restoreUploadLinks는 이번에 생성한 링크를 제거하고 백업 경로를 역순 복원한다.
//
// 인자는 적용한 변경 목록이다. 새로 생긴 일반 파일·데이터는 삭제하지 않는다.
// 반환값은 모든 복구 오류다. 복구 실패 시 백업 경로를 오류에 포함한다.
func restoreUploadLinks(changes []uploadLinkChange) error {
	var restoreErrors []error

	for index := len(changes) - 1; index >= 0; index-- {
		change := changes[index]

		if change.created {
			info, err := os.Lstat(change.path)

			if err == nil && info.Mode()&os.ModeSymlink == 0 {
				restoreErrors = append(restoreErrors, fmt.Errorf("복구할 경로가 변경되었습니다: %s, 백업: %s", change.path, change.backup))
				continue
			}

			if err != nil && !errors.Is(err, os.ErrNotExist) {
				restoreErrors = append(restoreErrors, err)
				continue
			}

			if err == nil {
				if err := os.Remove(change.path); err != nil {
					restoreErrors = append(restoreErrors, err)
					continue
				}
			}
		}

		if change.backup == "" {
			continue
		}

		if _, err := os.Lstat(change.path); !errors.Is(err, os.ErrNotExist) {
			restoreErrors = append(restoreErrors, fmt.Errorf("복구 경로가 존재하거나 검사할 수 없습니다: %s, 백업: %s", change.path, change.backup))
			continue
		}

		if err := os.Rename(change.backup, change.path); err != nil {
			restoreErrors = append(restoreErrors, fmt.Errorf("%s 백업 %s 복원 실패: %w", change.path, change.backup, err))
		}
	}

	return errors.Join(restoreErrors...)
}
