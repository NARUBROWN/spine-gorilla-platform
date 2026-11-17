English | [Korean](README.ko.md)

# spine-gorilla-platform

An independent Go module connecting Gorilla WebSocket to Spine's `spine.websocket-adapter/v1` contract. The module path is `github.com/NARUBROWN/spine-gorilla-platform`. It does not import Echo; select it alongside an HTTP platform.

## Preview installation

Use this channel to try features under development and check compatibility. The published tag is [`v0.2.0-preview.3`](https://github.com/NARUBROWN/spine-gorilla-platform/tree/v0.2.0-preview.3). Its `go.mod` specifies Spine `v0.7.0-preview.4` and Go 1.25.5 or later. The release policy on this branch targets `v0.2.0`.

```sh
go get github.com/NARUBROWN/spine-gorilla-platform@v0.2.0-preview.3
```

Other channels: [RC](https://github.com/NARUBROWN/spine-gorilla-platform/tree/rc) | [Stable](https://github.com/NARUBROWN/spine-gorilla-platform/tree/main).

```go
app := spine.New(echoPlatform.Option(), gorillaPlatform.Option())
```

`gorillaPlatform.Option()` is equivalent to `spine.WithWebSocketAdapter(gorillaPlatform.NewFactory())`. Constructor registration, handler registration, `boot.HTTPOptions.WebSocket`, and `ws.Send` remain unchanged. HTTP-only applications do not need this module.

The public contract is `github.com/NARUBROWN/spine/contract/websocket`. See the v0.1.0 compatibility section below for the Spine v0.6.5 combination.

This module handles read size limits, timeouts, and pong frames. The kernel manages origins, authentication, connection limits, ping intervals, and shutdown order. Connecting an adapter from `NewFactory().New(...)` directly to an HTTP route bypasses kernel policies.

```sh
go test -race ./... -count=1
go vet ./...
```

Follow the [Spine WebSocket contract](https://github.com/NARUBROWN/spine/blob/v0.6.5/contract/websocket/README.md) for contracts, shutdown, and concurrency rules. Tests use real connections to check text, binary, ping/pong, idle timeouts, separation of handler duration from read timeouts, message size limits, close frames, and concurrent Close calls.

## v0.1.0 compatibility

This release requires Spine `v0.6.5` or later, which includes the public contracts. Install the official tag without a `replace` pointing to a private path.

```sh
go get github.com/NARUBROWN/spine-gorilla-platform@v0.1.0
```

## Releases and public contracts

This platform publishes its own versions and release channels. The Spine version in `go.mod` is the minimum dependency for its public contracts and is managed separately from the core release channel. CI and public installation checks verify the platform against this module's dependencies. Core releases verify combinations of a new core and existing platform tags.
