package platform_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	platform "github.com/NARUBROWN/spine-gorilla-platform"
	wsadapter "github.com/NARUBROWN/spine/contract/websocket"
	"github.com/gorilla/websocket"
)

func newAdapter(t *testing.T, options wsadapter.Options) wsadapter.Adapter {
	t.Helper()
	factory := platform.NewFactory()
	if err := wsadapter.ValidateFactory(factory); err != nil {
		t.Fatal(err)
	}
	adapter, err := factory.New(options)
	if err != nil {
		t.Fatal(err)
	}
	return adapter
}

func TestFactoryAndFailedUpgrade(t *testing.T) {
	if _, err := platform.NewFactory().New(wsadapter.Options{}); !errors.Is(err, wsadapter.ErrInvalidAdapter) {
		t.Fatalf("잘못된 제한값을 거부해야 합니다: %v", err)
	}
	adapter := newAdapter(t, wsadapter.Options{HandshakeTimeout: time.Second, ReadTimeout: time.Second, MaxMessageBytes: 1024})
	for _, canceled := range []bool{false, true} {
		req := httptest.NewRequest(http.MethodGet, "http://example.test/ws", nil)
		if canceled {
			ctx, cancel := context.WithCancel(req.Context())
			cancel()
			req = req.WithContext(ctx)
		}
		recorder := httptest.NewRecorder()
		conn, err := adapter.Upgrade(recorder, req)
		if err == nil || conn != nil || recorder.Code < 400 {
			t.Fatalf("업그레이드 실패 시 HTTP 오류를 작성해야 합니다: conn=%v err=%v code=%d", conn, err, recorder.Code)
		}
	}
}

func TestTextBinaryAndClose(t *testing.T) {
	adapter := newAdapter(t, wsadapter.Options{HandshakeTimeout: time.Second, ReadTimeout: time.Second, MaxMessageBytes: 1024})
	results := make(chan error, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := adapter.Upgrade(w, r)
		if err != nil {
			results <- err
			return
		}
		defer conn.Close()
		for range 2 {
			kind, payload, err := conn.ReadMessage()
			if err != nil {
				results <- err
				return
			}
			if err := conn.WriteMessage(kind, payload, time.Now().Add(time.Second)); err != nil {
				results <- err
				return
			}
		}
		results <- conn.WriteClose(wsadapter.CloseNormalClosure, "done", time.Now().Add(time.Second))
	}))
	defer server.Close()
	client, _, err := websocket.DefaultDialer.Dial("ws"+strings.TrimPrefix(server.URL, "http"), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	_ = client.SetReadDeadline(time.Now().Add(2 * time.Second))
	for _, kind := range []int{websocket.TextMessage, websocket.BinaryMessage} {
		if err := client.WriteMessage(kind, []byte("echo")); err != nil {
			t.Fatal(err)
		}
		got, payload, err := client.ReadMessage()
		if err != nil || got != kind || string(payload) != "echo" {
			t.Fatalf("메시지 종류와 내용을 보존해야 합니다: %d %q %v", got, payload, err)
		}
	}
	if _, _, err := client.ReadMessage(); !websocket.IsCloseError(err, websocket.CloseNormalClosure) {
		t.Fatalf("정상 close 코드를 전달해야 합니다: %v", err)
	}
	if err := <-results; err != nil {
		t.Fatal(err)
	}
}

func TestReadLimitAndCloseUnblockRead(t *testing.T) {
	for _, closeWhileReading := range []bool{false, true} {
		t.Run(map[bool]string{false: "message_limit", true: "concurrent_close"}[closeWhileReading], func(t *testing.T) {
			adapter := newAdapter(t, wsadapter.Options{HandshakeTimeout: time.Second, ReadTimeout: time.Second, MaxMessageBytes: 8})
			accepted := make(chan wsadapter.Connection, 1)
			done := make(chan error, 1)
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				conn, err := adapter.Upgrade(w, r)
				if err != nil {
					done <- err
					return
				}
				defer conn.Close()
				accepted <- conn
				_, _, err = conn.ReadMessage()
				done <- err
			}))
			defer server.Close()
			client, _, err := websocket.DefaultDialer.Dial("ws"+strings.TrimPrefix(server.URL, "http"), nil)
			if err != nil {
				t.Fatal(err)
			}
			defer client.Close()
			conn := <-accepted
			if closeWhileReading {
				_ = conn.Close()
			} else if err := client.WriteMessage(websocket.TextMessage, []byte("123456789")); err != nil {
				t.Fatal(err)
			}
			select {
			case err := <-done:
				if err == nil {
					t.Fatal("읽기가 오류로 종료되어야 합니다")
				}
			case <-time.After(2 * time.Second):
				t.Fatal("읽기 종료 시간 초과")
			}
		})
	}
}

func TestPongKeepsIdleConnectionAlive(t *testing.T) {
	const readTimeout = 250 * time.Millisecond
	adapter := newAdapter(t, wsadapter.Options{HandshakeTimeout: time.Second, ReadTimeout: readTimeout, MaxMessageBytes: 1024})
	result := make(chan error, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := adapter.Upgrade(w, r)
		if err != nil {
			result <- err
			return
		}
		defer conn.Close()
		read := make(chan error, 1)
		go func() { _, _, err := conn.ReadMessage(); read <- err }()
		ticker := time.NewTicker(readTimeout / 4)
		defer ticker.Stop()
		for {
			select {
			case err := <-read:
				result <- err
				return
			case <-ticker.C:
				if err := conn.Ping(time.Now().Add(time.Second)); err != nil {
					result <- err
					return
				}
			}
		}
	}))
	defer server.Close()
	client, _, err := websocket.DefaultDialer.Dial("ws"+strings.TrimPrefix(server.URL, "http"), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	readerDone := make(chan struct{})
	go func() { defer close(readerDone); _, _, _ = client.ReadMessage() }()
	// 애플리케이션 메시지가 없어도 pong을 받으면 읽기 제한 시간을 연장해야 합니다.
	time.Sleep(3 * readTimeout)
	if err := client.WriteMessage(websocket.TextMessage, []byte("alive")); err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-result:
		if err != nil {
			t.Fatalf("pong을 받은 연결이 먼저 종료됐습니다: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("pong 검증 시간 초과")
	}
	_ = client.Close()
	<-readerDone
}

func TestReadTimeoutExcludesHandlerTime(t *testing.T) {
	const readTimeout = 100 * time.Millisecond
	adapter := newAdapter(t, wsadapter.Options{HandshakeTimeout: time.Second, ReadTimeout: readTimeout, MaxMessageBytes: 1024})
	firstRead := make(chan struct{})
	nextRead := make(chan struct{})
	result := make(chan error, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := adapter.Upgrade(w, r)
		if err != nil {
			result <- err
			return
		}
		defer conn.Close()
		if _, _, err := conn.ReadMessage(); err != nil {
			result <- err
			return
		}
		close(firstRead)
		<-nextRead
		_, _, err = conn.ReadMessage()
		result <- err
	}))
	defer server.Close()
	client, _, err := websocket.DefaultDialer.Dial("ws"+strings.TrimPrefix(server.URL, "http"), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	if err := client.WriteMessage(websocket.TextMessage, []byte("first")); err != nil {
		t.Fatal(err)
	}
	select {
	case <-firstRead:
	case <-time.After(2 * time.Second):
		t.Fatal("첫 메시지 읽기 시간 초과")
	}
	time.Sleep(3 * readTimeout)
	close(nextRead)
	if err := client.WriteMessage(websocket.TextMessage, []byte("second")); err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-result:
		if err != nil {
			t.Fatalf("핸들러 실행 시간은 다음 읽기 제한에서 제외해야 합니다: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("두 번째 메시지 읽기 시간 초과")
	}
}

func TestIdleReadTimesOut(t *testing.T) {
	adapter := newAdapter(t, wsadapter.Options{HandshakeTimeout: time.Second, ReadTimeout: 50 * time.Millisecond, MaxMessageBytes: 1024})
	result := make(chan error, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := adapter.Upgrade(w, r)
		if err != nil {
			result <- err
			return
		}
		defer conn.Close()
		_, _, err = conn.ReadMessage()
		result <- err
	}))
	defer server.Close()
	client, _, err := websocket.DefaultDialer.Dial("ws"+strings.TrimPrefix(server.URL, "http"), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	select {
	case err := <-result:
		var networkError interface{ Timeout() bool }
		if !errors.As(err, &networkError) || !networkError.Timeout() {
			t.Fatalf("유휴 읽기는 시간 초과로 종료해야 합니다: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("읽기 시간 제한이 적용되지 않았습니다")
	}
}
