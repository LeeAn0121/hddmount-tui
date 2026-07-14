# hddmount-tui

Ubuntu(18.04 / 20.04 / 22.04)에서 1TB/2TB/4TB/8TB HDD를 안전하게 마운트하기 위한
Go + [Bubbletea](https://github.com/charmbracelet/bubbletea) 기반 TUI 도구입니다.

정적 바이너리(CGO_ENABLED=0)로 빌드되므로 **사용자 서버에 Go/Python 등 별도 런타임이
전혀 필요 없습니다.** `apt install hddmount-tui` 한 줄이면 끝입니다.

## 주요 기능

- 시스템 루트 디스크를 자동으로 제외하고, 나머지 디스크를 1TB/2TB/4TB/8TB로 자동 분류
- 기존 파티션이 있으면 절대 자동으로 밀지 않고, "그대로 마운트" vs "전체 재포맷"을 직접 선택
- 포맷처럼 데이터가 삭제되는 작업은 장치/파티션 경로를 다시 타이핑해야 진행되는 이중 확인
- `/`, `/etc`, `/boot`, `/var`, `/home` 등 시스템 핵심 경로는 마운트 포인트로 지정 불가
- fstab 등록 시 자동 백업 + `mount -a --fake` 문법 검증 + 실패 시 자동 롤백

## 사용자용 설치 (apt 저장소, 배포 완료 후)

```bash
curl -fsSL https://<github-id>.github.io/hddmount-tui/pubkey.gpg \
  | sudo gpg --dearmor -o /usr/share/keyrings/hddmount-tui.gpg

echo "deb [signed-by=/usr/share/keyrings/hddmount-tui.gpg] \
https://<github-id>.github.io/hddmount-tui stable main" \
  | sudo tee /etc/apt/sources.list.d/hddmount-tui.list

sudo apt update
sudo apt install hddmount-tui

sudo hddmount
```

이후 새 버전이 나오면 평소처럼 `sudo apt update && sudo apt upgrade` 로 갱신됩니다.

`<github-id>`는 실제 GitHub 사용자/조직명으로, 저장소 이름은 `hddmount-tui`(또는 원하는
이름)로 바꿔서 사용하세요. `.goreleaser.yaml`의 `release.github.owner/name`과
`.github/workflows/apt-repo.yml`의 리포지토리 설정도 함께 맞춰야 합니다.

## 로컬 개발 빌드

```bash
git clone https://github.com/<github-id>/hddmount-tui.git
cd hddmount-tui
go mod tidy        # go.sum을 완성합니다 (최초 1회)
go build -o hddmount ./cmd/hddmount
sudo ./hddmount
```

## 프로젝트 구조

```
cmd/hddmount/         진입점 (root 권한 체크 후 TUI 실행)
internal/diskutil/    lsblk/parted/mkfs.ext4/mount/fstab 등 시스템 조작 로직
internal/ui/          Bubbletea 화면(디스크 목록 → 파티션 선택 → 확인 → 마운트 → 요약)
.goreleaser.yaml      amd64/arm64 빌드 + .deb 패키징 설정
.github/workflows/    release.yml(빌드·릴리스) + apt-repo.yml(APT 저장소 발행)
```

## 릴리스 만들기

```bash
git tag v0.1.0
git push origin v0.1.0
```

태그를 푸시하면 `release.yml`이 goreleaser로 `linux/amd64`, `linux/arm64` 바이너리와
`.deb` 패키지를 빌드해 GitHub Release에 올리고, 이어서 `apt-repo.yml`이 그 `.deb`를
받아 `gh-pages` 브랜치에 실제 APT 저장소(Packages/Release/서명)를 갱신합니다.

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
`https://<github-id>.github.io/hddmount-tui` 주소가 열립니다(워크플로우가 API로
자동 설정을 시도하지만, 최초 1회는 직접 확인하는 것을 권장합니다).

## 지원 범위

- OS: Ubuntu 18.04 / 20.04 / 22.04 (정적 바이너리라 glibc 버전 이슈 없음)
- 아키텍처: amd64, arm64
- 파일시스템: ext4 (포맷 시 고정)
