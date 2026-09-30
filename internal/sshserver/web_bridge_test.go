package sshserver

import "testing"

func TestBridgedClientAddr(t *testing.T) {
	env := []string{"WOLFBBS_BRIDGE_CLIENT=203.0.113.9", "WOLFBBS_BRIDGE_TOKEN=s3cret"}
	t.Setenv("WOLFBBS_TERMINAL_BRIDGE_SECRET", "")
	if got := bridgedClientAddr(env); got != "" {
		t.Fatalf("no secret configured must ignore the claim, got %q", got)
	}
	t.Setenv("WOLFBBS_TERMINAL_BRIDGE_SECRET", "s3cret")
	if got := bridgedClientAddr(env); got != "203.0.113.9:0" {
		t.Fatalf("got %q", got)
	}
	if got := bridgedClientAddr([]string{"WOLFBBS_BRIDGE_CLIENT=203.0.113.9", "WOLFBBS_BRIDGE_TOKEN=guess"}); got != "" {
		t.Fatalf("wrong token accepted: %q", got)
	}
	if got := bridgedClientAddr([]string{"WOLFBBS_BRIDGE_CLIENT=not-an-ip", "WOLFBBS_BRIDGE_TOKEN=s3cret"}); got != "" {
		t.Fatalf("junk address accepted: %q", got)
	}
}
