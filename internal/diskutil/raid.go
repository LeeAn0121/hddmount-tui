package diskutil

import (
	"fmt"
	"os"
	"os/exec"
)

// CreateRaidArray runs `mdadm --create` on the given whole-disk devices,
// producing /dev/md/<name>. Devices must already have their old
// signatures wiped (see WipeSignatures) or mdadm will interactively ask
// to confirm, which would hang a non-interactive run.
func CreateRaidArray(level int, name string, devices []string) (string, error) {
	args := []string{
		"--create", "/dev/md/" + name,
		"--run", // don't prompt even if a device looks like it belongs to another array
		fmt.Sprintf("--level=%d", level),
		fmt.Sprintf("--raid-devices=%d", len(devices)),
	}
	args = append(args, devices...)
	return runCmd("mdadm", args...)
}

// RaidDevicePath is the stable device path mdadm creates for a named array.
func RaidDevicePath(name string) string { return "/dev/md/" + name }

// PersistRaidConfig appends this machine's current array layout to
// /etc/mdadm/mdadm.conf so the kernel can reassemble the array on the next
// boot instead of leaving it inactive.
func PersistRaidConfig() (string, error) {
	scan, err := exec.Command("mdadm", "--detail", "--scan").Output()
	if err != nil {
		return "", fmt.Errorf("mdadm --detail --scan 실패: %w", err)
	}
	if len(scan) == 0 {
		return "", fmt.Errorf("mdadm --detail --scan 결과가 비어있습니다")
	}

	const confPath = "/etc/mdadm/mdadm.conf"
	f, err := os.OpenFile(confPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return "", fmt.Errorf("%s 쓰기 실패: %w", confPath, err)
	}
	defer f.Close()
	if _, err := f.Write(scan); err != nil {
		return "", fmt.Errorf("%s 쓰기 실패: %w", confPath, err)
	}
	return "mdadm.conf 에 배열 정보 추가 완료", nil
}

// UpdateInitramfs regenerates the initramfs so the RAID array is
// assembled early enough at boot for fstab to find it.
func UpdateInitramfs() (string, error) {
	return runCmd("update-initramfs", "-u")
}
