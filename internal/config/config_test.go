package config

import (
	"strings"
	"testing"
)

func baseEnv(t *testing.T) {
	t.Helper()
	for k, v := range map[string]string{
		"KITE_API_KEY": "key", "DISCORD_WEBHOOK_URL": "https://discord/webhook",
		"SCAN_NIFTY500": "true", "KITE_ACCESS_TOKEN": "", "SUPABASE_URL": "", "SUPABASE_SERVICE_KEY": "",
		"DISCORD_BOT_TOKEN": "", "DISCORD_APPLICATION_ID": "", "DISCORD_PUBLIC_KEY": "", "DISCORD_CHANNEL_ID": "",
		"SITE_URL": "",
	} {
		t.Setenv(k, v)
	}
}

func mustPanic(t *testing.T, contains string, f func()) {
	t.Helper()
	defer func() {
		r := recover()
		if r == nil || !strings.Contains(r.(string), contains) {
			t.Fatalf("panic = %v, want one mentioning %q", r, contains)
		}
	}()
	f()
}

func TestSupabaseModeNeedsNoAccessTokenOrBotSettings(t *testing.T) {
	baseEnv(t)
	t.Setenv("SUPABASE_URL", "https://x.supabase.co")
	t.Setenv("SUPABASE_SERVICE_KEY", "secret")
	t.Setenv("SITE_URL", "https://algo.vercel.app")
	cfg := Load()
	if !cfg.UseSupabase() || cfg.KiteAccessToken != "" || cfg.SiteURL != "https://algo.vercel.app" {
		t.Errorf("cfg = %+v", cfg)
	}
}

func TestWithoutSupabaseTheAccessTokenIsRequired(t *testing.T) {
	baseEnv(t)
	mustPanic(t, "KITE_ACCESS_TOKEN", func() { Load() })

	t.Setenv("KITE_ACCESS_TOKEN", "tok")
	if cfg := Load(); cfg.UseSupabase() || cfg.KiteAccessToken != "tok" {
		t.Errorf("cfg = %+v", cfg)
	}
}

func TestHalfConfiguredSupabaseIsAnError(t *testing.T) {
	baseEnv(t)
	t.Setenv("SUPABASE_URL", "https://x.supabase.co")
	mustPanic(t, "SUPABASE_SERVICE_KEY", func() { Load() })
}
