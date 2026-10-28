package diskutil

import (
	"fmt"
	"os"
	"strings"
)

const (
	// webRoot is the CMS document root created by the Cloud3.3 installer.
	webRoot = "/var/www/html"

	// dataUploads is the uploads directory on a dedicated data disk.
	dataUploads = "/data/uploads"
)

// LinkWebUploads connects the CMS upload paths to the real uploads directory.
//
// # Behavior
//
// /data/uploads 가 있으면 그 경로를 실체로 사용한다.
// 이 때 /var/www/html/uploads 와 /var/www/html/public/uploads 를 모두 연결한다.
// /data/uploads 가 없으면 /var/www/html/uploads 를 실체로 사용한다.
// 이 때 /var/www/html/public/uploads 만 연결한다.
// 두 경우 모두 대상 경로가 이미 있으면 연결을 건너뛴다.
//
// # Returns
//
// 수행한 작업 로그. 웹 루트가 없으면 건너뛴 사실만 담은 로그를 반환한다.
//
// # Errors
//
// 디렉터리 생성이나 심볼릭 링크 생성이 실패하면 오류를 반환한다.
func LinkWebUploads() (string, error) {
	var logs []string

	if !isDir(webRoot) {
		return webRoot + " 가 없습니다. 업로드 링크 작업을 건너뜁니다.", nil
	}

	uploadTarget := webRoot + "/uploads"

	if isDir(dataUploads) {
		uploadTarget = dataUploads
		logs = append(logs, "업로드 실제 경로: "+uploadTarget+" (데이터 디스크)")

		out, err := linkPath(uploadTarget, webRoot+"/uploads")
		logs = append(logs, out)
		if err != nil {
			return strings.Join(logs, "\n"), err
		}
	} else {
		logs = append(logs, "업로드 실제 경로: "+uploadTarget+" ("+dataUploads+" 없음)")

		if !isDir(uploadTarget) {
			if err := os.MkdirAll(uploadTarget, 0o755); err != nil {
				return strings.Join(logs, "\n"), fmt.Errorf("%s 생성 실패: %w", uploadTarget, err)
			}
			logs = append(logs, "디렉터리 생성: "+uploadTarget)
		}
	}

	publicRoot := webRoot + "/public"
	if !isDir(publicRoot) {
		logs = append(logs, publicRoot+" 가 없습니다. 퍼블릭 링크를 건너뜁니다.")
		return strings.Join(logs, "\n"), nil
	}

	out, err := linkPath(uploadTarget, publicRoot+"/uploads")
	logs = append(logs, out)
	if err != nil {
		return strings.Join(logs, "\n"), err
	}

	return strings.Join(logs, "\n"), nil
}

// linkPath creates a symlink at linkName pointing to target.
//
// 이미 존재하는 경로는 건드리지 않는다. 기존 디렉터리나 링크를 덮어쓰면
// 업로드 파일이 보이지 않게 될 수 있다.
func linkPath(target, linkName string) (string, error) {
	if _, err := os.Lstat(linkName); err == nil {
		return linkName + " 가 이미 존재합니다. 링크 생성을 건너뜁니다.", nil
	}

	if err := os.Symlink(target, linkName); err != nil {
		return "", fmt.Errorf("%s -> %s 링크 생성 실패: %w", linkName, target, err)
	}

	return "링크 생성: " + linkName + " -> " + target, nil
}

func isDir(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return info.IsDir()
}
