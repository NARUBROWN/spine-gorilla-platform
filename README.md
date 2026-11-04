# spine-gorilla-platform

> 이 작업 소스의 대상 버전은 `v0.2.0`이며 Preview, RC, 정식 채널로 발행합니다. [릴리스 운영](RELEASING.md)을 확인하세요. 기존 정식 `v0.1.0`은 변경하지 않습니다.

Gorilla WebSocket을 Spine의 `spine.websocket-adapter/v1` 계약에 연결하는 독립 Go 모듈입니다. 모듈 경로는 `github.com/NARUBROWN/spine-gorilla-platform`입니다. Echo를 import하지 않으며 HTTP 플랫폼과 함께 선택합니다.

```go
app := spine.New(echoPlatform.Option(), gorillaPlatform.Option())
```

`gorillaPlatform.Option()`은 `spine.WithWebSocketAdapter(gorillaPlatform.NewFactory())`와 같습니다. 생성자·핸들러 등록, `boot.HTTPOptions.WebSocket`, `ws.Send` 사용법은 유지합니다. HTTP만 사용하는 앱은 이 모듈이 필요하지 않습니다.

공개 계약은 `github.com/NARUBROWN/spine/contract/websocket`입니다. 이 구현은 WS 선택 API가 포함된 Spine `v0.6.5` 이상과 Gorilla WebSocket `v1.5.3`을 사용합니다. 최소 Go 버전은 1.25.5입니다. 로컬 `replace` 없이 공개 버전으로 설치·검증합니다. Spine v0.6.5 정식 태그와 함께 검증한 릴리스입니다.

읽기 크기·시간 제한과 pong 처리는 이 모듈이 담당하고, 출처·인증·연결 수 제한·ping 주기·종료 순서는 커널이 관리합니다. `NewFactory().New(...)`로 얻은 어댑터를 직접 HTTP 경로에 연결하면 커널의 정책을 거치지 않습니다.

```sh
go test -race ./... -count=1
go vet ./...
```

계약과 종료·동시성 규칙은 [Spine WS 계약 문서](https://github.com/NARUBROWN/spine/blob/v0.6.5/contract/websocket/README.md)를 따릅니다. 텍스트·바이너리, ping/pong, 유휴 시간 제한, 핸들러 실행 시간과 읽기 타임아웃의 분리, 메시지 크기 제한, close 프레임과 동시 Close를 실제 연결로 검사합니다.

## v0.1.0 호환 버전

이 릴리스는 공개 계약을 포함한 Spine `v0.6.5` 이상을 요구합니다. 개인 경로의 `replace` 없이 정식 태그로 설치합니다.

```sh
go get github.com/NARUBROWN/spine-gorilla-platform@v0.1.0
```
