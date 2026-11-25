package main

import (
	"flag"
	"fmt"

	"github.com/koolsign/hddmount-tui/internal/diskutil"
)

// cliPrepareUploads는 시스템 디스크 폴더 모드를 명시적으로 실행한다.
// args는 빈 인자 또는 --help다. 성공 0, 경로·권한·링크 오류 시 1을 반환한다.
// 포맷·마운트·fstab 변경 없이 새 폴더와 웹 링크를 준비하고 작업을 기록한다.
func cliPrepareUploads(args []string) int {
	flags := flag.NewFlagSet("prepare-uploads", flag.ExitOnError)
	flags.Parse(args)

	if flags.NArg() != 0 {
		return fail("prepare-uploads에는 추가 인자가 없습니다")
	}

	output, err := diskutil.PrepareSystemUploads()
	diskutil.LogEvent("cli-prepare-system-uploads", "/data/uploads", output, err)

	if output != "" {
		fmt.Println(output)
	}

	if err != nil {
		return fail("시스템 디스크 폴더 준비 실패: %v", err)
	}

	return 0
}
