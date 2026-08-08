# watchdog

미니 PC와 주요 Docker 서비스의 상태를 주기적으로 점검하고,
장애 발생/복구 시 **Discord Webhook**으로 알림을 보내는 경량 모니터링 프로그램 (Go).

Galaxy Wide4 (Termux, ARM64)에서 실행하는 것을 전제로,
외부 프레임워크 없이 표준 라이브러리 위주로 구현했다.

---

## 동작 방식

```
Termux (Galaxy Wide4)
  └─ watchdog (1분 주기)
       ├─ HTTP 체크 ──▶ 미니 PC의 서비스들 (Homepage, Portainer, ...)
       ├─ 상태 판정 (연속 실패 3회 → DOWN, 연속 성공 2회 → UP)
       ├─ 상태 파일 저장 (state.json) — 재시작 후에도 상태 유지
       └─ 상태 변화 시 ──▶ Discord Webhook 알림 (장애 🔴 / 복구 🟢)
```

### 상태 판정 규칙 (히스테리시스)

| 전환 | 조건 | 알림 |
|---|---|---|
| UP → DOWN | **연속 3회 실패** | 🔴 장애 |
| DOWN → UP | **연속 2회 성공** | 🟢 복구 |
| 같은 상태 유지 | — | 없음 (반복 전송 금지) |

> UP 판정에도 연속 성공 횟수를 요구하는 이유: 일시적인 네트워크 깜빡임으로
> UP/DOWN이 왔다 갔다 하면 알림이 과도해진다. 실패 3회 / 복구 2회 값은 `config.yaml`에서 조정 가능.

---

## 프로젝트 구조

```
watchdog/
├── cmd/watchdog/
│   ├── main.go          # 진입점: 설정 로드, 체크 루프, graceful shutdown
│   └── config.go        # YAML 설정 파싱 + 기본값 + 검증
├── internal/
│   ├── checker/         # HTTP 헬스 체크 (timeout, 자체 서명 인증서 지원)
│   ├── notifier/        # Discord Webhook 전송
│   └── state/           # UP/DOWN 판정 + 상태 파일 저장/로드
├── config.example.yaml  # 설정 예시
├── go.mod               # 의존성: yaml.v3 단 하나
└── README.md
```

- **checker**: 각 대상에 `context.WithTimeout`으로 타임아웃을 걸고 HTTP GET.
  DNS 조회까지 타임아웃에 포함된다. 2xx~3xx를 정상으로 판정.
- **state**: 서비스별 상태를 `map[string]*HostState` + `RWMutex`로 관리.
  판정 규칙(연속 실패/성공 임계값)이 모두 여기 있어 단위 테스트가 쉽다.
- **notifier**: webhook URL이 비어 있으면 no-op. 전송 실패는 로그만 남기고
  모니터링 루프를 멈추지 않는다.
- **main**: 모든 대상을 goroutine으로 **병렬 체크** 후 `WaitGroup`으로 수렴,
  상태 반영 → 알림 → 상태 저장 순서로 한 사이클을 처리한다.

---

## 설정

`config.example.yaml`을 `config.yaml`로 복사해 수정한다.

```yaml
check_interval: 60s          # 체크 주기
http_timeout: 10s            # HTTP 요청 타임아웃 (DNS 조회 포함)
failure_threshold: 3         # 연속 실패 3회 → DOWN
recovery_threshold: 2        # DOWN에서 연속 성공 2회 → UP
state_file: state.json       # 상태 유지 파일

targets:
  - name: Homepage           # 서비스 이름 (알림/로그에 표시)
    url: http://192.168.0.10:3001
  - name: Portainer
    url: https://192.168.0.10:9443
    insecure: true           # 자체 서명 인증서를 쓰면 true

discord:
  webhook_url: https://discord.com/api/webhooks/...   # 비워두면 알림 없음
```

기본값: `check_interval=60s`, `http_timeout=10s`, `failure_threshold=3`, `recovery_threshold=2`, `state_file=state.json`

---

## 빌드

### PC (크로스 빌드) — 권장

```bash
# AMD64/Intel PC에서 ARM64(Termux)용 바이너리 빌드
GOOS=linux GOARCH=arm64 CGO_ENABLED=0 go build -o watchdog ./cmd/watchdog

# 전송
scp watchdog user@192.168.0.10:/tmp/
# 또는 Termux에서 직접 다운로드 (GitHub Release 등)
```

### Termux에서 직접 빌드

```bash
pkg install golang
cd watchdog
go build -o watchdog ./cmd/watchdog
```

### 실행

```bash
cp config.example.yaml config.yaml
nano config.yaml   # 실제 서비스 URL / webhook 주소 입력

./watchdog -config config.yaml
```

---

## Termux에서 필요한 명령어

```bash
# 1. Go 설치 (이미 있으면 생략)
pkg update && pkg install golang

# 2. 소스 받기
pkg install git
git clone https://github.com/사용자명/watchdog && cd watchdog

# 3. 설정
cp config.example.yaml config.yaml
nano config.yaml

# 4. 빌드 + 실행
go build -o watchdog ./cmd/watchdog
./watchdog -config config.yaml
```

### 백그라운드 실행 (화면 잠금/앱 종료 대응)

```bash
# Termux 앱이 백그라운드에서 종료되지 않게 유지
termux-wake-lock

# 세션을 닫아도 계속 실행
nohup ./watchdog -config config.yaml > watchdog.log 2>&1 &

# 확인
tail -f watchdog.log

# 종료
kill $(pgrep -f "watchdog -config")
```

> `termux-wake-lock` 없이 실행하면 화면이 꺼지면 Termux가 앱을 종료할 수 있다.
> 와이파이만 유지돼도 HTTP 체크에는 문제없으므로 wake-lock이면 충분하다.
> (배터리 최적화에서 Termux 예외 설정도 해두면 좋다.)

---

## 테스트

```bash
go test ./...
```

| 패키지 | 검증 내용 |
|---|---|
| `internal/state` | 연속 실패/성공 판정, 히스테리시스, 알림 조건, 상태 파일 저장/복원 |
| `internal/checker` | 200/500 응답, 연결 거부, **timeout**, 자체 서명 인증서(insecure) |
| `internal/notifier` | webhook payload, 실패 시 에러, 미설정 no-op |
| `cmd/watchdog` | 설정 파싱, 기본값 적용, 검증 실패 케이스 |

### 로컬에서 직접 시연

```bash
# webhook 수신 확인용 (임시)
python3 -m http.server 19999

# 설정에서 webhook_url을 http://127.0.0.1:19999 로 바꾸고,
# targets 중 하나를 죽은 포트로 지정하면 3주기 후 🔴 DOWN 알림이 로그에 찍힌다.
```

로그 예시:

```
2026/08/08 02:18:06 [START] watchdog 시작 — 대상 2개, 체크 주기 1s, 타임아웃 2s
2026/08/08 02:18:06 [CHECK] up-service: UP (HTTP 200, 12ms)
2026/08/08 02:18:08 [CHECK] down-service: DOWN (connection refused, 1ms)
2026/08/08 02:18:08 [ALERT] down-service:  → DOWN — Discord 알림 전송
2026/08/08 02:18:19 [CHECK] down-service: UP (HTTP 200, 2ms)
2026/08/08 02:18:19 [ALERT] down-service: DOWN → UP — Discord 알림 전송
2026/08/08 02:18:22 [SHUTDOWN] 종료 신호 수신 — 상태 저장 후 종료
```

---

## 다음 단계에서 추가하면 좋은 기능 (2단계 확장)

1. **SSH로 Docker 컨테이너 상태 직접 조회** (예정)
   - `ssh user@host docker ps` 를 정기 실행해 컨테이너별 상태 체크
   - HTTP로 드러나지 않는 컨테이너(내부 포트만 노출)까지 감시 가능
   - `ssh`는 `golang.org/x/crypto/ssh`(순수 Go)로 구현하거나
     Termux에 이미 있는 `ssh` 바이너리를 exec로 호출
2. **HTTP 응답 본문/지연 시간 임계값 체크** — 200이어도 응답이 5초 넘게 걸리면 경고
3. **알림 실패 재시도** — webhook 전송 실패 시 2~3회 재시도 + 실패 로그
4. **체크 이력 로테이션** — 상태 변화 이력을 JSON으로 남겨 일주일치 그래프
5. **여러 Discord 채널 / severity 분리** — 장애와 복구를 다른 채널로
6. **watchdog 자체 헬스** — Termux에서 죽으면 재시작하는 `termux-services` 연동

## 참고

- 의존성은 `gopkg.in/yaml.v3` 단 하나 (YAML 파싱용). 나머지는 전부 Go 표준 라이브러리.
- 상태 파일은 임시 파일 + rename 방식으로 원자적 저장 → 중간에 죽어도 파일이 깨지지 않는다.
