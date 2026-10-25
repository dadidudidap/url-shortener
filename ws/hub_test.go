package ws

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

func newTestServer(t *testing.T) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		Manager.ServeWS(w, r, r.URL.Query().Get("code"))
	}))
	t.Cleanup(srv.Close)
	return srv
}

func dialTestClient(t *testing.T, srv *httptest.Server, code string, header http.Header) *websocket.Conn {
	t.Helper()
	url := "ws" + strings.TrimPrefix(srv.URL, "http") + "/?code=" + code
	conn, _, err := websocket.DefaultDialer.Dial(url, header)
	if err != nil {
		t.Fatalf("dial %s: %v", code, err)
	}
	t.Cleanup(func() { conn.Close() })
	return conn
}

func waitForClients(t *testing.T, code string, want int) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		Manager.mu.RLock()
		got := len(Manager.clients[code])
		Manager.mu.RUnlock()
		if got == want {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("client count for %q never reached %d", code, want)
}

func readClicks(t *testing.T, conn *websocket.Conn, wantCode string) int32 {
	t.Helper()
	_ = conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	var message struct {
		ShortCode string `json:"short_code"`
		Clicks    int32  `json:"clicks"`
	}
	if err := conn.ReadJSON(&message); err != nil {
		t.Fatalf("read message: %v", err)
	}
	if message.ShortCode != wantCode {
		t.Fatalf("short_code = %q, want %q", message.ShortCode, wantCode)
	}
	return message.Clicks
}

func TestBroadcastOnlyReachesSubscribers(t *testing.T) {
	srv := newTestServer(t)
	first := dialTestClient(t, srv, "live01", nil)
	second := dialTestClient(t, srv, "live01", nil)
	other := dialTestClient(t, srv, "other1", nil)
	waitForClients(t, "live01", 2)
	waitForClients(t, "other1", 1)

	Manager.Broadcast("live01", 7)

	for _, conn := range []*websocket.Conn{first, second} {
		if got := readClicks(t, conn, "live01"); got != 7 {
			t.Errorf("clicks = %d, want 7", got)
		}
	}

	_ = other.SetReadDeadline(time.Now().Add(100 * time.Millisecond))
	if _, _, err := other.ReadMessage(); err == nil {
		t.Fatal("client on another code received the broadcast")
	}
}

func TestConcurrentBroadcastsAreSerialized(t *testing.T) {
	srv := newTestServer(t)
	conn := dialTestClient(t, srv, "race01", nil)
	waitForClients(t, "race01", 1)

	const messages = 10
	for i := int32(0); i < messages; i++ {
		go Manager.Broadcast("race01", i)
	}

	for i := 0; i < messages; i++ {
		readClicks(t, conn, "race01")
	}
}

func TestDisconnectRemovesClient(t *testing.T) {
	srv := newTestServer(t)
	conn := dialTestClient(t, srv, "gone01", nil)
	waitForClients(t, "gone01", 1)

	conn.Close()
	waitForClients(t, "gone01", 0)
}

func TestRejectsCrossOriginUpgrade(t *testing.T) {
	srv := newTestServer(t)
	url := "ws" + strings.TrimPrefix(srv.URL, "http") + "/?code=evil01"
	header := http.Header{"Origin": []string{"http://evil.example"}}

	conn, resp, err := websocket.DefaultDialer.Dial(url, header)
	if err == nil {
		conn.Close()
		t.Fatal("cross-origin upgrade was accepted")
	}
	if resp == nil || resp.StatusCode != http.StatusForbidden {
		t.Fatalf("status = %v, want 403", resp)
	}
}
