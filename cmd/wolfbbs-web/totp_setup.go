package main

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"rsc.io/qr"

	"wolfbbs/internal/auth"
)

// Two-step TOTP setup. The old flow switched 2FA on in one click without ever
// showing the secret, which locked people out. Now "set up" only creates a
// pending secret; 2FA turns on after the caller proves their app works.

const (
	sysSettingTOTPPendingRoot = "account.totp_pending."
	totpPendingTTL            = 15 * time.Minute
)

type totpPending struct {
	Secret    string    `json:"secret"`
	CreatedAt time.Time `json:"created_at"`
}

func totpPendingKey(handle string) string {
	handle = normalizeHandleKey(handle)
	if handle == "" {
		return ""
	}
	return sysSettingTOTPPendingRoot + handle
}

func (a *webApp) loadTOTPPending(handle string) (totpPending, bool) {
	var p totpPending
	key := totpPendingKey(handle)
	if a.adminRepo == nil || key == "" {
		return p, false
	}
	raw, err := a.adminRepo.GetSystemSetting(key)
	if err != nil || strings.TrimSpace(raw) == "" {
		return p, false
	}
	if json.Unmarshal([]byte(raw), &p) != nil || p.Secret == "" {
		return totpPending{}, false
	}
	if time.Since(p.CreatedAt) > totpPendingTTL {
		return totpPending{}, false
	}
	return p, true
}

func (a *webApp) storeTOTPPending(handle string, p *totpPending) {
	key := totpPendingKey(handle)
	if key == "" {
		return
	}
	if p == nil {
		a.persistSystemSetting(key, "")
		return
	}
	raw, err := json.Marshal(p)
	if err != nil {
		return
	}
	a.persistSystemSetting(key, string(raw))
}

func totpProvisioningURI(issuer, handle, secret string) string {
	issuer = strings.TrimSpace(issuer)
	label := url.PathEscape(issuer + ":" + handle)
	q := url.Values{}
	q.Set("secret", secret)
	q.Set("issuer", issuer)
	q.Set("algorithm", "SHA1")
	q.Set("digits", "6")
	q.Set("period", "30")
	return "otpauth://totp/" + label + "?" + q.Encode()
}

func totpQRDataURI(uri string) string {
	code, err := qr.Encode(uri, qr.M)
	if err != nil {
		return ""
	}
	code.Scale = 6
	return "data:image/png;base64," + base64.StdEncoding.EncodeToString(code.PNG())
}

func groupSecret(secret string) string {
	var b strings.Builder
	for i, r := range secret {
		if i > 0 && i%4 == 0 {
			b.WriteByte(' ')
		}
		b.WriteRune(r)
	}
	return b.String()
}

// handleTOTPAction runs the 2FA settings actions and returns the notice to show.
func (a *webApp) handleTOTPAction(r *http.Request, handle, action string) (string, error) {
	switch action {
	case "enable_2fa":
		secret, err := auth.GenerateTOTPSecret()
		if err != nil {
			return "", errors.New("2FA setup failed.")
		}
		a.storeTOTPPending(handle, &totpPending{Secret: secret, CreatedAt: time.Now().UTC()})
		return "Scan the code below with your authenticator app, then enter the 6-digit code it shows. 2FA is not on yet.", nil
	case "confirm_2fa":
		pending, ok := a.loadTOTPPending(handle)
		if !ok {
			return "", errors.New("2FA setup expired. Start again.")
		}
		if !auth.VerifyTOTP(pending.Secret, r.FormValue("code"), time.Now()) {
			return "", errors.New("That code didn't match. Check your app's clock and try the current code. 2FA is still off.")
		}
		codes, err := auth.GenerateRecoveryCodes(8)
		if err != nil {
			return "", errors.New("2FA setup failed.")
		}
		if err := a.authSvc.SetTOTPSecret(handle, pending.Secret); err != nil {
			return "", errors.New("2FA setup failed.")
		}
		_ = a.authSvc.SetRecoveryCodes(handle, codes)
		a.storeTOTPPending(handle, nil)
		return "2FA is on. Save your recovery codes below somewhere safe: each one gets you in once if you lose your phone.", nil
	case "cancel_2fa_setup":
		a.storeTOTPPending(handle, nil)
		return "2FA setup cancelled. 2FA is still off.", nil
	}
	return "", fmt.Errorf("unknown 2FA action %q", action)
}

func (a *webApp) totpSetupBlock(handle, csrf string) string {
	pending, ok := a.loadTOTPPending(handle)
	if !ok {
		return `<p>2FA is currently off.</p>` +
			`<form method="POST" action="/settings"><input type="hidden" name="action" value="enable_2fa">` + csrf +
			`<button type="submit">Set up authenticator app</button></form>`
	}
	uri := totpProvisioningURI(a.siteDisplayName(), handle, pending.Secret)
	var b strings.Builder
	b.WriteString(`<p><strong>Finish setting up 2FA.</strong> It is not on until you enter a code.</p>`)
	if img := totpQRDataURI(uri); img != "" {
		b.WriteString(`<p><img src="` + img + `" width="240" height="240" alt="2FA setup QR code" style="background:#fff;padding:8px"></p>`)
	}
	b.WriteString(`<p>Can't scan? Enter this key in your app: <code>` + htmlEscape(groupSecret(pending.Secret)) + `</code> (time-based, 6 digits). On a phone you can also <a href="` + htmlEscape(uri) + `">open it in your authenticator</a>.</p>`)
	b.WriteString(`<form method="POST" action="/settings"><input type="hidden" name="action" value="confirm_2fa">` + csrf +
		`<label>Code from your app <input name="code" inputmode="numeric" autocomplete="one-time-code" pattern="[0-9]{6}" maxlength="6" required></label> ` +
		`<button type="submit">Turn on 2FA</button></form>`)
	b.WriteString(`<form method="POST" action="/settings"><input type="hidden" name="action" value="cancel_2fa_setup">` + csrf +
		`<button type="submit">Cancel</button></form>`)
	return b.String()
}
