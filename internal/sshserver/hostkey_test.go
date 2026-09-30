package sshserver

import (
	"os"
	"path/filepath"
	"testing"

	gossh "golang.org/x/crypto/ssh"
)

func TestHostKeyIsCreatedOnceAndReused(t *testing.T) {
	path := filepath.Join(t.TempDir(), "ssh", "ssh_host_ed25519_key")
	first, created, err := loadOrCreateHostKey(path)
	if err != nil || !created {
		t.Fatalf("first load: created=%t err=%v", created, err)
	}
	info, err := os.Stat(path)
	if err != nil || info.Mode().Perm() != 0o600 {
		t.Fatalf("key file should exist with mode 600, got %v err=%v", info.Mode().Perm(), err)
	}
	second, created, err := loadOrCreateHostKey(path)
	if err != nil || created {
		t.Fatalf("second load: created=%t err=%v", created, err)
	}
	if gossh.FingerprintSHA256(first.PublicKey()) != gossh.FingerprintSHA256(second.PublicKey()) {
		t.Fatal("host key changed between loads")
	}
}

func TestHostKeyCorruptFileIsAnErrorNotAnOverwrite(t *testing.T) {
	path := filepath.Join(t.TempDir(), "ssh_host_ed25519_key")
	_ = os.WriteFile(path, []byte("not a key"), 0o600)
	if _, _, err := loadOrCreateHostKey(path); err == nil {
		t.Fatal("expected an error for a corrupt key")
	}
	if raw, _ := os.ReadFile(path); string(raw) != "not a key" {
		t.Fatal("a corrupt key must not be silently replaced")
	}
}
