// Package platform은 Gorilla WebSocket을 Spine의 공개 WS 계약에 연결합니다.
package platform

import (
	"fmt"
	"net/http"
	"time"

	"github.com/NARUBROWN/spine"
	wsadapter "github.com/NARUBROWN/spine/contract/websocket"
	"github.com/gorilla/websocket"
)

// Option은 HTTP 플랫폼과 독립적으로 조합할 수 있는 Spine 생성 옵션입니다.
func Option() spine.Option { return spine.WithWebSocketAdapter(NewFactory()) }

type factory struct{}

var _ wsadapter.Factory = factory{}
var _ wsadapter.Adapter = (*adapter)(nil)
var _ wsadapter.Connection = (*connection)(nil)

// NewFactory는 리스너나 연결을 열지 않는 Gorilla 어댑터 factory를 반환합니다.
func NewFactory() wsadapter.Factory { return factory{} }

func (factory) Capabilities() wsadapter.Capabilities {
	return wsadapter.Capabilities{ContractVersion: wsadapter.ContractVersion}
}

func (factory) New(options wsadapter.Options) (wsadapter.Adapter, error) {
	if err := options.Validate(); err != nil {
		return nil, err
	}
	return &adapter{options: options}, nil
}

type adapter struct{ options wsadapter.Options }

func (a *adapter) Upgrade(w http.ResponseWriter, req *http.Request) (wsadapter.Connection, error) {
	if err := req.Context().Err(); err != nil {
		http.Error(w, http.StatusText(http.StatusServiceUnavailable), http.StatusServiceUnavailable)
		return nil, err
	}
	u := websocket.Upgrader{
		HandshakeTimeout: a.options.HandshakeTimeout,
		// 출처 정책은 커널이 인증과 업그레이드 전에 적용합니다.
		CheckOrigin: func(*http.Request) bool { return true },
	}
	conn, err := u.Upgrade(w, req, nil)
	if err != nil {
		return nil, err
	}
	c := &connection{conn: conn, readTimeout: a.options.ReadTimeout}
	conn.SetReadLimit(a.options.MaxMessageBytes)
	if err := c.refreshReadDeadline(); err != nil {
		_ = conn.Close()
		return nil, err
	}
	conn.SetPongHandler(func(string) error { return c.refreshReadDeadline() })
	return c, nil
}

type connection struct {
	conn        *websocket.Conn
	readTimeout time.Duration
}

func (c *connection) refreshReadDeadline() error {
	return c.conn.SetReadDeadline(time.Now().Add(c.readTimeout))
}

func (c *connection) ReadMessage() (int, []byte, error) {
	// 핸들러 실행 시간은 다음 메시지의 읽기 대기 시간에서 제외합니다.
	if err := c.refreshReadDeadline(); err != nil {
		return 0, nil, err
	}
	return c.conn.ReadMessage()
}

func (c *connection) WriteMessage(kind int, payload []byte, deadline time.Time) error {
	if err := c.conn.SetWriteDeadline(deadline); err != nil {
		return err
	}
	return c.conn.WriteMessage(kind, payload)
}

func (c *connection) Ping(deadline time.Time) error {
	return c.conn.WriteControl(websocket.PingMessage, nil, deadline)
}

func (c *connection) WriteClose(code int, reason string, deadline time.Time) error {
	if len(reason) > 123 {
		return fmt.Errorf("close reason exceeds 123 bytes")
	}
	return c.conn.WriteControl(websocket.CloseMessage, websocket.FormatCloseMessage(code, reason), deadline)
}

func (c *connection) Close() error { return c.conn.Close() }
