package supabase

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func tokenServer(t *testing.T, created time.Time) string {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, `[{"access_token":"from_supabase","user_id":"AB1234","created_at":%q}]`, created.UTC().Format(time.RFC3339))
	}))
	t.Cleanup(srv.Close)
	return srv.URL
}

func TestResolveAccessTokenPrefersEnv(t *testing.T) {
	t.Setenv("KITE_ACCESS_TOKEN", "from_env")
	t.Setenv("SUPABASE_URL", tokenServer(t, time.Now()))
	t.Setenv("SUPABASE_SERVICE_KEY", "k")
	if tok, err := ResolveAccessToken(context.Background()); err != nil || tok != "from_env" {
		t.Errorf("got %q, %v", tok, err)
	}
}

func TestResolveAccessTokenFallsBackToSupabase(t *testing.T) {
	t.Setenv("KITE_ACCESS_TOKEN", "")
	t.Setenv("SUPABASE_URL", tokenServer(t, time.Now()))
	t.Setenv("SUPABASE_SERVICE_KEY", "k")
	if tok, err := ResolveAccessToken(context.Background()); err != nil || tok != "from_supabase" {
		t.Errorf("got %q, %v", tok, err)
	}
}

func TestResolveAccessTokenRejectsExpiredSupabaseToken(t *testing.T) {
	t.Setenv("KITE_ACCESS_TOKEN", "")
	t.Setenv("SUPABASE_URL", tokenServer(t, time.Now().Add(-48*time.Hour)))
	t.Setenv("SUPABASE_SERVICE_KEY", "k")
	if _, err := ResolveAccessToken(context.Background()); err == nil || !strings.Contains(err.Error(), "Log in to Kite") {
		t.Errorf("err = %v, want a hint to log in on the website", err)
	}
}

func TestResolveAccessTokenUnconfigured(t *testing.T) {
	t.Setenv("KITE_ACCESS_TOKEN", "")
	t.Setenv("SUPABASE_URL", "")
	t.Setenv("SUPABASE_SERVICE_KEY", "")
	if _, err := ResolveAccessToken(context.Background()); err == nil {
		t.Error("expected an error when neither source is configured")
	}
}
