//go:build linux

package diskutil

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestPrepareSystemUploadsCreatesFolders는 마운트 없이 표준 폴더와 공유 링크를 준비한다.
func TestPrepareSystemUploadsCreatesFolders(t *testing.T) {
	base := t.TempDir()
	root := filepath.Join(base, "html")
	target := filepath.Join(base, "data", "uploads")
	output, err := prepareSystemUploadsAt(root, target, os.Getuid(), os.Getgid(), noUploadMount)

	if err != nil || !strings.Contains(output, "포맷·마운트·fstab 변경 없음") {
		t.Fatalf("시스템 폴더 준비 실패: %s, %v", output, err)
	}

	for _, name := range contentDirs {
		info, err := os.Stat(filepath.Join(target, name))

		if err != nil || !info.IsDir() || info.Mode().Perm() != 0o755 {
			t.Fatalf("표준 폴더 준비 실패: %s, %v", name, err)
		}
	}

	for _, link := range []string{filepath.Join(root, "uploads"), filepath.Join(root, "public", "uploads")} {
		resolved, err := filepath.EvalSymlinks(link)

		if err != nil || resolved != target {
			t.Fatalf("웹 링크 대상 불일치: %s, %s, %v", link, resolved, err)
		}
	}

	marker := filepath.Join(target, "keep.txt")

	if err := os.WriteFile(marker, []byte("keep"), 0o600); err != nil {
		t.Fatal(err)
	}

	if err := os.Chmod(filepath.Join(target, "photo"), 0o700); err != nil {
		t.Fatal(err)
	}

	if _, err := prepareSystemUploadsAt(root, target, os.Getuid(), os.Getgid(), noUploadMount); err != nil {
		t.Fatal(err)
	}

	content, err := os.ReadFile(filepath.Join(root, "public", "uploads", "keep.txt"))

	if err != nil || string(content) != "keep" {
		t.Fatal("재실행이 기존 데이터를 변경했습니다")
	}

	info, err := os.Stat(filepath.Join(target, "photo"))

	if err != nil || info.Mode().Perm() != 0o700 {
		t.Fatal("재실행이 기존 폴더 권한을 변경했습니다")
	}
}

// TestPrepareSystemUploadsRejectsMountedStorage는 마운트된 경로를 시스템 폴더로 취급하지 않는다.
func TestPrepareSystemUploadsRejectsMountedStorage(t *testing.T) {
	for _, mountedPath := range []string{"data", "uploads"} {
		t.Run(mountedPath, func(t *testing.T) {
			base := t.TempDir()
			root := filepath.Join(base, "html")
			target := filepath.Join(base, "data", "uploads")
			mounted := filepath.Dir(target)

			if mountedPath == "uploads" {
				mounted = target
			}

			_, err := prepareSystemUploadsAt(root, target, os.Getuid(), os.Getgid(), func(path string) bool {
				return path == mounted
			})

			if err == nil {
				t.Fatal("마운트된 경로를 허용했습니다")
			}

			if _, err := os.Lstat(filepath.Dir(target)); !os.IsNotExist(err) {
				t.Fatal("마운트 검사 실패 후 폴더를 생성했습니다")
			}
		})
	}
}

// TestPrepareSystemUploadsConflictPreservesData는 충돌을 폴더 생성 전에 거부한다.
func TestPrepareSystemUploadsConflictPreservesData(t *testing.T) {
	base := t.TempDir()
	root := filepath.Join(base, "html")
	target := filepath.Join(base, "data", "uploads")
	existing := filepath.Join(root, "uploads")

	if err := os.MkdirAll(existing, 0o700); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(filepath.Join(existing, "keep.txt"), []byte("keep"), 0o600); err != nil {
		t.Fatal(err)
	}

	if _, err := prepareSystemUploadsAt(root, target, os.Getuid(), os.Getgid(), noUploadMount); err == nil {
		t.Fatal("기존 웹 데이터를 덮어쓰는 경로를 허용했습니다")
	}

	if _, err := os.Lstat(filepath.Dir(target)); !os.IsNotExist(err) {
		t.Fatal("충돌 검사 실패 후 폴더를 생성했습니다")
	}

	content, err := os.ReadFile(filepath.Join(existing, "keep.txt"))

	if err != nil || string(content) != "keep" {
		t.Fatal("기존 웹 데이터를 변경했습니다")
	}
}

// TestPrepareSystemUploadsRejectsSymlinkDirectories는 데이터 경로의 링크를 따라가지 않는다.
func TestPrepareSystemUploadsRejectsSymlinkDirectories(t *testing.T) {
	for _, position := range []string{"data", "uploads", "photo"} {
		t.Run(position, func(t *testing.T) {
			base := t.TempDir()
			root := filepath.Join(base, "html")
			target := filepath.Join(base, "data", "uploads")
			link := filepath.Dir(target)

			if position == "uploads" {
				link = target
			} else if position == "photo" {
				link = filepath.Join(target, "photo")
			}

			if err := os.MkdirAll(filepath.Dir(link), 0o755); err != nil {
				t.Fatal(err)
			}

			requireSymlink(t, t.TempDir(), link)

			if _, err := prepareSystemUploadsAt(root, target, os.Getuid(), os.Getgid(), noUploadMount); err == nil {
				t.Fatal("데이터 경로의 링크를 허용했습니다")
			}

			if _, err := os.Lstat(root); !os.IsNotExist(err) {
				t.Fatal("데이터 경로 검사 실패 후 웹 루트를 변경했습니다")
			}
		})
	}
}

// TestSystemUploadMountpointFailsClosed는 검사 도구 실패와 미마운트를 구분한다.
func TestSystemUploadMountpointFailsClosed(t *testing.T) {
	directory := t.TempDir()
	mounted, err := systemUploadMountpoint(directory)

	if err != nil || mounted {
		t.Fatalf("임시 폴더 마운트 검사 실패: %v, %v", mounted, err)
	}

	t.Setenv("PATH", "")

	if _, err := systemUploadMountpoint(directory); err == nil {
		t.Fatal("검사 도구 부재를 미마운트로 처리했습니다")
	}
}
