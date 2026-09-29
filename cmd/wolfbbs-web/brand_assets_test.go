package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestBrandIconsServeAndInject(t *testing.T) {
	for _, tc := range []struct {
		path, contentType string
		handler           http.Handler
	}{
		{"/favicon.ico", "image/x-icon", faviconHandler("favicon.ico", "image/x-icon")},
		{"/apple-touch-icon.png", "image/png", faviconHandler("icon-180.png", "image/png")},
		{"/assets/icons/icon-192.png", "image/png", brandAssetHandler()},
		{"/assets/fonts/Web437_ATT_PC6300.woff", "font/woff", brandAssetHandler()},
	} {
		rec := httptest.NewRecorder()
		tc.handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, tc.path, nil))
		if rec.Code != http.StatusOK || rec.Body.Len() == 0 {
			t.Fatalf("%s: got %d with %d bytes", tc.path, rec.Code, rec.Body.Len())
		}
		if got := rec.Header().Get("Content-Type"); !strings.HasPrefix(got, tc.contentType) {
			t.Fatalf("%s: content type %q, want %q", tc.path, got, tc.contentType)
		}
	}

	page := injectModernUI(`<!doctype html><html><head><title>x</title></head><body></body></html>`)
	if !strings.Contains(page, `<link rel="icon" href="/favicon.ico"`) || !strings.Contains(page, `rel="apple-touch-icon"`) {
		t.Fatal("expected icon links in injected head")
	}
	own := injectModernUI(`<!doctype html><html><head><link rel="icon" href="/custom.ico"></head><body></body></html>`)
	if strings.Contains(own, `href="/favicon.ico"`) {
		t.Fatal("pages with their own icon must keep it")
	}
}
