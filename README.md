# hddmount-tui

Ubuntu(18.04 / 20.04 / 22.04)에서 1TB/2TB/4TB/8TB HDD를 안전하게 마운트하기 위한
Go + [Bubbletea](https://github.com/charmbracelet/bubbletea) 기반 TUI 도구입니다.
인터랙티브 TUI뿐 아니라 스크립트/자동화에 바로 꽂을 수 있는 CLI 서브커맨드도 제공합니다.

정적 바이너리(CGO_ENABLED=0)로 빌드되므로 **사용자 서버에 Go/Python 등 별도 런타임이
전혀 필요 없습니다.** `apt install hddmount-tui` 한 줄이면 끝입니다.

프로젝트 홈페이지: https://LeeAn0121.github.io/hddmount-tui

## 주요 기능

- 시스템 루트 디스크를 자동으로 제외하고, 나머지 디스크를 1TB/2TB/4TB/8TB로 자동 분류
- 기존 파티션이 있으면 절대 자동으로 밀지 않고, "그대로 마운트" vs "전체 재포맷"을 직접 선택
- 포맷처럼 데이터가 삭제되는 작업은 장치/파티션 경로를 다시 타이핑해야 진행되는 이중 확인
- `/` 자체와 `/etc`, `/boot` 등 시스템 핵심 경로는 마운트 포인트로 지정 불가
- `/data`처럼 운영용으로 나눈 최상위 데이터 경로는 마운트 포인트로 지정 가능
- fstab 등록 시 자동 백업 + `mount -a --fake` 문법 검증 + 실패 시 자동 롤백
- 디스크 목록에서 SMART 상태를 바로 확인(`s`), 상세 리포트(`smartctl -a`) 조회 가능
- 마운트된 파티션을 마운트 위저드 없이 바로 해제(`u`)
- 마운트 시 마운트 포인트 이름을 딴 ext4 라벨 자동 설정
- `/var/log/hddmount.log` 에 모든 마운트/언마운트/포맷 작업 기록 (운영 감사용)
- 동일 용량 디스크 여러 개로 mdadm RAID1 / RAID10 배열 구성(`R`)
- 자동화용 비대화형 CLI 서브커맨드 (`list`/`parts`/`smart`/`mount`/`unmount`/`format-disk`)

## 사용자용 설치 (apt 저장소)

```bash
curl -fsSL https://LeeAn0121.github.io/hddmount-tui/pubkey.gpg \
  | sudo gpg --dearmor -o /usr/share/keyrings/hddmount-tui.gpg

echo "deb [signed-by=/usr/share/keyrings/hddmount-tui.gpg] \
https://LeeAn0121.github.io/hddmount-tui stable main" \
  | sudo tee /etc/apt/sources.list.d/hddmount-tui.list

sudo apt update
sudo apt install hddmount-tui

sudo hddmount
```

이후 새 버전이 나오면 평소처럼 `sudo apt update && sudo apt upgrade` 로 갱신됩니다.

이 저장소를 포크해서 자신의 계정으로 배포하려면 위 주소의 `LeeAn0121`을 자신의
GitHub 사용자/조직명으로 바꾸면 됩니다(저장소 이름도 원하는 대로 변경 가능). `README.md`,
`docs/index.html`의 URL도 함께 바꿔주세요.

## TUI 사용법

`sudo hddmount` 로 실행하면 마법사가 뜹니다.

| 화면 | 키 |
|---|---|
| 디스크 목록 | `↑/↓` 이동, `enter` 선택, `s` SMART 상세, `R` RAID 구성, `q` 종료 |
| 파티션 선택 | `↑/↓` 이동, `enter` 선택(마운트/포맷), `u` 마운트 해제, `b` 뒤로 |
| 확인/경고 화면 | `y`/`n` 또는 `←/→` + `enter`, 파괴적 작업은 장치 경로 재입력으로 이중 확인 |
| 완료 화면 | `r` 다른 디스크 계속 마운트, `q` 종료 |

RAID 구성은 디스크 목록에서 현재 커서가 있는 디스크와 같은 용량 라벨(예: 4TB)을 가진
디스크들을 후보로 묶어 보여줍니다. 디스크 2개 이상이면 RAID1, 4개 이상(짝수)이면 RAID10도
선택할 수 있습니다. 배열 생성 후에는 일반 마운트 흐름(마운트 포인트 입력 → fstab 등록 여부)
을 그대로 이어서 사용합니다.

## 비대화형 CLI (자동화)

인자를 주고 실행하면 TUI 없이 바로 동작합니다. 프로비저닝 스크립트, cron, config
management 도구 등에서 사용하세요.

```bash
# 디스크 목록 (JSON도 지원)
sudo hddmount list
sudo hddmount list --json

# 특정 디스크의 파티션 목록
sudo hddmount parts --disk sdb --json

# SMART 상태
sudo hddmount smart --disk sdb --json

# 기존 파티션 마운트 (필요하면 --format 으로 ext4 포맷부터)
sudo hddmount mount --partition sdb1 --mountpoint /data/hdd1 --format --fstab

# 마운트 해제
sudo hddmount unmount --partition sdb1
sudo hddmount unmount --mountpoint /data/hdd1

# 디스크 전체 초기화 + 포맷 (+선택적으로 마운트까지)
# --confirm 에 디스크 이름을 다시 입력해야 실제로 실행됩니다.
sudo hddmount format-disk --disk sdb --confirm sdb \
  --mountpoint /data/hdd1 --fstab

# 전체 사용법
hddmount --help
```

모든 CLI 동작은 TUI와 동일하게 `/var/log/hddmount.log` 에 기록됩니다.

## 로컬 개발 빌드

```bash
git clone https://github.com/LeeAn0121/hddmount-tui.git
cd hddmount-tui
go mod tidy        # go.sum을 완성합니다 (최초 1회)
go build -o hddmount ./cmd/hddmount
sudo ./hddmount
```

## 프로젝트 구조

```
cmd/hddmount/         진입점 (root 권한 체크, CLI 서브커맨드 분기, TUI 실행)
internal/diskutil/    lsblk/parted/mkfs.ext4/mount/fstab/smartctl/mdadm 등 시스템 조작 로직
internal/ui/          Bubbletea 화면(디스크 목록 → 파티션 선택 → 확인 → 마운트 → 요약, SMART, RAID)
docs/                 GitHub Pages 랜딩 페이지 소스 (gh-pages 배포 시 자동 반영)
.goreleaser.yaml      amd64/arm64 빌드 + .deb 패키징 설정
.github/workflows/    release.yml(빌드·릴리스) + apt-repo.yml(APT 저장소 + Pages 발행)
```

## 릴리스 만들기

버전은 [Semantic Versioning](https://semver.org/)을 따릅니다: `MAJOR.MINOR.PATCH`
(하위 호환 깨짐 / 기능 추가 / 버그 수정). 정식 배포 전 테스트 태그가 필요하면
`v1.0.0-alpha`, `-beta`, `-rc`, `-dev` 접미사를 사용하세요.

```bash
git tag v0.4.0
git push origin v0.4.0
```

태그를 푸시하면 `release.yml`이 goreleaser로 `linux/amd64`, `linux/arm64` 바이너리와
`.deb` 패키지를 빌드해 GitHub Release에 올리고, 이어서 `apt-repo.yml`이 그 `.deb`를
받아 `gh-pages` 브랜치에 실제 APT 저장소(Packages/Release/서명)와 랜딩 페이지를 갱신합니다.

## APT 저장소 서명 키 최초 설정 (1회만)

```bash
gpg --full-generate-key            # RSA 4096, 만료 없음(또는 원하는 기간) 권장
gpg --list-secret-keys --keyid-format=long   # 여기서 keyID 확인

gpg --export-secret-keys --armor <KEYID> > private.asc
gpg --export --armor <KEYID> > public.asc
```

GitHub 저장소 Settings → Secrets and variables → Actions 에 아래 3개를 등록하세요.

| Secret 이름 | 값 |
|---|---|
| `GPG_PRIVATE_KEY` | `private.asc` 파일 내용 전체 |
| `GPG_PASSPHRASE` | 위 키 생성 시 입력한 암호 |
| `GPG_KEY_ID` | `<KEYID>` |

등록 후 `private.asc` 파일은 로컬에서 안전하게 삭제(또는 별도 보관)하세요.

또한 저장소 Settings → Pages 에서 소스를 `gh-pages` 브랜치 `/` 로 지정해야
`https://LeeAn0121.github.io/hddmount-tui` 주소가 열립니다(워크플로우가 API로
자동 설정을 시도하지만, 최초 1회는 직접 확인하는 것을 권장합니다).

## 지원 범위

- OS: Ubuntu 18.04 / 20.04 / 22.04 (정적 바이너리라 glibc 버전 이슈 없음)
- 아키텍처: amd64, arm64
- 파일시스템: ext4 (포맷 시 고정)
- RAID: mdadm RAID1 / RAID10 (동일 용량 디스크 필요)
