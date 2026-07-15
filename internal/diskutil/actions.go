package diskutil

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"time"
)

func runCmd(name string, args ...string) (string, error) {
	cmd := exec.Command(name, args...)
	out, err := cmd.CombinedOutput()
	trimmed := strings.TrimSpace(string(out))
	if err != nil {
		if trimmed == "" {
			return trimmed, fmt.Errorf("%s: %w", name, err)
		}
		return trimmed, fmt.Errorf("%s: %w: %s", name, err, trimmed)
	}
	return trimmed, nil
}

// WipeSignatures removes any existing filesystem/partition-table signatures
// from a whole disk so a fresh partition table can be created cleanly.
func WipeSignatures(dev string) (string, error) {
	return runCmd("wipefs", "-a", dev)
}

// CreatePartitionTable creates a fresh GPT label with a single partition
// spanning the entire disk. GPT is used unconditionally (not just for >2TB
// disks) so the same code path works for every supported size.
func CreatePartitionTable(dev string) (string, error) {
	var logs []string

	out, err := runCmd("parted", "-s", dev, "mklabel", "gpt")
	logs = append(logs, out)
	if err != nil {
		return strings.Join(logs, "\n"), fmt.Errorf("파티션 테이블 생성 실패: %w", err)
	}

	out, err = runCmd("parted", "-s", dev, "mkpart", "primary", "ext4", "0%", "100%")
	logs = append(logs, out)
	if err != nil {
		return strings.Join(logs, "\n"), fmt.Errorf("파티션 생성 실패: %w", err)
	}

	_, _ = runCmd("partprobe", dev)
	time.Sleep(2 * time.Second)

	return strings.Join(logs, "\n"), nil
}

// NewPartitionPath guesses the resulting first-partition device path for a
// disk right after CreatePartitionTable (handles both /dev/sdb1 and the
// /dev/nvme0n1p1-style naming used by NVMe/loop devices).
func NewPartitionPath(dev string) string {
	p1 := dev + "1"
	if _, err := os.Stat(p1); err == nil {
		return p1
	}
	alt := dev + "p1"
	if _, err := os.Stat(alt); err == nil {
		return alt
	}
	return p1
}

// FormatExt4 creates an ext4 filesystem on the given partition.
func FormatExt4(part string) (string, error) {
	return runCmd("mkfs.ext4", "-F", part)
}

// MountPartition creates the mount point directory (if needed) and mounts
// the partition onto it.
func MountPartition(part, mountPoint string) (string, error) {
	if err := os.MkdirAll(mountPoint, 0o755); err != nil {
		return "", fmt.Errorf("마운트 포인트 생성 실패: %w", err)
	}
	return runCmd("mount", part, mountPoint)
}

// IsMountpoint reports whether path is currently a mount point.
func IsMountpoint(path string) bool {
	return exec.Command("mountpoint", "-q", path).Run() == nil
}

// DirHasContent reports whether path exists and is a non-empty directory.
func DirHasContent(path string) bool {
	entries, err := os.ReadDir(path)
	if err != nil {
		return false
	}
	return len(entries) > 0
}

// GetUUID returns the filesystem UUID of a partition, used for fstab entries.
func GetUUID(part string) (string, error) {
	out, err := exec.Command("blkid", "-s", "UUID", "-o", "value", part).Output()
	if err != nil {
		return "", fmt.Errorf("UUID 조회 실패: %w", err)
	}
	return strings.TrimSpace(string(out)), nil
}

const fstabPath = "/etc/fstab"

// SetupFstab appends a `UUID=... <mountPoint> ext4 defaults,nofail 0 2` entry
// to /etc/fstab. It backs up fstab first, and if `mount -a --fake` reports a
// syntax problem afterwards, the backup is restored automatically so a typo
// never breaks the next reboot.
func SetupFstab(part, mountPoint string) (string, error) {
	var logs []string

	uuid, err := GetUUID(part)
	if err != nil || uuid == "" {
		return "", fmt.Errorf("UUID를 확인할 수 없어 fstab 등록을 건너뜁니다")
	}

	existing, err := os.ReadFile(fstabPath)
	if err != nil {
		return "", fmt.Errorf("fstab 읽기 실패: %w", err)
	}
	if strings.Contains(string(existing), uuid) {
		return "이미 fstab에 등록된 UUID 입니다. 건너뜁니다.", nil
	}

	backupPath := fmt.Sprintf("%s.bak.%s", fstabPath, time.Now().Format("20060102150405"))
	if err := os.WriteFile(backupPath, existing, 0o644); err != nil {
		return "", fmt.Errorf("fstab 백업 실패: %w", err)
	}
	logs = append(logs, "fstab 백업 생성: "+backupPath)

	line := fmt.Sprintf("UUID=%s  %s  ext4  defaults,nofail  0  2\n", uuid, mountPoint)
	f, err := os.OpenFile(fstabPath, os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return strings.Join(logs, "\n"), fmt.Errorf("fstab 쓰기 실패: %w", err)
	}
	if _, err := f.WriteString(line); err != nil {
		f.Close()
		return strings.Join(logs, "\n"), fmt.Errorf("fstab 쓰기 실패: %w", err)
	}
	f.Close()
	logs = append(logs, "fstab 항목 추가 완료")

	out, err := runCmd("mount", "-a", "--fake")
	if err != nil {
		logs = append(logs, "fstab 문법 오류 감지, 백업으로 롤백합니다:", out)
		orig, _ := os.ReadFile(backupPath)
		_ = os.WriteFile(fstabPath, orig, 0o644)
		return strings.Join(logs, "\n"), fmt.Errorf("fstab 문법 오류로 롤백되었습니다")
	}
	logs = append(logs, "fstab 문법 검증 통과 (mount -a --fake)")

	return strings.Join(logs, "\n"), nil
}

// DiskFree returns `df -h` output for a mounted path, for the final summary screen.
func DiskFree(path string) (string, error) {
	return runCmd("df", "-h", path)
}

// Unmount detaches a mounted partition or path.
func Unmount(path string) (string, error) {
	return runCmd("umount", path)
}

// SetLabel sets the ext4 filesystem label on a partition, so `lsblk -o LABEL`
// / `blkid` and the disk's physical sticker can agree on a human-readable
// name instead of just a size bucket.
func SetLabel(part, label string) (string, error) {
	return runCmd("e2label", part, label)
}

var labelSanitizeRe = regexp.MustCompile(`[^A-Za-z0-9_-]+`)

// SanitizeLabel reduces s to the character set and length (16 bytes) that
// ext2/3/4 filesystem labels accept.
func SanitizeLabel(s string) string {
	s = labelSanitizeRe.ReplaceAllString(s, "_")
	s = strings.Trim(s, "_")
	if len(s) > 16 {
		s = s[:16]
	}
	if s == "" {
		s = "hdd"
	}
	return s
}

// SmartHealth runs a quick `smartctl -H` health check on a whole-disk device
// and reduces it to a short verdict. smartctl's exit code is a bitmask that
// is nonzero even when the check itself succeeds but flags a problem, so the
// verdict is derived from the output text rather than the error. A missing
// smartctl binary, USB bridges that don't pass SMART through, or any other
// failure to get a clear answer all fall back to "확인불가" rather than
// blocking the rest of the tool.
func SmartHealth(dev string) string {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	out, _ := exec.CommandContext(ctx, "smartctl", "-H", dev).CombinedOutput()
	text := string(out)
	switch {
	case strings.Contains(text, "FAILED"):
		return "위험(FAILED)"
	case strings.Contains(text, "PASSED") || strings.Contains(text, "OK"):
		return "정상"
	default:
		return "확인불가"
	}
}

// SmartDetail returns the full `smartctl -a` report for a disk, for a
// detail screen the user can drill into from the disk list.
func SmartDetail(dev string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	out, err := exec.CommandContext(ctx, "smartctl", "-a", dev).CombinedOutput()
	text := strings.TrimSpace(string(out))
	if text == "" && err != nil {
		return "", fmt.Errorf("smartctl 실행 실패 (smartmontools 설치 여부를 확인하세요): %w", err)
	}
	return text, nil
}
