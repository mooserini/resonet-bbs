package main

import (
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/mail"
	"net/url"
	"os"
	"strings"
	"time"
)

// Recovery email: an off-board address a caller confirms so password resets
// have somewhere safe to go. Addresses on the board's own domain are refused,
// because losing the BBS account would also lose that mailbox.

const (
	sysSettingRecoveryEmailRoot = "account.recovery_email."
	recoveryEmailConfirmTTL     = 24 * time.Hour
	recoveryEmailConfirmPath    = "/settings/recovery-email/confirm"
)

type recoveryEmailState struct {
	Email          string    `json:"email,omitempty"`
	VerifiedAt     time.Time `json:"verified_at,omitempty"`
	PendingEmail   string    `json:"pending_email,omitempty"`
	TokenHash      string    `json:"token_hash,omitempty"`
	TokenExpiresAt time.Time `json:"token_expires_at,omitempty"`
}

func (s recoveryEmailState) verified() bool {
	return strings.TrimSpace(s.Email) != "" && !s.VerifiedAt.IsZero()
}

func recoveryEmailSettingKey(handle string) string {
	handle = normalizeHandleKey(handle)
	if handle == "" {
		return ""
	}
	return sysSettingRecoveryEmailRoot + handle
}

func (a *webApp) loadRecoveryEmail(handle string) recoveryEmailState {
	var state recoveryEmailState
	if a.adminRepo == nil {
		return state
	}
	key := recoveryEmailSettingKey(handle)
	if key == "" {
		return state
	}
	raw, err := a.adminRepo.GetSystemSetting(key)
	if err != nil || strings.TrimSpace(raw) == "" {
		return state
	}
	if err := json.Unmarshal([]byte(raw), &state); err != nil {
		a.addAppError("recovery_email", fmt.Errorf("decode recovery email for %s: %w", handle, err))
		return recoveryEmailState{}
	}
	return state
}

func (a *webApp) persistRecoveryEmail(handle string, state recoveryEmailState) {
	key := recoveryEmailSettingKey(handle)
	if key == "" {
		return
	}
	if state.Email == "" && state.PendingEmail == "" {
		a.persistSystemSetting(key, "")
		return
	}
	raw, err := json.Marshal(state)
	if err != nil {
		a.addAppError("recovery_email", fmt.Errorf("encode recovery email for %s: %w", handle, err))
		return
	}
	a.persistSystemSetting(key, string(raw))
}

// recoveryEmailBlockedDomains lists domains that can never be a recovery
// address: the board host, its parent domain, and anything in
// WOLFBBS_RECOVERY_EMAIL_BLOCKED_DOMAINS.
func (a *webApp) recoveryEmailBlockedDomains() []string {
	out := []string{}
	add := func(domain string) {
		domain = strings.Trim(strings.ToLower(strings.TrimSpace(domain)), ".")
		if domain == "" || domain == "localhost" || !strings.Contains(domain, ".") {
			return
		}
		for _, existing := range out {
			if existing == domain {
				return
			}
		}
		out = append(out, domain)
	}
	host := strings.ToLower(sanitizedConfiguredHost(a.siteHost()))
	if h, _, found := strings.Cut(host, ":"); found {
		host = h
	}
	add(host)
	if labels := strings.Split(host, "."); len(labels) > 2 {
		add(strings.Join(labels[len(labels)-2:], "."))
	}
	for _, domain := range strings.Split(os.Getenv("WOLFBBS_RECOVERY_EMAIL_BLOCKED_DOMAINS"), ",") {
		add(domain)
	}
	return out
}

func emailDomainBlocked(address string, blocked []string) bool {
	at := strings.LastIndex(address, "@")
	if at < 0 {
		return false
	}
	domain := strings.Trim(strings.ToLower(address[at+1:]), ".")
	for _, b := range blocked {
		if domain == b || strings.HasSuffix(domain, "."+b) {
			return true
		}
	}
	return false
}

func (a *webApp) validateRecoveryEmail(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", errors.New("Enter an email address.")
	}
	addr, err := mail.ParseAddress(raw)
	if err != nil || addr.Address != raw || !strings.Contains(addr.Address, "@") {
		return "", errors.New("That doesn't look like a valid email address.")
	}
	email := strings.ToLower(addr.Address)
	if emailDomainBlocked(email, a.recoveryEmailBlockedDomains()) {
		return "", errors.New("Your BBS mailbox can't be your recovery email. If you lose access to your account you lose that mailbox too. Use a personal address outside this board.")
	}
	return email, nil
}

func hashRecoveryToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

func (a *webApp) sendRecoveryEmailConfirmation(r *http.Request, handle, email, token string) error {
	emailGateway := a.activeEmailGateway()
	if emailGateway == nil || !emailGateway.Enabled() {
		return errors.New("outbound email is not configured")
	}
	base := normalizedPublicURL(a.publicBaseURL)
	if base == "" {
		base = a.activityPubBase(r)
	}
	link := strings.TrimRight(base, "/") + recoveryEmailConfirmPath +
		"?handle=" + url.QueryEscape(handle) + "&token=" + url.QueryEscape(token)
	subject := a.siteDisplayName() + ": confirm your recovery email"
	body := "Someone (hopefully you) asked to use this address as the recovery email for the " +
		a.siteDisplayName() + " account \"" + handle + "\".\n\n" +
		"Open this link within 24 hours to confirm it:\n" + link + "\n\n" +
		"Password reset links for that account will be sent here.\n" +
		"If this wasn't you, ignore this message and nothing changes."
	return emailGateway.SendOutbound("wolfbbs-account", []string{email}, subject, body)
}

// handleRecoveryEmailAction handles the settings form actions. It returns the
// notice to show, or an error to show instead.
func (a *webApp) handleRecoveryEmailAction(r *http.Request, handle, action string) (string, error) {
	state := a.loadRecoveryEmail(handle)
	switch action {
	case "set_recovery_email":
		email, err := a.validateRecoveryEmail(r.FormValue("recovery_email"))
		if err != nil {
			return "", err
		}
		if state.verified() && state.Email == email {
			return "That address is already your confirmed recovery email.", nil
		}
		token := randomToken(40)
		state.PendingEmail = email
		state.TokenHash = hashRecoveryToken(token)
		state.TokenExpiresAt = time.Now().UTC().Add(recoveryEmailConfirmTTL)
		a.persistRecoveryEmail(handle, state)
		if err := a.sendRecoveryEmailConfirmation(r, handle, email, token); err != nil {
			a.addAppError("recovery_email", fmt.Errorf("send confirmation for %s: %w", handle, err))
			return "Saved " + email + " as pending, but this board can't send email yet, so the confirmation link wasn't sent. Ask the sysop to finish email setup, then save it again.", nil
		}
		return "Check " + email + " for a confirmation link. It expires in 24 hours.", nil
	case "remove_recovery_email":
		a.persistRecoveryEmail(handle, recoveryEmailState{})
		return "Recovery email removed. Password resets can't reach you until you add one again.", nil
	}
	return "", errors.New("Unknown recovery email action.")
}

func (a *webApp) handleRecoveryEmailConfirm(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	handle := strings.TrimSpace(r.URL.Query().Get("handle"))
	token := strings.TrimSpace(r.URL.Query().Get("token"))
	state := a.loadRecoveryEmail(handle)
	valid := handle != "" && token != "" && state.PendingEmail != "" && state.TokenHash != "" &&
		time.Now().UTC().Before(state.TokenExpiresAt) &&
		subtle.ConstantTimeCompare([]byte(hashRecoveryToken(token)), []byte(state.TokenHash)) == 1
	title := a.siteDisplayName() + " recovery email"
	if !valid {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`<!doctype html><html lang="en"><head><meta charset="utf-8"><title>` + htmlEscape(title) + `</title></head><body><h1>Link expired or invalid</h1><p>This confirmation link has expired, was already used, or was replaced by a newer one. Sign in and add your recovery email again from <a href="/settings">settings</a>.</p></body></html>`))
		return
	}
	confirmed := state.PendingEmail
	a.persistRecoveryEmail(handle, recoveryEmailState{Email: confirmed, VerifiedAt: time.Now().UTC()})
	// Proving control of an outside mailbox is what "verified" gates on
	// (external email), so a confirmed recovery email verifies the account.
	if a.authSvc != nil {
		if err := a.authSvc.SetVerified(handle, true); err != nil {
			a.addAppError("recovery_email", fmt.Errorf("mark %s verified: %w", handle, err))
		}
	}
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(`<!doctype html><html lang="en"><head><meta charset="utf-8"><title>` + htmlEscape(title) + `</title></head><body><h1>Recovery email confirmed</h1><p>Password reset links for <strong>` + htmlEscape(handle) + `</strong> will now go to <strong>` + htmlEscape(confirmed) + `</strong>.</p><p><a href="/settings">Back to settings</a></p></body></html>`))
}

// passwordResetRecipientFor picks where a reset link may be sent: the
// confirmed recovery email, else an email-form handle that isn't on the
// board's own domain.
func (a *webApp) passwordResetRecipientFor(handle string) string {
	if state := a.loadRecoveryEmail(handle); state.verified() {
		return state.Email
	}
	recipient := passwordResetRecipient(handle)
	if recipient == "" || emailDomainBlocked(strings.ToLower(recipient), a.recoveryEmailBlockedDomains()) {
		return ""
	}
	return recipient
}

func (a *webApp) recoveryEmailSettingsBlock(handle, csrf string) string {
	state := a.loadRecoveryEmail(handle)
	blocked := a.recoveryEmailBlockedDomains()
	var b strings.Builder
	b.WriteString(`<h2 id="recovery-email">Recovery Email</h2>`)
	b.WriteString(`<p class="wolfbbs-callout"><strong>Important:</strong> use a personal email you control outside this board. `)
	if len(blocked) > 0 {
		b.WriteString(`Addresses at <strong>` + htmlEscape(strings.Join(blocked, ", ")) + `</strong> are refused, because `)
	} else {
		b.WriteString(`Your BBS mailbox can't be used, because `)
	}
	b.WriteString(`if you're locked out of your account, you're locked out of that mailbox too.</p>`)
	switch {
	case state.verified():
		b.WriteString(`<p>Confirmed recovery email: <strong>` + htmlEscape(state.Email) + `</strong> (since ` + state.VerifiedAt.Local().Format("2006-01-02") + `)</p>`)
	default:
		b.WriteString(`<p><strong>No recovery email confirmed.</strong> If you forget your password, there's nowhere to send a reset link.</p>`)
	}
	if state.PendingEmail != "" && time.Now().UTC().Before(state.TokenExpiresAt) {
		b.WriteString(`<p>Waiting for confirmation: <strong>` + htmlEscape(state.PendingEmail) + `</strong>. Check that inbox for the link.</p>`)
	}
	b.WriteString(`<form method="POST" action="/settings"><input type="hidden" name="action" value="set_recovery_email">` + csrf +
		`<label>Recovery email <input name="recovery_email" type="email" autocomplete="email" size="40" required></label> ` +
		`<button type="submit">Send confirmation link</button></form>`)
	if state.Email != "" || state.PendingEmail != "" {
		b.WriteString(`<form method="POST" action="/settings"><input type="hidden" name="action" value="remove_recovery_email">` + csrf +
			`<button type="submit">Remove recovery email</button></form>`)
	}
	return b.String()
}
