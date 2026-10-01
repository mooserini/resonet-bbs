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

func TestAcknowledgeCodesVerifiesAccount(t *testing.T) {
	authSvc := auth.NewService(repository.NewInMemoryUserRepository())
	if _, err := authSvc.Register("moose", "password123"); err != nil {
		t.Fatal(err)
	}
	app := &webApp{authSvc: authSvc, adminRepo: repository.NewInMemoryAdminRepository(), siteName: "ResoNET BBS"}

	if _, err := app.handleTOTPAction(totpRequest(""), "moose", "enable_2fa"); err != nil {
		t.Fatalf("enable: %v", err)
	}
	pending, ok := app.loadTOTPPending("moose")
	if !ok {
		t.Fatal("pending secret should exist after enable")
	}
	if _, err := app.handleTOTPAction(totpRequest(testTOTPCode(pending.Secret, time.Now())), "moose", "confirm_2fa"); err != nil {
		t.Fatalf("confirm: %v", err)
	}
	if u, _ := authSvc.GetUser("moose"); u.Verified {
		t.Fatal("confirming 2FA must not verify the account before codes are acknowledged")
	}
	if _, err := app.handleTOTPAction(totpRequest(""), "moose", "acknowledge_codes"); err != nil {
		t.Fatalf("acknowledge: %v", err)
	}
	if u, _ := authSvc.GetUser("moose"); !u.Verified {
		t.Fatal("acknowledging saved codes should verify the account")
	}
}

func TestSettingsShowsCodesAckUntilAcknowledged(t *testing.T) {
	authSvc := auth.NewService(repository.NewInMemoryUserRepository())
	if _, err := authSvc.Register("moose", "password123"); err != nil {
		t.Fatal(err)
	}
	app := &webApp{authSvc: authSvc, adminRepo: repository.NewInMemoryAdminRepository(), siteName: "ResoNET BBS", sessions: map[string]sessionState{}}
	sess, ok := app.createSession("moose")
	if !ok {
		t.Fatal("session creation failed")
	}
	getSettings := func() string {
		req := httptest.NewRequest(http.MethodGet, "/settings", nil)
		req.AddCookie(&http.Cookie{Name: "wolfbbs_session", Value: sess})
		rr := httptest.NewRecorder()
		app.handleSettings(rr, req)
		return rr.Body.String()
	}
	if body := getSettings(); strings.Contains(body, "acknowledge_codes") {
		t.Fatal("no ack button before 2FA is on")
	}
	if _, err := app.handleTOTPAction(totpRequest(""), "moose", "enable_2fa"); err != nil {
		t.Fatalf("enable: %v", err)
	}
	pending, ok := app.loadTOTPPending("moose")
	if !ok {
		t.Fatal("pending secret should exist after enable")
	}
	if _, err := app.handleTOTPAction(totpRequest(testTOTPCode(pending.Secret, time.Now())), "moose", "confirm_2fa"); err != nil {
		t.Fatalf("confirm: %v", err)
	}
	if body := getSettings(); !strings.Contains(body, "acknowledge_codes") {
		t.Fatal("settings should ask to acknowledge newly shown codes")
	}
	if _, err := app.handleTOTPAction(totpRequest(""), "moose", "acknowledge_codes"); err != nil {
		t.Fatalf("acknowledge: %v", err)
	}
	if body := getSettings(); strings.Contains(body, "acknowledge_codes") {
		t.Fatal("ack button should disappear once codes are acknowledged")
	}
}

func TestDisable2FARecomputesVerified(t *testing.T) {
	authSvc := auth.NewService(repository.NewInMemoryUserRepository())
	if _, err := authSvc.Register("moose", "password123"); err != nil {
		t.Fatal(err)
	}
	app := &webApp{authSvc: authSvc, adminRepo: repository.NewInMemoryAdminRepository(), siteName: "ResoNET BBS", sessions: map[string]sessionState{}}
	sess, ok := app.createSession("moose")
	if !ok {
		t.Fatal("session creation failed")
	}
	post := func(action string, pairs ...string) {
		form := url.Values{"action": {action}, "csrf_token": {app.sessions[sess].csrf}}
		for i := 0; i+1 < len(pairs); i += 2 {
			form.Set(pairs[i], pairs[i+1])
		}
		req := httptest.NewRequest(http.MethodPost, "/settings", strings.NewReader(form.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req.AddCookie(&http.Cookie{Name: "wolfbbs_session", Value: sess})
		rr := httptest.NewRecorder()
		app.handleSettings(rr, req)
		if rr.Code != http.StatusFound {
			t.Fatalf("%s status = %d body=%s", action, rr.Code, rr.Body.String())
		}
	}
	post("enable_2fa")
	pending, ok := app.loadTOTPPending("moose")
	if !ok {
		t.Fatal("expected a pending 2FA setup")
	}
	post("confirm_2fa", "code", testTOTPCode(pending.Secret, time.Now()))
	post("acknowledge_codes")
	if u, _ := authSvc.GetUser("moose"); !u.Verified {
		t.Fatal("acked codes should verify the account")
	}
	post("disable_2fa")
	if u, _ := authSvc.GetUser("moose"); u.Verified {
		t.Fatal("disabling 2FA with no confirmed email should unverify the account")
	}
	post("enable_2fa")
	pending, ok = app.loadTOTPPending("moose")
	if !ok {
		t.Fatal("expected a pending 2FA setup")
	}
	post("confirm_2fa", "code", testTOTPCode(pending.Secret, time.Now()))
	app.persistRecoveryEmail("moose", recoveryEmailState{Email: "moose@example.com", VerifiedAt: time.Now()})
	post("acknowledge_codes")
	post("disable_2fa")
	if u, _ := authSvc.GetUser("moose"); !u.Verified {
		t.Fatal("disabling 2FA with a confirmed email should keep the account verified")
	}
}

func TestDisable2FAPreservesIndependentVerification(t *testing.T) {
	authSvc := auth.NewService(repository.NewInMemoryUserRepository())
	if _, err := authSvc.Register("moose", "password123"); err != nil {
		t.Fatal(err)
	}
	if err := authSvc.SetVerified("moose", true); err != nil {
		t.Fatal(err)
	}
	app := &webApp{authSvc: authSvc, adminRepo: repository.NewInMemoryAdminRepository(), siteName: "ResoNET BBS", sessions: map[string]sessionState{}}
	sess, ok := app.createSession("moose")
	if !ok {
		t.Fatal("session creation failed")
	}
	post := func(action string, pairs ...string) {
		form := url.Values{"action": {action}, "csrf_token": {app.sessions[sess].csrf}}
		for i := 0; i+1 < len(pairs); i += 2 {
			form.Set(pairs[i], pairs[i+1])
		}
		req := httptest.NewRequest(http.MethodPost, "/settings", strings.NewReader(form.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req.AddCookie(&http.Cookie{Name: "wolfbbs_session", Value: sess})
		rr := httptest.NewRecorder()
		app.handleSettings(rr, req)
		if rr.Code != http.StatusFound {
			t.Fatalf("%s status = %d body=%s", action, rr.Code, rr.Body.String())
		}
	}
	post("disable_2fa")
	if u, _ := authSvc.GetUser("moose"); !u.Verified {
		t.Fatal("disabling 2FA must not revoke verification that never came from codes")
	}
}

func TestConfirmClearsStaleCodesAck(t *testing.T) {
	authSvc := auth.NewService(repository.NewInMemoryUserRepository())
	if _, err := authSvc.Register("moose", "password123"); err != nil {
		t.Fatal(err)
	}
	app := &webApp{authSvc: authSvc, adminRepo: repository.NewInMemoryAdminRepository(), siteName: "ResoNET BBS"}
	enableAndConfirm := func() {
		if _, err := app.handleTOTPAction(totpRequest(""), "moose", "enable_2fa"); err != nil {
			t.Fatalf("enable: %v", err)
		}
		pending, ok := app.loadTOTPPending("moose")
		if !ok {
			t.Fatal("pending secret should exist after enable")
		}
		if _, err := app.handleTOTPAction(totpRequest(testTOTPCode(pending.Secret, time.Now())), "moose", "confirm_2fa"); err != nil {
			t.Fatalf("confirm: %v", err)
		}
	}
	enableAndConfirm()
	if _, err := app.handleTOTPAction(totpRequest(""), "moose", "acknowledge_codes"); err != nil {
		t.Fatalf("acknowledge: %v", err)
	}
	enableAndConfirm()
	if app.codesAcknowledged("moose") {
		t.Fatal("fresh codes from a new setup must not inherit a previous acknowledgement")
	}
}
