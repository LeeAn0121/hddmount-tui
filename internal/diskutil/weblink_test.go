package diskutil

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func uploadTestPaths(t *testing.T) (string, string) {
	t.Helper()
	base := t.TempDir()
	root := filepath.Join(base, "html")
	target := filepath.Join(base, "data", "uploads")

	if err := os.MkdirAll(filepath.Join(root, "public"), 0o755); err != nil {
		t.Fatal(err)
	}

	if err := os.MkdirAll(target, 0o755); err != nil {
		t.Fatal(err)
	}

	return root, target
}

func requireSymlink(t *testing.T, target, link string) {
	t.Helper()

	if err := os.Symlink(target, link); err != nil {
		t.Fatalf("심볼릭 링크 생성 실패: %v", err)
	}
}

func noUploadMount(string) bool { return false }

func TestUploadLinksShareTargetAndAreIdempotent(t *testing.T) {
	root, target := uploadTestPaths(t)

	for attempt := 0; attempt < 2; attempt++ {
		if _, err := linkWebUploadsAt(root, target, noUploadMount); err != nil {
			t.Fatal(err)
		}

		for _, link := range []string{filepath.Join(root, "uploads"), filepath.Join(root, "public", "uploads")} {
			resolved, err := filepath.EvalSymlinks(link)

			if err != nil || resolved != target {
				t.Fatalf("%s 대상 불일치: %s, %v", link, resolved, err)
			}
		}
	}

	if err := os.WriteFile(filepath.Join(root, "uploads", "shared.txt"), []byte("shared"), 0o644); err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(filepath.Join(root, "public", "uploads", "shared.txt"))

	if err != nil || string(data) != "shared" {
		t.Fatalf("두 경로가 같은 파일을 공유하지 않음: %q, %v", data, err)
	}

	backups, _ := filepath.Glob(filepath.Join(root, ".uploads-before-data-*"))

	if len(backups) != 0 {
		t.Fatal("재실행 시 불필요한 백업 생성")
	}
}

func TestUploadLinksReplaceEmptyDirectoriesAndWrongLinks(t *testing.T) {
	root, target := uploadTestPaths(t)
	oldDir := filepath.Join(root, "uploads")
	publicLink := filepath.Join(root, "public", "uploads")

	if err := os.Mkdir(oldDir, 0o755); err != nil {
		t.Fatal(err)
	}

	requireSymlink(t, filepath.Join(root, "missing"), publicLink)
	logs, err := linkWebUploadsAt(root, target, noUploadMount)

	if err != nil || !strings.Contains(logs, "이전 경로 백업") {
		t.Fatalf("기존 경로 교체 실패: %s, %v", logs, err)
	}

	for _, parent := range []string{root, filepath.Join(root, "public")} {
		backups, _ := filepath.Glob(filepath.Join(parent, ".uploads-before-data-*", "uploads"))

		if len(backups) != 1 {
			t.Fatalf("원래 경로 백업 누락: %s", parent)
		}
	}
}

func TestUploadConflictLeavesBothPathsUntouched(t *testing.T) {
	for _, conflict := range []string{"directory", "file", "mount"} {
		t.Run(conflict, func(t *testing.T) {
			root, target := uploadTestPaths(t)
			first := filepath.Join(root, "uploads")
			second := filepath.Join(root, "public", "uploads")
			if err := os.Mkdir(first, 0o755); err != nil {
				t.Fatal(err)
			}
			isMount := noUploadMount

			if conflict == "file" {
				if err := os.WriteFile(second, []byte("keep"), 0o644); err != nil {
					t.Fatal(err)
				}
			} else {
				if err := os.Mkdir(second, 0o755); err != nil {
					t.Fatal(err)
				}

				if conflict == "directory" {
					if err := os.WriteFile(filepath.Join(second, "keep.txt"), []byte("keep"), 0o644); err != nil {
						t.Fatal(err)
					}
				} else {
					isMount = func(path string) bool { return path == second }
				}
			}

			if _, err := linkWebUploadsAt(root, target, isMount); err == nil {
				t.Fatal("기존 데이터·파일·마운트 충돌 허용")
			}

			info, err := os.Lstat(first)

			if err != nil || !info.IsDir() {
				t.Fatal("첫 경로가 부분 변경됨")
			}

			if conflict == "directory" {
				data, err := os.ReadFile(filepath.Join(second, "keep.txt"))
				if err != nil || string(data) != "keep" {
					t.Fatal("기존 데이터 변경")
				}
			}
		})
	}
}

func TestUploadLinksRollbackEarlierChange(t *testing.T) {
	root, target := uploadTestPaths(t)
	first := filepath.Join(root, "uploads")
	second := filepath.Join(root, "public", "uploads")
	for _, path := range []string{first, second} {
		if err := os.Mkdir(path, 0o755); err != nil {
			t.Fatal(err)
		}
	}

	checks := 0
	isMount := func(path string) bool {
		if path == second {
			checks++
		}
		return path == second && checks > 1
	}

	if _, err := linkWebUploadsAt(root, target, isMount); err == nil {
		t.Fatal("작업 도중 발생한 충돌 누락")
	}

	info, err := os.Lstat(first)

	if err != nil || !info.IsDir() {
		t.Fatal("첫 경로의 원래 빈 디렉터리 복구 실패")
	}
}

func TestUploadLinkToExistingDataIsNotReplaced(t *testing.T) {
	root, target := uploadTestPaths(t)
	oldTarget := filepath.Join(t.TempDir(), "old-uploads")

	if err := os.Mkdir(oldTarget, 0o755); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(filepath.Join(oldTarget, "keep.txt"), []byte("keep"), 0o644); err != nil {
		t.Fatal(err)
	}

	link := filepath.Join(root, "uploads")
	requireSymlink(t, oldTarget, link)

	if _, err := linkWebUploadsAt(root, target, noUploadMount); err == nil {
		t.Fatal("기존 데이터로 향하는 링크가 교체됨")
	}

	resolved, err := filepath.EvalSymlinks(link)

	if err != nil || resolved != oldTarget {
		t.Fatal("원래 데이터 경로가 유지되지 않음")
	}
}
