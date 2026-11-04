package web

import (
	"strings"
	"testing"
)

func TestThemeInitializesBeforeStylesWithoutWeakeningCSP(t *testing.T) {
	f := fixtureNav(t)
	server := f.server(t)
	for _, target := range []string{"/activity", "/teams", "/roles", "/profiles", "/unknown-screen"} {
		t.Run(target, func(t *testing.T) {
			response, body := f.get(t, server, target)
			const boot = `<script src="/static/lib/theme-init.js"></script>`
			firstStyle := strings.Index(body, `<link rel="stylesheet"`)
			if strings.Count(body, boot) != 1 || firstStyle < 0 || strings.Index(body, boot) > firstStyle {
				t.Fatal("synchronous theme initializer must precede every stylesheet")
			}
			if got := response.Header.Get("Content-Security-Policy"); got != "default-src 'self'" {
				t.Fatalf("CSP changed: %q", got)
			}
			if !strings.Contains(body, `aria-controls="theme-menu" disabled`) {
				t.Fatal("theme control must stay disabled until initialization is available")
			}
		})
	}
}
