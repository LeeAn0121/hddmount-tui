package diskutil

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"strconv"
	"strings"
)

// PrepareSystemUploads는 HDD 없이 시스템 디스크에 업로드 폴더와 웹 링크를 준비한다.
// 인자는 없다. /data가 별도로 마운트된 경우에는 실행하지 않는다.
// 작업 로그와 계정·경로·권한·링크 오류를 반환한다.
// 포맷·마운트·fstab 변경은 하지 않는다. 새 폴더만 www-data 권한으로 설정한다.
func PrepareSystemUploads() (string, error) {
	account, err := user.Lookup("www-data")

	if err != nil {
		return "", fmt.Errorf("www-data 계정 확인 실패: %w", err)
	}

	ownerID, err := strconv.Atoi(account.Uid)

	if err != nil {
		return "", fmt.Errorf("www-data UID 확인 실패: %w", err)
	}

	groupID, err := strconv.Atoi(account.Gid)

	if err != nil {
		return "", fmt.Errorf("www-data GID 확인 실패: %w", err)
	}

	var mountErr error
	checkMount := func(path string) bool {
		mounted, err := systemUploadMountpoint(path)

		if err != nil {
			mountErr = errors.Join(mountErr, err)

			return true
		}

		return mounted
	}

	output, err := prepareSystemUploadsAt(webRoot, dataUploads, ownerID, groupID, checkMount)

	return output, errors.Join(err, mountErr)
}

// systemUploadMountpoint는 존재하는 경로의 마운트 상태를 검사한다.
// path는 절대경로다. 미생성 경로는 false를 반환한다.
// 검사 도구 오류를 미마운트 상태로 취급하지 않고 오류로 반환한다.
func systemUploadMountpoint(path string) (bool, error) {
	if _, err := os.Lstat(path); errors.Is(err, os.ErrNotExist) {
		return false, nil
	} else if err != nil {
		return false, err
	}

	_, err := runCmd("mountpoint", "--quiet", path)

	if err == nil {
		return true, nil
	}

	var exitErr *exec.ExitError

	if errors.As(err, &exitErr) && exitErr.ExitCode() == 32 {
		return false, nil
	}

	return false, fmt.Errorf("%s 마운트 상태 검사 실패: %w", path, err)
}

// prepareSystemUploadsAt은 시스템 폴더 모드의 경로를 검사한 뒤 폴더와 링크를 준비한다.
// root·target은 절대 웹 루트·업로드 경로, ownerID·groupID는 새 폴더 소유자다.
// isMountpoint는 검사 실패도 true로 처리해야 한다. 로그와 준비 오류를 반환한다.
// 기존 폴더·파일의 내용과 권한은 보존한다. 실패 시 새 폴더를 임의 삭제하지 않는다.
func prepareSystemUploadsAt(root, target string, ownerID, groupID int, isMountpoint func(string) bool) (string, error) {
	if !filepath.IsAbs(root) || !filepath.IsAbs(target) || filepath.Clean(root) == filepath.Clean(target) {
		return "", fmt.Errorf("웹 루트와 대상은 서로 다른 절대경로여야 합니다")
	}

	dataRoot := filepath.Dir(target)

	if isMountpoint(dataRoot) || isMountpoint(target) {
		return "", fmt.Errorf("%s 또는 %s가 마운트된 상태입니다. 데이터 디스크에는 link-uploads를 사용하세요", dataRoot, target)
	}

	directories := []string{dataRoot, target}

	for _, name := range contentDirs {
		directories = append(directories, filepath.Join(target, name))
	}

	for _, directory := range directories {
		info, err := os.Lstat(directory)

		if errors.Is(err, os.ErrNotExist) {
			continue
		}

		if err != nil {
			return "", err
		}

		if !info.IsDir() {
			return "", fmt.Errorf("%s는 실제 디렉터리여야 합니다. 기존 파일·링크는 변경하지 않습니다", directory)
		}
	}

	for _, link := range []string{filepath.Join(root, "uploads"), filepath.Join(root, "public", "uploads")} {
		if _, err := checkUploadPath(link, target, isMountpoint); err != nil {
			return "", err
		}
	}

	logs := []string{"저장 방식: 시스템 디스크 폴더 (포맷·마운트·fstab 변경 없음)"}

	for index, directory := range directories {
		created, err := createSystemUploadDirectory(directory, ownerID, groupID, index != 0)

		if err != nil {
			return strings.Join(logs, "\n"), err
		}

		if created {
			logs = append(logs, "디렉터리 생성: "+directory)
		}
	}

	output, err := linkWebUploadsAt(root, target, isMountpoint)
	logs = append(logs, output)

	return strings.Join(logs, "\n"), err
}

// createSystemUploadDirectory는 없는 폴더만 생성하고 새 폴더의 권한을 설정한다.
// path는 검사한 경로, ownerID·groupID는 소유자, setOwner는 업로드 폴더 여부다.
// 생성 여부와 파일·권한 오류를 반환한다. 기존 폴더의 권한은 변경하지 않는다.
func createSystemUploadDirectory(path string, ownerID, groupID int, setOwner bool) (bool, error) {
	err := os.Mkdir(path, 0o755)

	if errors.Is(err, os.ErrExist) {
		info, statErr := os.Lstat(path)

		if statErr != nil || !info.IsDir() {
			return false, fmt.Errorf("준비 중 경로가 변경되었습니다: %s", path)
		}

		return false, nil
	}

	if err != nil {
		return false, fmt.Errorf("%s 생성 실패: %w", path, err)
	}

	if setOwner {
		if err := os.Chown(path, ownerID, groupID); err != nil {
			return true, fmt.Errorf("%s 새 폴더 소유자 설정 실패: %w", path, err)
		}
	}

	if err := os.Chmod(path, 0o755); err != nil {
		return true, fmt.Errorf("%s 새 폴더 권한 설정 실패: %w", path, err)
	}

	return true, nil
}
