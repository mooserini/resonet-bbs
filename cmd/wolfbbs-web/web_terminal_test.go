package main

import (
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	gssh "github.com/gliderlabs/ssh"
	"github.com/gorilla/websocket"
)

// startEchoSSH runs a tiny SSH server that reports its PTY size and env,
// then echoes input back upper-cased until it sees "q".
func startEchoSSH(t *testing.T) string {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	srv := &gssh.Server{Handler: func(s gssh.Session) {
		pty, winCh, ok := s.Pty()
		if !ok {
			io.WriteString(s, "no pty\r\n")
			return
		}
		fmt.Fprintf(s, "PTY %s %dx%d ENV %s\r\n", pty.Term, pty.Window.Width, pty.Window.Height, strings.Join(s.Environ(), ","))
		go func() {
			for win := range winCh {
				fmt.Fprintf(s, "WIN %dx%d\r\n", win.Width, win.Height)
			}
		}()
		buf := make([]byte, 64)
		for {
			n, err := s.Read(buf)
			if err != nil {
				return
			}
			in := string(buf[:n])
			if strings.Contains(in, "q") {
				io.WriteString(s, "BYE\r\n")
				return
			}
			io.WriteString(s, strings.ToUpper(in))
		}
	}}
	go srv.Serve(ln)
	t.Cleanup(func() { _ = srv.Close() })
	return ln.Addr().String()
}

func readUntil(t *testing.T, conn *websocket.Conn, want string) string {
	t.Helper()
	var got strings.Builder
	_ = conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	for !strings.Contains(got.String(), want) {
		_, data, err := conn.ReadMessage()
		if err != nil {
			t.Fatalf("waiting for %q, got %q: %v", want, got.String(), err)
		}
		got.Write(data)
	}
	return got.String()
}

func TestWebTerminalBridgesToSSH(t *testing.T) {
	t.Setenv("WOLFBBS_TERMINAL_SSH_ADDR", startEchoSSH(t))
	t.Setenv("WOLFBBS_TERMINAL_BRIDGE_SECRET", "s3cret")
	term := newWebTerminalFromEnv(func() string { return "Test BBS" })
	web := httptest.NewServer(http.HandlerFunc(term.handleSocket))
	defer web.Close()

	wsURL := "ws" + strings.TrimPrefix(web.URL, "http") + terminalSocketPath + "?cols=100&rows=30"
	header := http.Header{"Origin": {web.URL}, "Cf-Connecting-Ip": {"203.0.113.7"}}
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, header)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()

	banner := readUntil(t, conn, "\r\n")
	if !strings.Contains(banner, "PTY xterm-256color 100x30") {
		t.Fatalf("pty not requested with browser size: %q", banner)
	}
	if !strings.Contains(banner, "WOLFBBS_BRIDGE_CLIENT=203.0.113.7") || !strings.Contains(banner, "WOLFBBS_BRIDGE_TOKEN=s3cret") {
		t.Fatalf("client address not passed through: %q", banner)
	}

	if err := conn.WriteMessage(websocket.BinaryMessage, []byte("hi\r")); err != nil {
		t.Fatal(err)
	}
	readUntil(t, conn, "HI\r")

	if err := conn.WriteMessage(websocket.TextMessage, []byte(`{"t":"resize","cols":132,"rows":43}`)); err != nil {
		t.Fatal(err)
	}
	readUntil(t, conn, "WIN 132x43")

	if err := conn.WriteMessage(websocket.BinaryMessage, []byte("q")); err != nil {
		t.Fatal(err)
	}
	readUntil(t, conn, "BYE")
	_ = conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	for {
		if _, _, err := conn.ReadMessage(); err != nil {
			break // bridge hung up after the board ended the session
		}
	}
}

func TestWebTerminalRejectsForeignOrigin(t *testing.T) {
	t.Setenv("WOLFBBS_TERMINAL_SSH_ADDR", startEchoSSH(t))
	term := newWebTerminalFromEnv(nil)
	web := httptest.NewServer(http.HandlerFunc(term.handleSocket))
	defer web.Close()

	wsURL := "ws" + strings.TrimPrefix(web.URL, "http") + terminalSocketPath
	for _, origin := range []string{"https://evil.example", ""} {
		header := http.Header{}
		if origin != "" {
			header.Set("Origin", origin)
		}
		if _, resp, err := websocket.DefaultDialer.Dial(wsURL, header); err == nil {
			t.Fatalf("origin %q should be refused", origin)
		} else if resp == nil || resp.StatusCode != http.StatusForbidden {
			t.Fatalf("origin %q: want 403, got %v", origin, resp)
		}
	}
}

func TestWebTerminalDisabled(t *testing.T) {
	t.Setenv("WOLFBBS_WEB_TERMINAL", "off")
	term := newWebTerminalFromEnv(nil)
	rr := httptest.NewRecorder()
	term.handlePage(rr, httptest.NewRequest(http.MethodGet, terminalPagePath, nil))
	if rr.Code != http.StatusNotFound {
		t.Fatalf("disabled terminal page: want 404, got %d", rr.Code)
	}
}

func TestTerminalClientIPTrustsHeadersOnlyFromLocalPeers(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.Header.Set("Cf-Connecting-IP", "198.51.100.4")
	r.RemoteAddr = "172.18.0.1:5555"
	if got := terminalClientIP(r); got != "198.51.100.4" {
		t.Fatalf("local peer: got %q", got)
	}
	r.RemoteAddr = "8.8.8.8:5555"
	if got := terminalClientIP(r); got != "8.8.8.8" {
		t.Fatalf("public peer must not be able to spoof: got %q", got)
	}
}

func TestModernUIInjectsTerminalLauncherOnlyWhenEnabled(t *testing.T) {
	page := `<!doctype html><html><head></head><body>x</body></html>`
	if strings.Contains(injectModernUIWith(page, modernUIView{siteName: "S"}), "wolfbbs-terminal-launcher") {
		t.Fatal("launcher injected while terminal disabled")
	}
	if !strings.Contains(injectModernUIWith(page, modernUIView{siteName: "S", terminal: true}), "wolfbbs-terminal-launcher") {
		t.Fatal("launcher missing while terminal enabled")
	}
}
