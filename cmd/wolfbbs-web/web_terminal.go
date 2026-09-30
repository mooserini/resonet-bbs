package main

import (
	"encoding/json"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"golang.org/x/crypto/ssh"
)

// The browser terminal is the real SSH board, not the command-mode
// WebSocket login server: /terminal/ws dials the BBS SSH listener from inside
// the web container and relays raw bytes both ways, so ANSI menus and doors
// behave exactly as they do in a desktop SSH client. The BBS login screen
// still runs inside the session; the bridge adds no authentication of its own.

const (
	terminalPagePath   = "/terminal"
	terminalSocketPath = "/terminal/ws"
	terminalPingEvery  = 30 * time.Second
	terminalDialWait   = 5 * time.Second
)

type webTerminal struct {
	enabled  bool
	sshAddr  string
	secret   string
	siteName func() string
	upgrader websocket.Upgrader
}

func newWebTerminalFromEnv(siteName func() string) *webTerminal {
	t := &webTerminal{
		enabled:  true,
		sshAddr:  "127.0.0.1:2222",
		secret:   strings.TrimSpace(os.Getenv("WOLFBBS_TERMINAL_BRIDGE_SECRET")),
		siteName: siteName,
	}
	switch strings.ToLower(strings.TrimSpace(os.Getenv("WOLFBBS_WEB_TERMINAL"))) {
	case "0", "false", "off", "no":
		t.enabled = false
	}
	if addr := strings.TrimSpace(os.Getenv("WOLFBBS_TERMINAL_SSH_ADDR")); addr != "" {
		t.sshAddr = addr
	}
	t.upgrader = websocket.Upgrader{
		ReadBufferSize:  4096,
		WriteBufferSize: 32 * 1024,
		CheckOrigin:     terminalOriginAllowed,
	}
	return t
}

// terminalOriginAllowed blocks cross-site WebSocket hijacking: the socket
// only opens from a page served by this same host.
func terminalOriginAllowed(r *http.Request) bool {
	origin := strings.TrimSpace(r.Header.Get("Origin"))
	if origin == "" {
		return false
	}
	host := origin
	if i := strings.Index(host, "://"); i >= 0 {
		host = host[i+3:]
	} else {
		return false
	}
	return strings.EqualFold(host, strings.TrimSpace(r.Host))
}

// terminalClientIP picks the caller's address. Proxy headers are only
// believed when the direct peer is local (cloudflared, Docker's gateway).
func terminalClientIP(r *http.Request) string {
	peer := r.RemoteAddr
	if host, _, err := net.SplitHostPort(peer); err == nil {
		peer = host
	}
	peerIP := net.ParseIP(strings.Trim(peer, "[]"))
	if peerIP == nil {
		return ""
	}
	if !peerIP.IsLoopback() && !peerIP.IsPrivate() {
		return peerIP.String()
	}
	if ip := net.ParseIP(strings.TrimSpace(r.Header.Get("Cf-Connecting-IP"))); ip != nil {
		return ip.String()
	}
	if first, _, _ := strings.Cut(r.Header.Get("X-Forwarded-For"), ","); first != "" {
		if ip := net.ParseIP(strings.TrimSpace(first)); ip != nil {
			return ip.String()
		}
	}
	return peerIP.String()
}

func clampTerminalDim(raw string, fallback, lo, hi int) int {
	n, err := strconv.Atoi(strings.TrimSpace(raw))
	if err != nil {
		return fallback
	}
	if n < lo {
		return lo
	}
	if n > hi {
		return hi
	}
	return n
}

type terminalControl struct {
	Type string `json:"t"`
	Cols int    `json:"cols"`
	Rows int    `json:"rows"`
}

func (t *webTerminal) handleSocket(w http.ResponseWriter, r *http.Request) {
	if !t.enabled {
		http.NotFound(w, r)
		return
	}
	conn, err := t.upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	defer conn.Close()
	conn.SetReadLimit(64 * 1024)

	cols := clampTerminalDim(r.URL.Query().Get("cols"), 80, 20, 400)
	rows := clampTerminalDim(r.URL.Query().Get("rows"), 24, 10, 200)

	var writeMu sync.Mutex
	send := func(kind int, data []byte) error {
		writeMu.Lock()
		defer writeMu.Unlock()
		_ = conn.SetWriteDeadline(time.Now().Add(15 * time.Second))
		return conn.WriteMessage(kind, data)
	}
	fail := func(msg string) {
		_ = send(websocket.BinaryMessage, []byte("\r\n\x1b[0;31m"+msg+"\x1b[0m\r\n"))
	}

	client, err := ssh.Dial("tcp", t.sshAddr, &ssh.ClientConfig{
		User: "web",
		// The BBS listener sits on the private Docker network (or loopback),
		// and its host key is regenerated per container, so there is nothing
		// stable to pin. Callers authenticate at the BBS login screen.
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         terminalDialWait,
	})
	if err != nil {
		log.Printf("web terminal: ssh dial %s: %v", t.sshAddr, err)
		fail("The board is not answering right now. Try again in a minute.")
		return
	}
	defer client.Close()

	sess, err := client.NewSession()
	if err != nil {
		fail("Could not open a session on the board.")
		return
	}
	defer sess.Close()
	if t.secret != "" {
		if ip := terminalClientIP(r); ip != "" {
			_ = sess.Setenv("WOLFBBS_BRIDGE_CLIENT", ip)
			_ = sess.Setenv("WOLFBBS_BRIDGE_TOKEN", t.secret)
		}
	}
	if err := sess.RequestPty("xterm-256color", rows, cols, ssh.TerminalModes{}); err != nil {
		fail("The board refused a terminal.")
		return
	}
	stdin, err := sess.StdinPipe()
	if err != nil {
		return
	}
	stdout, err := sess.StdoutPipe()
	if err != nil {
		return
	}
	stderr, err := sess.StderrPipe()
	if err != nil {
		return
	}
	if err := sess.Shell(); err != nil {
		fail("The board did not start a shell.")
		return
	}

	done := make(chan struct{})
	var once sync.Once
	finish := func() { once.Do(func() { close(done) }) }

	// Hang up only after both output streams drain, so the board's last
	// words (a goodbye screen) reach the browser before the close frame.
	var pumps sync.WaitGroup
	pump := func(src io.Reader) {
		defer pumps.Done()
		buf := make([]byte, 32*1024)
		for {
			n, err := src.Read(buf)
			if n > 0 {
				if werr := send(websocket.BinaryMessage, buf[:n]); werr != nil {
					finish()
					return
				}
			}
			if err != nil {
				return
			}
		}
	}
	pumps.Add(2)
	go pump(stdout)
	go pump(stderr)
	go func() {
		pumps.Wait()
		finish()
	}()

	go func() {
		defer finish()
		for {
			kind, payload, err := conn.ReadMessage()
			if err != nil {
				return
			}
			switch kind {
			case websocket.BinaryMessage:
				if _, err := stdin.Write(payload); err != nil {
					return
				}
			case websocket.TextMessage:
				var ctl terminalControl
				if json.Unmarshal(payload, &ctl) == nil && ctl.Type == "resize" {
					c := clampTerminalDim(strconv.Itoa(ctl.Cols), 80, 20, 400)
					h := clampTerminalDim(strconv.Itoa(ctl.Rows), 24, 10, 200)
					_ = sess.WindowChange(h, c)
				}
			}
		}
	}()

	// Cloudflare drops WebSocket connections that go quiet for ~100s.
	ticker := time.NewTicker(terminalPingEvery)
	defer ticker.Stop()
	for {
		select {
		case <-done:
			_ = send(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, "bye"))
			return
		case <-ticker.C:
			writeMu.Lock()
			err := conn.WriteControl(websocket.PingMessage, nil, time.Now().Add(5*time.Second))
			writeMu.Unlock()
			if err != nil {
				return
			}
		}
	}
}

// connectLink points /connect callers at the browser terminal.
func (t *webTerminal) connectLink() string {
	if t == nil || !t.enabled {
		return ""
	}
	return `<p><a class="wolfbbs-action-card" href="/terminal"><strong>Open the board in your browser</strong><span>the full SSH experience, doors included, with no client to install. Press the backquote key on any page to drop it down.</span></a></p>` + "\n"
}

func (t *webTerminal) handlePage(w http.ResponseWriter, r *http.Request) {
	if !t.enabled || r.URL.Path != terminalPagePath {
		http.NotFound(w, r)
		return
	}
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	site := "WolfBBS"
	if t.siteName != nil {
		if name := strings.TrimSpace(t.siteName()); name != "" {
			site = name
		}
	}
	bodyClass := ""
	if r.URL.Query().Get("embed") == "1" {
		bodyClass = "embed"
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	page := strings.NewReplacer(
		"{{SITE}}", htmlEscape(site),
		"{{BODYCLASS}}", bodyClass,
		"{{ICONS}}", brandIconLinks,
	).Replace(terminalPageHTML)
	_, _ = w.Write([]byte(page))
}

// terminalPageHTML is served as-is (no modern UI injection) so nothing else
// on the page competes with the terminal for keystrokes.
const terminalPageHTML = `<!doctype html>
<html lang="en"><head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>{{SITE}} Terminal</title>
{{ICONS}}
<link rel="stylesheet" href="https://cdn.jsdelivr.net/npm/@xterm/xterm@5.5.0/css/xterm.min.css">
<style>
@font-face{font-family:"Web437 ATT PC6300";src:url("/assets/fonts/Web437_ATT_PC6300.woff") format("woff");font-display:block}
:root{--ink:#171819;--cream:#EDE3D5;--sub:#B7A593;--chrome:linear-gradient(180deg,#F7EEE2 0%,#D7C5B2 55%,#A98B70 100%);--pc:"Web437 ATT PC6300",ui-monospace,Menlo,monospace}
html,body{margin:0;height:100%;background:var(--ink);color:var(--cream)}
body{display:flex;flex-direction:column;overflow:hidden}
.bar{display:flex;align-items:center;gap:14px;padding:5px 12px;background:var(--chrome);color:var(--ink);font:14px/1.2 var(--pc);letter-spacing:.04em;border-bottom:1px solid #4A342A;user-select:none}
.bar .title{font-weight:normal}
.bar .spacer{flex:1}
.bar a,.bar button{font:inherit;color:var(--ink);background:none;border:1px solid rgba(23,24,25,.45);padding:1px 8px;cursor:pointer;text-decoration:none}
.bar a:hover,.bar button:hover{background:rgba(23,24,25,.12)}
#status::before{content:"\25CF ";}
#status.on{color:#006b00}
#status.off{color:#8b0000}
#pop{display:none}
.embed #pop{display:inline-block}
body:not(.embed) #close{display:none}
.embed #hint{display:inline}
#hint{display:none;color:#4A342A}
#wrap{position:relative;flex:1;min-height:0;display:flex;align-items:center;justify-content:center;background:#000}
#term{width:100%;height:100%;box-sizing:border-box;padding:6px}
#wrap::after{content:"";position:absolute;inset:0;pointer-events:none;background:repeating-linear-gradient(0deg,rgba(0,0,0,.18) 0,rgba(0,0,0,.18) 1px,transparent 1px,transparent 3px);box-shadow:inset 0 0 90px rgba(0,0,0,.65)}
.xterm .xterm-viewport{background:#000 !important}
</style>
</head>
<body class="{{BODYCLASS}}">
<header class="bar">
<span class="title">{{SITE}} &middot; TERMINAL</span>
<span id="status">CONNECTING</span>
<span class="spacer"></span>
<span id="hint">CTRL+&#96; TO HIDE</span>
<button type="button" id="reconnect" hidden>REDIAL</button>
<a href="/terminal" target="_top" id="pop">FULL SCREEN</a>
<button type="button" id="close">CLOSE</button>
</header>
<main id="wrap"><div id="term"></div></main>
<script src="https://cdn.jsdelivr.net/npm/@xterm/xterm@5.5.0/lib/xterm.min.js"></script>
<script src="https://cdn.jsdelivr.net/npm/@xterm/addon-fit@0.10.0/lib/addon-fit.min.js"></script>
<script>
(function(){
  var embed = document.body.classList.contains('embed');
  var statusEl = document.getElementById('status');
  var redial = document.getElementById('reconnect');
  var host = document.getElementById('term');
  function setStatus(text, cls){ statusEl.textContent = text; statusEl.className = cls || ''; }
  function closePopup(){ if (embed && window.parent !== window) window.parent.postMessage({wolfbbs:'terminal-close'}, location.origin); }
  document.getElementById('close').addEventListener('click', closePopup);

  if (typeof Terminal === 'undefined') { setStatus('TERMINAL FAILED TO LOAD', 'off'); return; }

  var phosphor = '#00ff66';
  try {
    var prefs = JSON.parse(localStorage.getItem('wolfbbs:ui:prefs:v2') || '{}');
    phosphor = {amber:'#ffb000', cyan:'#00e5ff', copper:'#e29b68', violet:'#d946ef'}[prefs.phosphor] || phosphor;
  } catch (e) {}

  var term = new Terminal({
    fontFamily: '"Web437 ATT PC6300", ui-monospace, Menlo, monospace',
    fontSize: 16,
    lineHeight: 1,
    letterSpacing: 0,
    cursorBlink: true,
    scrollback: 2000,
    convertEol: false,
    theme: {
      background:'#000000', foreground:'#AAAAAA', cursor:phosphor, cursorAccent:'#000000', selectionBackground:'rgba(237,227,213,.3)',
      black:'#000000', red:'#AA0000', green:'#00AA00', yellow:'#AA5500', blue:'#0000AA', magenta:'#AA00AA', cyan:'#00AAAA', white:'#AAAAAA',
      brightBlack:'#555555', brightRed:'#FF5555', brightGreen:'#55FF55', brightYellow:'#FFFF55', brightBlue:'#5555FF', brightMagenta:'#FF55FF', brightCyan:'#55FFFF', brightWhite:'#FFFFFF'
    }
  });
  var fit = new FitAddon.FitAddon();
  term.loadAddon(fit);

  // Scale the font so at least 80x25 always fits, like a CRT filling its glass.
  function layout(){
    var w = host.clientWidth - 12, h = host.clientHeight - 12;
    var size = Math.floor(Math.min(w / 40, h / 25));
    size = Math.max(8, Math.min(size, 32));
    if (term.options.fontSize !== size) term.options.fontSize = size;
    try { fit.fit(); } catch (e) {}
  }

  var ws = null;
  var enc = new TextEncoder();
  function sendResize(){
    if (ws && ws.readyState === 1) ws.send(JSON.stringify({t:'resize', cols:term.cols, rows:term.rows}));
  }
  function connect(){
    if (ws && (ws.readyState === 0 || ws.readyState === 1)) return;
    redial.hidden = true;
    setStatus('DIALING');
    var proto = location.protocol === 'https:' ? 'wss:' : 'ws:';
    ws = new WebSocket(proto + '//' + location.host + '/terminal/ws?cols=' + term.cols + '&rows=' + term.rows);
    ws.binaryType = 'arraybuffer';
    ws.onopen = function(){ setStatus('ONLINE', 'on'); layout(); sendResize(); term.focus(); };
    ws.onmessage = function(ev){
      if (typeof ev.data === 'string') term.write(ev.data); else term.write(new Uint8Array(ev.data));
    };
    ws.onclose = function(){
      setStatus('NO CARRIER', 'off');
      redial.hidden = false;
      term.write('\r\n\x1b[0m\x1b[1;30m-- NO CARRIER -- press Enter to redial --\x1b[0m\r\n');
    };
  }
  redial.addEventListener('click', function(){ connect(); term.focus(); });

  term.onData(function(data){
    if (ws && ws.readyState === 1) { ws.send(enc.encode(data)); return; }
    if (data === '\r') connect();
  });
  term.onBinary(function(data){
    if (!ws || ws.readyState !== 1) return;
    var bytes = new Uint8Array(data.length);
    for (var i = 0; i < data.length; i++) bytes[i] = data.charCodeAt(i) & 255;
    ws.send(bytes);
  });
  term.onResize(sendResize);
  term.attachCustomKeyEventHandler(function(ev){
    if (embed && ev.type === 'keydown' && ev.ctrlKey && (ev.code === 'Backquote' || ev.key === '\x60')) { closePopup(); return false; }
    return true;
  });
  window.addEventListener('message', function(ev){
    if (ev.origin === location.origin && ev.data && ev.data.wolfbbs === 'terminal-focus') { layout(); term.focus(); }
  });

  var pending = 0;
  window.addEventListener('resize', function(){ clearTimeout(pending); pending = setTimeout(layout, 80); });

  // The board sizes its whole session from the first PTY size, so settle
  // the layout for a couple of frames before dialing.
  function start(){
    term.open(host);
    layout();
    requestAnimationFrame(function(){ requestAnimationFrame(function(){ layout(); connect(); }); });
  }
  if (document.fonts && document.fonts.load) {
    document.fonts.load('16px "Web437 ATT PC6300"').then(start, start);
  } else {
    start();
  }
})();
</script>
</body></html>`

// terminalLauncherTag is injected into every site page. It drops the
// terminal down over the page (Quake-console style) on the backquote key.
const terminalLauncherTag = `<script id="wolfbbs-terminal-launcher">
(function(){
  if (window.top !== window) return;
  var overlay = null, frame = null;
  function build(){
    var style = document.createElement('style');
    style.textContent =
      '#wolfbbs-term-drop{position:fixed;left:0;right:0;top:0;height:min(72vh,720px);z-index:2147483000;display:flex;flex-direction:column;' +
      'background:#171819;border-bottom:3px solid #A98B70;box-shadow:0 18px 60px rgba(0,0,0,.6);transform:translateY(-102%);transition:transform .16s ease-out}' +
      '#wolfbbs-term-drop.open{transform:none}' +
      '#wolfbbs-term-drop iframe{flex:1;border:0;width:100%;background:#000}' +
      '@media (prefers-reduced-motion:reduce){#wolfbbs-term-drop{transition:none}}';
    document.head.appendChild(style);
    overlay = document.createElement('div');
    overlay.id = 'wolfbbs-term-drop';
    overlay.setAttribute('role', 'dialog');
    overlay.setAttribute('aria-label', 'BBS terminal');
    frame = document.createElement('iframe');
    frame.title = 'BBS terminal';
    frame.src = '/terminal?embed=1';
    overlay.appendChild(frame);
    document.body.appendChild(overlay);
    frame.addEventListener('load', focusFrame);
  }
  function focusFrame(){
    if (!frame) return;
    frame.focus();
    try { frame.contentWindow.postMessage({wolfbbs:'terminal-focus'}, location.origin); } catch (e) {}
  }
  function open(){
    if (!overlay) build();
    requestAnimationFrame(function(){ overlay.classList.add('open'); focusFrame(); });
  }
  function close(){
    if (!overlay) return;
    overlay.classList.remove('open');
    if (frame) frame.blur();
    window.focus();
  }
  function toggle(){ if (overlay && overlay.classList.contains('open')) close(); else open(); }
  document.addEventListener('keydown', function(ev){
    if ((ev.code !== 'Backquote' && ev.key !== '\x60') || ev.metaKey || ev.altKey || ev.shiftKey) return;
    var t = ev.target;
    var editing = t && (t.isContentEditable || /^(INPUT|TEXTAREA|SELECT)$/.test(t.tagName || ''));
    if (editing && !ev.ctrlKey) return;
    ev.preventDefault();
    toggle();
  });
  window.addEventListener('message', function(ev){
    if (ev.origin === location.origin && ev.data && ev.data.wolfbbs === 'terminal-close') close();
  });
  window.wolfbbsOpenTerminal = open;
})();
</script>`
