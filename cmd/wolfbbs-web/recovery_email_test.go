package main

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"wolfbbs/internal/repository"
)

func newRecoveryEmailTestApp() *webApp {
	return &webApp{
		adminRepo:    repository.NewInMemoryAdminRepository(),
		siteHostname: "reso.getadongle.com",
	}
}

func TestRecoveryEmailBlocksBoardDomains(t *testing.T) {
	t.Setenv("WOLFBBS_RECOVERY_EMAIL_BLOCKED_DOMAINS", "example.net")
	app := newRecoveryEmailTestApp()

	for _, bad := range []string{
		"moose@reso.getadongle.com",
		"moose@getadongle.com",
		"moose@mail.getadongle.com",
		"MOOSE@Reso.GetADongle.com",
		"someone@example.net",
		"not-an-email",
		"Moose <moose@gmail.com>",
		"",
	} {
		if _, err := app.validateRecoveryEmail(bad); err == nil {
			t.Errorf("expected %q to be rejected", bad)
		}
	}
	for _, good := range []string{"moose@gmail.com", "tom@notgetadongle.com"} {
		if _, err := app.validateRecoveryEmail(good); err != nil {
			t.Errorf("expected %q to be accepted, got %v", good, err)
		}
	}
}

func TestRecoveryEmailConfirmFlow(t *testing.T) {
	app := newRecoveryEmailTestApp()
	token := "known-token"
	app.persistRecoveryEmail("moose", recoveryEmailState{
		PendingEmail:   "moose@gmail.com",
		TokenHash:      hashRecoveryToken(token),
		TokenExpiresAt: time.Now().UTC().Add(time.Hour),
	})

	if got := app.passwordResetRecipientFor("moose"); got != "" {
		t.Fatalf("pending address must not receive resets, got %q", got)
	}

	bad := httptest.NewRecorder()
	app.handleRecoveryEmailConfirm(bad, httptest.NewRequest(http.MethodGet, recoveryEmailConfirmPath+"?handle=moose&token=wrong", nil))
	if bad.Code != http.StatusBadRequest {
		t.Fatalf("wrong token should fail, got %d", bad.Code)
	}

	ok := httptest.NewRecorder()
	q := url.Values{"handle": {"moose"}, "token": {token}}
	app.handleRecoveryEmailConfirm(ok, httptest.NewRequest(http.MethodGet, recoveryEmailConfirmPath+"?"+q.Encode(), nil))
	if ok.Code != http.StatusOK || !strings.Contains(ok.Body.String(), "confirmed") {
		t.Fatalf("valid token should confirm, got %d %s", ok.Code, ok.Body.String())
	}
	if got := app.passwordResetRecipientFor("moose"); got != "moose@gmail.com" {
		t.Fatalf("expected confirmed recovery email, got %q", got)
	}

	reuse := httptest.NewRecorder()
	app.handleRecoveryEmailConfirm(reuse, httptest.NewRequest(http.MethodGet, recoveryEmailConfirmPath+"?"+q.Encode(), nil))
	if reuse.Code != http.StatusBadRequest {
		t.Fatalf("token must be single-use, got %d", reuse.Code)
	}
}

func TestRecoveryEmailExpiredToken(t *testing.T) {
	app := newRecoveryEmailTestApp()
	app.persistRecoveryEmail("moose", recoveryEmailState{
		PendingEmail:   "moose@gmail.com",
		TokenHash:      hashRecoveryToken("t"),
		TokenExpiresAt: time.Now().UTC().Add(-time.Minute),
	})
	rec := httptest.NewRecorder()
	app.handleRecoveryEmailConfirm(rec, httptest.NewRequest(http.MethodGet, recoveryEmailConfirmPath+"?handle=moose&token=t", nil))
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expired token should fail, got %d", rec.Code)
	}
}

func TestPasswordResetSkipsBoardDomainHandles(t *testing.T) {
	app := newRecoveryEmailTestApp()
	if got := app.passwordResetRecipientFor("moose@reso.getadongle.com"); got != "" {
		t.Fatalf("board-domain handle must not receive resets, got %q", got)
	}
	if got := app.passwordResetRecipientFor("moose@gmail.com"); got != "moose@gmail.com" {
		t.Fatalf("email-form handle off the board domain should still work, got %q", got)
	}
}

func TestRecoveryEmailSetWithoutSMTPStaysPending(t *testing.T) {
	app := newRecoveryEmailTestApp()
	form := url.Values{"recovery_email": {"moose@gmail.com"}}
	req := httptest.NewRequest(http.MethodPost, "/settings", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	msg, err := app.handleRecoveryEmailAction(req, "moose", "set_recovery_email")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(msg, "can't send email yet") {
		t.Fatalf("expected a no-SMTP notice, got %q", msg)
	}
	state := app.loadRecoveryEmail("moose")
	if state.PendingEmail != "moose@gmail.com" || state.verified() {
		t.Fatalf("expected pending, unverified state, got %+v", state)
	}
}
