package sshserver

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/pem"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	gssh "github.com/gliderlabs/ssh"
	gossh "golang.org/x/crypto/ssh"
)

// defaultHostKeyPath lives on the "sshkeys" volume in docker-compose.yml, so
// the key survives rebuilds. Without a stable key, every restart generated a
// new one and SSH clients warned "REMOTE HOST IDENTIFICATION HAS CHANGED".
const defaultHostKeyPath = ".wolfbbs/ssh/ssh_host_ed25519_key"

func hostKeyPath() string {
	if p := strings.TrimSpace(os.Getenv("WOLFBBS_SSH_HOST_KEY")); p != "" {
		return p
	}
	return defaultHostKeyPath
}

// loadOrCreateHostKey reads the host key at path, generating and saving a new
// ed25519 key (mode 600) the first time.
func loadOrCreateHostKey(path string) (gssh.Signer, bool, error) {
	if raw, err := os.ReadFile(path); err == nil {
		signer, err := gossh.ParsePrivateKey(raw)
		if err != nil {
			return nil, false, fmt.Errorf("parse host key %s: %w", path, err)
		}
		return signer, false, nil
	} else if !os.IsNotExist(err) {
		return nil, false, fmt.Errorf("read host key %s: %w", path, err)
	}
	_, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return nil, false, err
	}
	block, err := gossh.MarshalPrivateKey(priv, "wolfbbs host key")
	if err != nil {
		return nil, false, err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return nil, false, err
	}
	if err := os.WriteFile(path, pem.EncodeToMemory(block), 0o600); err != nil {
		return nil, false, err
	}
	signer, err := gossh.NewSignerFromKey(priv)
	if err != nil {
		return nil, false, err
	}
	return signer, true, nil
}
