// Package diskutil wraps the Linux block-device tools (lsblk, blkid, parted,
// mkfs.ext4, wipefs, mount) used to discover, format, and mount HDDs.
package diskutil

import (
	"fmt"
	"os/exec"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// Disk represents one whole block device (e.g. /dev/sdb).
type Disk struct {
	Name  string // "sdb"
	Size  int64  // bytes
	Model string
	Label string // "1TB" / "2TB" / "4TB" / "8TB" / "기타(...)"
}

// Partition represents one partition that belongs to a Disk.
type Partition struct {
	Name       string // "sdb1"
	FSType     string
	MountPoint string
}

func (d Disk) DevPath() string      { return "/dev/" + d.Name }
func (p Partition) DevPath() string { return "/dev/" + p.Name }
func (p Partition) HasFS() bool     { return p.FSType != "" }
func (p Partition) IsMounted() bool { return p.MountPoint != "" }

var kvRe = regexp.MustCompile(`(\S+?)="([^"]*)"`)

// parseKV parses one line of `lsblk ... -P` output (KEY="value" KEY="value" ...)
// into a map. This is the Go equivalent of the `eval "$line"` trick used in the
// bash version of this tool, but without shell injection risk.
func parseKV(line string) map[string]string {
	m := map[string]string{}
	for _, match := range kvRe.FindAllStringSubmatch(line, -1) {
		m[match[1]] = match[2]
	}
	return m
}

// ClassifySize buckets a raw byte size into the manufacturer-advertised
// capacity (decimal GB/TB), allowing for the usual reporting slack.
func ClassifySize(bytes int64) string {
	gb := bytes / 1_000_000_000
	switch {
	case gb >= 900 && gb <= 1100:
		return "1TB"
	case gb >= 1800 && gb <= 2200:
		return "2TB"
	case gb >= 3600 && gb <= 4400:
		return "4TB"
	case gb >= 7200 && gb <= 8800:
		return "8TB"
	default:
		return fmt.Sprintf("기타(%dGB)", gb)
	}
}

// RootDiskName returns the kernel device name (e.g. "sda") backing the
// currently mounted root filesystem, so it can be excluded from the list of
// mountable disks.
func RootDiskName() (string, error) {
	out, err := exec.Command("findmnt", "-n", "-o", "SOURCE", "/").Output()
	if err != nil {
		return "", fmt.Errorf("루트 파일시스템 소스 확인 실패: %w", err)
	}
	src := strings.TrimSpace(string(out))
	if src == "" {
		return "", fmt.Errorf("루트 파일시스템 소스를 확인할 수 없습니다")
	}

	out, err = exec.Command("lsblk", "-no", "PKNAME", src).Output()
	if err == nil {
		if name := strings.TrimSpace(string(out)); name != "" {
			return name, nil
		}
	}
	// PKNAME이 비어있다면 src 자체가 디스크(파티션이 아닌 경우)
	parts := strings.Split(src, "/")
	return parts[len(parts)-1], nil
}

// ListDisks returns every whole-disk block device except the root disk.
func ListDisks() ([]Disk, error) {
	rootDisk, _ := RootDiskName() // 실패해도 목록 조회 자체는 계속 진행

	// NOTE: -r (raw) and -P (pairs) are mutually exclusive on newer util-linux
	// (2.38+); -P alone already produces one safely-quoted line per device.
	out, err := exec.Command("lsblk", "-dbn", "-o", "NAME,SIZE,TYPE,MODEL", "-P").Output()
	if err != nil {
		return nil, fmt.Errorf("lsblk 실행 실패: %w", err)
	}

	var disks []Disk
	for _, line := range strings.Split(string(out), "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		kv := parseKV(line)
		if kv["TYPE"] != "disk" {
			continue
		}
		if rootDisk != "" && kv["NAME"] == rootDisk {
			continue
		}
		size, _ := strconv.ParseInt(kv["SIZE"], 10, 64)
		model := kv["MODEL"]
		if model == "" {
			model = "알수없음"
		}
		disks = append(disks, Disk{
			Name:  kv["NAME"],
			Size:  size,
			Model: model,
			Label: ClassifySize(size),
		})
	}
	sort.Slice(disks, func(i, j int) bool { return disks[i].Name < disks[j].Name })
	return disks, nil
}

// GetPartitions returns the partitions belonging to the given disk name
// ("sdb", not "/dev/sdb").
func GetPartitions(diskName string) ([]Partition, error) {
	dev := "/dev/" + diskName
	out, err := exec.Command("lsblk", "-n", "-o", "NAME,TYPE,FSTYPE,MOUNTPOINT", "-P", dev).Output()
	if err != nil {
		return nil, fmt.Errorf("lsblk 실행 실패: %w", err)
	}
	var parts []Partition
	for _, line := range strings.Split(string(out), "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		kv := parseKV(line)
		if kv["TYPE"] != "part" {
			continue
		}
		parts = append(parts, Partition{
			Name:       kv["NAME"],
			FSType:     kv["FSTYPE"],
			MountPoint: kv["MOUNTPOINT"],
		})
	}
	return parts, nil
}

// DangerousMountpoints lists top-level system paths that must never be used
// as a mount point, to prevent a mis-click from burying /etc, /boot, etc.
// under a freshly mounted (and likely empty) disk.
var DangerousMountpoints = []string{
	"/", "/bin", "/boot", "/dev", "/etc", "/lib", "/lib32", "/lib64",
	"/libx32", "/proc", "/root", "/run", "/sbin", "/srv", "/sys",
	"/usr", "/var", "/home", "/opt", "/media", "/tmp",
}

// IsDangerousMountpoint reports whether path is a protected system directory.
func IsDangerousMountpoint(path string) bool {
	for _, d := range DangerousMountpoints {
		if path == d {
			return true
		}
	}
	return false
}
