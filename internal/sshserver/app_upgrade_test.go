package sshserver

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestAppUpgradeUnavailableWithoutHostMounts(t *testing.T) {
	t.Setenv(appUpgradeCommandEnv, "docker compose up -d")
	t.Setenv(appUpgradeWorkDirEnv, filepath.Join(t.TempDir(), "missing"))
	if reason := appUpgradeUnavailableReason(); !strings.Contains(reason, "not mounted") {
		t.Fatalf("expected a missing-mount reason, got %q", reason)
	}
	t.Setenv(appUpgradeWorkDirEnv, t.TempDir())
	if reason := appUpgradeUnavailableReason(); reason != "no Docker socket" && reason != "" {
		t.Fatalf("unexpected reason %q", reason)
	}
	t.Setenv(appUpgradeCommandEnv, "")
	if reason := appUpgradeUnavailableReason(); reason != "" {
		t.Fatalf("unconfigured upgrade should fall through to the existing message, got %q", reason)
	}
}
