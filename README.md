# GitHub Meta

GitHub에서 제공하는 Meta 정보를 기반으로 방화벽 등에서 허용할 IP 대역을 설정할 수 있도록 도와주는 도구입니다.
해당 도구는 Go 언어로 작성되었으며, GitHub의 Meta API를 호출하여 필요한 정보를 가져옵니다.

## 다운로드 (Download)

가장 쉬운 방법은 **[GitHub Releases](../../releases)** 페이지에서 자신의 운영체제에 맞는 최신 버전을 다운로드하는 것입니다.
실행 파일은 Linux, Windows, macOS에 대해 `amd64` 및 `arm64` 아키텍처로 각각 제공됩니다.

## 직접 빌드 및 실행 (Build & Run from Source)

Go 언어 환경이 구성되어 있다면 소스 코드를 직접 빌드하거나 실행할 수 있습니다.

```bash
# 소스 코드를 직접 실행
go run main.go [options]

# 또는 프로젝트를 빌드하여 실행 파일을 생성
go build
./github-meta [options]
```

## 사용법 (Options)

| 옵션 | 설명 | 기본값 |
| --- | --- | --- |
| `--output <dir>` | 생성된 파일을 저장할 디렉토리입니다. `json` 백엔드의 경우 `github_meta.json` 파일이, 다른 백엔드의 경우 `<type>_<port>.txt` 형식의 파일이 생성됩니다. 지정하지 않으면 표준 출력으로 표시됩니다. | "" |
| `--type <types>` | 가져올 IP 유형을 쉼표로 구분하여 지정합니다. 포트를 지정하지 않으면 기본 포트(`git`/`ssh_keys`: 22, 기타: 443)가 사용됩니다. (예: `actions,git:22,git:443`) | 모든 유형 |
| `--all` | 모든 IP 유형을 기본 포트로 가져옵니다. `--type`이 지정되지 않은 경우 기본 동작입니다. | false |
| `--backend <format>` | 출력 형식을 지정합니다. (지원 형식: `iptables`, `netsh`, `ufw`, `json`) | "" (일반 텍스트) |
| `--list-types` | GitHub API에서 가져올 수 있는 모든 IP 유형과 기본 포트를 나열하고 종료합니다. | |
| `--version` | 도구의 버전 정보를 표시합니다. | |
| `--help` | 도움말 메시지를 표시합니다. | |

## 지원되는 백엔드 (Supported Backends)

- `iptables`: `iptables` 방화벽 규칙을 생성합니다.
- `netsh`: Windows `netsh advfirewall` 규칙을 생성합니다.
- `ufw`: `ufw` (Uncomplicated Firewall) 규칙을 생성합니다.
- `json`: 모든 IP 정보를 `{"<type>:<port>": ["ip", ...]}` 형식의 구조화된 단일 JSON 파일(`github_meta.json`)로 저장합니다.

## 사용 예시 (Examples)

1.  **사용 가능한 모든 IP 유형 확인:**
    ```bash
    ./github-meta --list-types
    ```

2.  **`actions` IP 대역을 `ufw` 규칙으로 변환하여 터미널에 바로 출력:**

    ```bash
    ./github-meta --type actions --backend ufw
    ```

3.  **`git` 유형에 대해 SSH(22)와 HTTPS(443) IP 대역을 모두 가져와 `./github_ips` 디렉토리에 `json` 파일로 저장:**

    ```bash
    ./github-meta --type git:22,git:443 --backend json --output ./github_ips
    ```
    결과: `./github_ips/github_meta.json` 파일이 생성됩니다.

4.  **모든 IP 대역을 가져와 현재 디렉토리에 `iptables` 규칙 파일들로 저장:**

    ```bash
    ./github-meta --backend iptables --output .
    ```
    결과: `actions_443.txt`, `git_22.txt`, `hooks_443.txt` 등 다수의 파일이 생성됩니다.

## CI/CD 및 릴리즈 (CI/CD & Release)

이 프로젝트는 GitHub Actions를 통해 CI/CD 파이프라인이 구축되어 있습니다. 주요 기능은 다음과 같습니다:

- **자동 빌드 및 테스트**: `main` 브랜치에 코드가 푸시되거나 Pull Request가 생성되면 자동으로 빌드 및 테스트(`go vet`, `go test`)가 수행됩니다.
- **멀티 플랫폼 빌드**: Linux, Windows, macOS 및 `amd64`, `arm64` 아키텍처에 대한 실행 파일이 빌드됩니다.
- **자동 버전 태깅**: `main.go` 파일의 `VERSION` 상수가 변경되고 `main` 브랜치에 푸시되면, 해당 버전으로 Git 태그(`vX.Y.Z`)가 자동으로 생성됩니다.
- **자동 릴리즈 생성**: 새로운 버전 태그가 생성되면, 빌드된 모든 플랫폼의 바이너리를 첨부한 GitHub Release가 자동으로 생성됩니다.
