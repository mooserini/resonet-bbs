package main

import (
	"crypto/hmac"
	"crypto/sha1"
	"encoding/base32"
	"encoding/binary"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"wolfbbs/internal/auth"
	"wolfbbs/internal/repository"
)

func testTOTPCode(secret string, now time.Time) string {
	key, _ := base32.StdEncoding.WithPadding(base32.NoPadding).DecodeString(secret)
	buf := make([]byte, 8)
	binary.BigEndian.PutUint64(buf, uint64(now.Unix()/30))
	mac := hmac.New(sha1.New, key)
	mac.Write(buf)
	d := mac.Sum(nil)
	off := int(d[len(d)-1] & 0x0f)
	v := (int(d[off]&0x7f) << 24) | (int(d[off+1]) << 16) | (int(d[off+2]) << 8) | int(d[off+3])
	return fmt.Sprintf("%06d", v%1000000)
}

func totpRequest(code string) *http.Request {
	form := url.Values{"code": {code}}
	req := httptest.NewRequest(http.MethodPost, "/settings", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	return req
}

func TestTOTPSetupRequiresWorkingCode(t *testing.T) {
	authSvc := auth.NewService(repository.NewInMemoryUserRepository())
	if _, err := authSvc.Register("moose", "password123"); err != nil {
		t.Fatal(err)
	}
	app := &webApp{authSvc: authSvc, adminRepo: repository.NewInMemoryAdminRepository(), siteName: "ResoNET BBS"}

	if _, err := app.handleTOTPAction(totpRequest(""), "moose", "enable_2fa"); err != nil {
		t.Fatalf("enable: %v", err)
	}
	if u, _ := authSvc.GetUser("moose"); u.TOTPSecret != "" {
		t.Fatal("enabling must not turn 2FA on before a code is confirmed")
	}
	block := app.totpSetupBlock("moose", "")
	if !strings.Contains(block, "data:image/png;base64,") || !strings.Contains(block, "otpauth://totp/") {
		t.Fatalf("setup block should show a QR code and otpauth link: %s", block)
	}

	if _, err := app.handleTOTPAction(totpRequest("000000"), "moose", "confirm_2fa"); err == nil {
		t.Fatal("a wrong code must be rejected")
	}
	if u, _ := authSvc.GetUser("moose"); u.TOTPSecret != "" {
		t.Fatal("a wrong code must leave 2FA off")
	}

	pending, ok := app.loadTOTPPending("moose")
	if !ok {
		t.Fatal("pending secret should still exist after a wrong code")
	}
	if _, err := app.handleTOTPAction(totpRequest(testTOTPCode(pending.Secret, time.Now())), "moose", "confirm_2fa"); err != nil {
		t.Fatalf("confirm with the right code: %v", err)
	}
	u, _ := authSvc.GetUser("moose")
	if u.TOTPSecret != pending.Secret || len(u.RecoveryCodes) != 8 {
		t.Fatalf("expected 2FA on with 8 recovery codes, got secret=%t codes=%d", u.TOTPSecret != "", len(u.RecoveryCodes))
	}
	if _, ok := app.loadTOTPPending("moose"); ok {
		t.Fatal("pending secret should be cleared once 2FA is on")
	}
}

func TestTOTPSetupCancelAndExpiry(t *testing.T) {
	authSvc := auth.NewService(repository.NewInMemoryUserRepository())
	_, _ = authSvc.Register("moose", "password123")
	app := &webApp{authSvc: authSvc, adminRepo: repository.NewInMemoryAdminRepository()}
	app.storeTOTPPending("moose", &totpPending{Secret: "JBSWY3DPEHPK3PXP", CreatedAt: time.Now().Add(-totpPendingTTL - time.Minute)})
	if _, ok := app.loadTOTPPending("moose"); ok {
		t.Fatal("expired setup should not load")
	}
	_, _ = app.handleTOTPAction(totpRequest(""), "moose", "enable_2fa")
	_, _ = app.handleTOTPAction(totpRequest(""), "moose", "cancel_2fa_setup")
	if _, ok := app.loadTOTPPending("moose"); ok {
		t.Fatal("cancel should clear the pending setup")
	}
}
