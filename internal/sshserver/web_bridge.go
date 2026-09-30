package sshserver

import (
	"crypto/subtle"
	"net"
	"os"
	"strings"
)

// bridgedClientAddr returns the browser caller's address when this session
// was opened by the web terminal bridge (cmd/wolfbbs-web/web_terminal.go).
// Without it every web caller would look like the web container. The claim
// is only believed when it carries WOLFBBS_TERMINAL_BRIDGE_SECRET, which the
// bbs and web services share; any plain SSH client can send env vars.
func bridgedClientAddr(environ []string) string {
	secret := strings.TrimSpace(os.Getenv("WOLFBBS_TERMINAL_BRIDGE_SECRET"))
	if secret == "" {
		return ""
	}
	var client, token string
	for _, kv := range environ {
		key, value, ok := strings.Cut(kv, "=")
		if !ok {
			continue
		}
		switch key {
		case "WOLFBBS_BRIDGE_CLIENT":
			client = strings.TrimSpace(value)
		case "WOLFBBS_BRIDGE_TOKEN":
			token = value
		}
	}
	if client == "" || subtle.ConstantTimeCompare([]byte(token), []byte(secret)) != 1 {
		return ""
	}
	ip := net.ParseIP(client)
	if ip == nil {
		return ""
	}
	return net.JoinHostPort(ip.String(), "0")
}
