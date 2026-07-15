// Non-interactive CLI mode, for scripting/automation. Selected when the
// first argument matches one of the known subcommands below; otherwise
// main() falls through to the interactive TUI.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path"
	"strings"

	"github.com/koolsign/hddmount-tui/internal/diskutil"
)

var cliSubcommands = map[string]bool{
	"list": true, "parts": true, "mount": true, "unmount": true,
	"smart": true, "format-disk": true,
}

func printCLIUsage() {
	fmt.Fprintln(os.Stderr, `hddmount <subcommand> [flags]  -- 비대화형(자동화) 모드

  list                                디스크 목록 출력
    [--json]

  parts --disk sdb                    디스크의 파티션 목록 출력
    [--json]

  smart --disk sdb                    SMART 상태 조회
    [--json]

  mount --partition sdb1 --mountpoint /data/hdd1
    [--format]     파일시스템이 없으면 먼저 ext4로 포맷
    [--fstab]      /etc/fstab 에 등록 (재부팅 시 자동 마운트)

  unmount --partition sdb1 | --mountpoint /data/hdd1

  format-disk --disk sdb --confirm sdb
    전체 디스크를 새 GPT + ext4로 초기화합니다 (데이터 전부 삭제).
    --confirm 에 디스크 이름(예: sdb)을 정확히 다시 넣어야 실행됩니다.
    [--mountpoint /data/hdd1] [--fstab]

인터랙티브 TUI를 쓰려면 인자 없이 hddmount 를 실행하세요.`)
}

func runCLI(sub string, args []string) int {
	switch sub {
	case "list":
		return cliList(args)
	case "parts":
		return cliParts(args)
	case "smart":
		return cliSmart(args)
	case "mount":
		return cliMount(args)
	case "unmount":
		return cliUnmount(args)
	case "format-disk":
		return cliFormatDisk(args)
	}
	printCLIUsage()
	return 2
}

func fail(format string, a ...any) int {
	fmt.Fprintf(os.Stderr, "오류: "+format+"\n", a...)
	return 1
}

func printJSON(v any) int {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(v); err != nil {
		return fail("%v", err)
	}
	return 0
}

func cliList(args []string) int {
	fs := flag.NewFlagSet("list", flag.ExitOnError)
	jsonOut := fs.Bool("json", false, "JSON으로 출력")
	fs.Parse(args)

	disks, err := diskutil.ListDisks()
	if err != nil {
		return fail("%v", err)
	}
	if *jsonOut {
		return printJSON(disks)
	}
	for _, d := range disks {
		fmt.Printf("%-10s  %-6s  %-30s  SMART=%s\n", d.DevPath(), d.Label, d.Model, d.SmartState)
	}
	return 0
}

func cliParts(args []string) int {
	fs := flag.NewFlagSet("parts", flag.ExitOnError)
	disk := fs.String("disk", "", "디스크 이름 (예: sdb)")
	jsonOut := fs.Bool("json", false, "JSON으로 출력")
	fs.Parse(args)

	if *disk == "" {
		return fail("--disk 를 지정하세요")
	}
	parts, err := diskutil.GetPartitions(strings.TrimPrefix(*disk, "/dev/"))
	if err != nil {
		return fail("%v", err)
	}
	if *jsonOut {
		return printJSON(parts)
	}
	for _, p := range parts {
		fmt.Printf("%-12s  FSTYPE=%-8s  MOUNTPOINT=%s\n", p.DevPath(), p.FSType, p.MountPoint)
	}
	return 0
}

func cliSmart(args []string) int {
	fs := flag.NewFlagSet("smart", flag.ExitOnError)
	disk := fs.String("disk", "", "디스크 이름 (예: sdb)")
	jsonOut := fs.Bool("json", false, "JSON으로 출력")
	fs.Parse(args)

	if *disk == "" {
		return fail("--disk 를 지정하세요")
	}
	dev := "/dev/" + strings.TrimPrefix(*disk, "/dev/")
	out, err := diskutil.SmartDetail(dev)
	if err != nil {
		return fail("%v", err)
	}
	if *jsonOut {
		return printJSON(struct {
			Device string `json:"device"`
			Health string `json:"health"`
			Detail string `json:"detail"`
		}{Device: dev, Health: diskutil.SmartHealth(dev), Detail: out})
	}
	fmt.Println(out)
	return 0
}

func cliMount(args []string) int {
	fs := flag.NewFlagSet("mount", flag.ExitOnError)
	partition := fs.String("partition", "", "파티션 이름 또는 경로 (예: sdb1 또는 /dev/sdb1)")
	mountPoint := fs.String("mountpoint", "", "마운트할 경로 (예: /data/hdd1)")
	format := fs.Bool("format", false, "파일시스템이 없으면 ext4로 포맷")
	fstab := fs.Bool("fstab", false, "/etc/fstab 에 등록")
	fs.Parse(args)

	mp := path.Clean(*mountPoint)
	if *partition == "" || *mountPoint == "" {
		return fail("--partition 과 --mountpoint 를 모두 지정하세요")
	}
	if !strings.HasPrefix(mp, "/") {
		return fail("마운트 경로는 절대경로여야 합니다")
	}
	if diskutil.IsDangerousMountpoint(mp) {
		return fail("시스템 핵심 경로(%s)는 마운트 포인트로 사용할 수 없습니다", mp)
	}

	part := *partition
	if !strings.HasPrefix(part, "/dev/") {
		part = "/dev/" + part
	}

	if *format {
		out, err := diskutil.FormatExt4(part)
		diskutil.LogEvent("cli-format", part, out, err)
		if err != nil {
			fmt.Println(out)
			return fail("%v", err)
		}
	}

	out, err := diskutil.MountPartition(part, mp)
	diskutil.LogEvent("cli-mount", part+" -> "+mp, out, err)
	if out != "" {
		fmt.Println(out)
	}
	if err != nil {
		return fail("%v", err)
	}
	fmt.Printf("마운트 완료: %s -> %s\n", part, mp)

	out, err = diskutil.PrepareContentTree(mp)
	diskutil.LogEvent("cli-prepare-content-tree", mp, out, err)
	if out != "" {
		fmt.Println(out)
	}
	if err != nil {
		return fail("%v", err)
	}

	if *fstab {
		out, err := diskutil.SetupFstab(part, mp)
		diskutil.LogEvent("cli-fstab", part, out, err)
		if out != "" {
			fmt.Println(out)
		}
		if err != nil {
			return fail("%v", err)
		}
	}
	return 0
}

func cliUnmount(args []string) int {
	fs := flag.NewFlagSet("unmount", flag.ExitOnError)
	partition := fs.String("partition", "", "파티션 이름 또는 경로 (예: sdb1)")
	mountPoint := fs.String("mountpoint", "", "마운트 경로")
	fs.Parse(args)

	target := *mountPoint
	if target == "" && *partition != "" {
		target = *partition
		if !strings.HasPrefix(target, "/dev/") {
			target = "/dev/" + target
		}
	}
	if target == "" {
		return fail("--partition 또는 --mountpoint 를 지정하세요")
	}

	out, err := diskutil.Unmount(target)
	diskutil.LogEvent("cli-unmount", target, out, err)
	if out != "" {
		fmt.Println(out)
	}
	if err != nil {
		return fail("%v", err)
	}
	fmt.Printf("마운트 해제 완료: %s\n", target)
	return 0
}

func cliFormatDisk(args []string) int {
	fs := flag.NewFlagSet("format-disk", flag.ExitOnError)
	disk := fs.String("disk", "", "디스크 이름 (예: sdb)")
	confirm := fs.String("confirm", "", "디스크 이름을 다시 입력해 확인")
	mountPoint := fs.String("mountpoint", "", "포맷 후 마운트할 경로 (선택)")
	fstab := fs.Bool("fstab", false, "/etc/fstab 에 등록 (mountpoint 지정 시)")
	fs.Parse(args)

	name := strings.TrimPrefix(*disk, "/dev/")
	if name == "" {
		return fail("--disk 를 지정하세요")
	}
	if strings.TrimPrefix(*confirm, "/dev/") != name {
		return fail("--confirm 에 디스크 이름(%s)을 정확히 입력해야 진행됩니다", name)
	}

	dev := "/dev/" + name
	if out, err := diskutil.WipeSignatures(dev); err != nil {
		diskutil.LogEvent("cli-format-disk", dev, out, err)
		fmt.Println(out)
		return fail("%v", err)
	}
	out, err := diskutil.CreatePartitionTable(dev)
	diskutil.LogEvent("cli-format-disk", dev, out, err)
	fmt.Println(out)
	if err != nil {
		return fail("%v", err)
	}
	part := diskutil.NewPartitionPath(dev)
	if out, err := diskutil.FormatExt4(part); err != nil {
		diskutil.LogEvent("cli-format-disk", part, out, err)
		fmt.Println(out)
		return fail("%v", err)
	}
	diskutil.LogEvent("cli-format-disk", part, "ext4 포맷 완료", nil)
	fmt.Printf("포맷 완료: %s\n", part)

	if *mountPoint == "" {
		return 0
	}
	mp := path.Clean(*mountPoint)
	if !strings.HasPrefix(mp, "/") {
		return fail("마운트 경로는 절대경로여야 합니다")
	}
	if diskutil.IsDangerousMountpoint(mp) {
		return fail("시스템 핵심 경로(%s)는 마운트 포인트로 사용할 수 없습니다", mp)
	}
	out, err = diskutil.MountPartition(part, mp)
	diskutil.LogEvent("cli-mount", part+" -> "+mp, out, err)
	if out != "" {
		fmt.Println(out)
	}
	if err != nil {
		return fail("%v", err)
	}
	fmt.Printf("마운트 완료: %s -> %s\n", part, mp)

	out, err = diskutil.PrepareContentTree(mp)
	diskutil.LogEvent("cli-prepare-content-tree", mp, out, err)
	if out != "" {
		fmt.Println(out)
	}
	if err != nil {
		return fail("%v", err)
	}

	if *fstab {
		out, err := diskutil.SetupFstab(part, mp)
		diskutil.LogEvent("cli-fstab", part, out, err)
		if out != "" {
			fmt.Println(out)
		}
		if err != nil {
			return fail("%v", err)
		}
	}
	return 0
}
