# GitHub Meta API

Github에서 제공하는 Meta 정보를 기반으로 방화벽 등에서 허용할 IP 대역을 설정할 수 있도록 도와주는 도구입니다.
해당 도구는 Go 언어로 작성되었으며, GitHub의 Meta API를 호출하여 필요한 정보를 가져옵니다.

## 설치 및 실행 (Installation & Usage)

Go 언어 환경이 구성되어 있어야 합니다.

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
| `--output <dir>` | 생성된 파일을 저장할 출력 디렉토리입니다. 지정하지 않으면 표준 출력으로 표시됩니다. | "" |
| `--type <types>` | 가져올 IP 유형을 쉼표로 구분하여 지정합니다 (예: `actions,hooks`). | 모든 유형 |
| `--all` | 모든 IP 유형을 가져옵니다. `--type`이 지정되지 않은 경우 기본 동작입니다. | false |
| `--backend <format>` | 출력 형식을 지정합니다. (지원 형식: `iptables`, `netsh`, `ufw`, `json`) | "" (일반 텍스트) |
| `--version` | 도구의 버전 정보를 표시합니다. | |
| `--help` | 도움말 메시지를 표시합니다. | |

## 지원되는 백엔드 (Supported Backends)

- `iptables`: `iptables` 방화벽 규칙을 생성합니다.
- `netsh`: Windows `netsh advfirewall` 규칙을 생성합니다.
- `ufw`: `ufw` (Uncomplicated Firewall) 규칙을 생성합니다.
- `json`: 모든 IP 정보를 구조화된 단일 JSON 파일로 저장합니다.

## 사용 예시 (Examples)

1.  **`actions`와 `hooks` IP 대역을 `ufw` 규칙으로 변환하여 터미널에 바로 출력:**

    ```bash
    go run main.go --type actions,hooks --backend ufw
    ```

2.  **모든 IP 대역을 가져와 `./github_ips` 디렉토리에 단일 JSON 파일로 저장:**

    ```bash
    go run main.go --backend json --output ./github_ips
    ```
    결과: `./github_ips/github_meta.json` 파일이 생성됩니다.

3.  **`git` 유형의 IP만 가져와 현재 디렉토리에 `iptables` 규칙 파일로 저장:**

    ```bash
    go run main.go --type git --backend iptables --output .
    ```
    결과: `git.txt` 파일이 생성됩니다.