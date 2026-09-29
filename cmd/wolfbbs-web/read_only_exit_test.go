package main

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"wolfbbs/internal/auth"
	"wolfbbs/internal/config"
	"wolfbbs/internal/repository"
)

func newReadOnlyTestApp(t *testing.T) (*webApp, string, string) {
	t.Helper()
	userRepo := repository.NewInMemoryUserRepository()
	authSvc := auth.NewService(userRepo)
	user, err := authSvc.Register("sysop", "password123")
	if err != nil {
		t.Fatalf("register sysop: %v", err)
	}
	if err := authSvc.SetRole(user.Handle, roleAdmin); err != nil {
		t.Fatalf("set sysop role: %v", err)
	}
	app := &webApp{
		authSvc:      authSvc,
		adminRepo:    repository.NewInMemoryAdminRepository(),
		sessions:     map[string]sessionState{},
		runtimeCfg:   config.DefaultRuntime(),
		siteName:     "WolfBBS",
		siteHostname: "localhost",
		menuRoot:     t.TempDir(),
		readOnly:     true,
	}
	sid, ok := app.createSession(user.Handle)
	if !ok {
		t.Fatal("create session failed")
	}
	return app, sid, app.sessions[sid].csrf
}

func postAdminConfig(app *webApp, sid string, form url.Values) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPost, "/admin/config", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.AddCookie(&http.Cookie{Name: "wolfbbs_session", Value: sid})
	rr := httptest.NewRecorder()
	app.mustBeRole(roleAdmin, http.HandlerFunc(app.handleAdminConfig)).ServeHTTP(rr, req)
	return rr
}

func TestReadOnlyBlocksAdminSavesButNotExit(t *testing.T) {
	app, sid, csrf := newReadOnlyTestApp(t)

	blocked := postAdminConfig(app, sid, url.Values{"action": {"save_text"}, "motd": {"hi"}, "csrf_token": {csrf}})
	if blocked.Code != http.StatusForbidden {
		t.Fatalf("expected normal saves to stay blocked in read-only mode, got %d", blocked.Code)
	}

	noCSRF := postAdminConfig(app, sid, url.Values{"action": {"exit_read_only"}})
	if noCSRF.Code != http.StatusForbidden || !app.readOnly {
		t.Fatalf("exit without CSRF must fail, got %d readOnly=%t", noCSRF.Code, app.readOnly)
	}

	exit := postAdminConfig(app, sid, url.Values{"action": {"exit_read_only"}, "csrf_token": {csrf}})
	if exit.Code != http.StatusFound || app.readOnly {
		t.Fatalf("expected exit to succeed, got %d readOnly=%t", exit.Code, app.readOnly)
	}
	if v, err := app.adminRepo.GetSystemSetting(sysSettingReadOnly); err != nil || v != "false" {
		t.Fatalf("expected persisted read_only=false, got %q err=%v", v, err)
	}

	saved := postAdminConfig(app, sid, url.Values{"action": {"save_text"}, "motd": {"hi"}, "csrf_token": {csrf}})
	if saved.Code != http.StatusFound {
		t.Fatalf("expected saves to work after exit, got %d", saved.Code)
	}
}

func TestReadOnlyExitNeedsAdmin(t *testing.T) {
	app, _, _ := newReadOnlyTestApp(t)
	caller, err := app.authSvc.Register("caller", "password123")
	if err != nil {
		t.Fatalf("register caller: %v", err)
	}
	sid, _ := app.createSession(caller.Handle)
	rr := postAdminConfig(app, sid, url.Values{"action": {"exit_read_only"}, "csrf_token": {app.sessions[sid].csrf}})
	if rr.Code == http.StatusFound && !app.readOnly {
		t.Fatal("a non-admin must not be able to turn off read-only mode")
	}
	if !app.readOnly {
		t.Fatal("read-only mode changed for a non-admin request")
	}
}
