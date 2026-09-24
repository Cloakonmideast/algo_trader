package supabase

import (
	"context"
	"errors"
	"os"
	"strings"
	"time"

	"algotrader/internal/domain"
)

// ResolveAccessToken finds a Kite access token for the offline tools
// (backfill, scanner, backtest symbol refresh): KITE_ACCESS_TOKEN if set,
// otherwise today's token saved by the website's "Log in to Kite" button.
func ResolveAccessToken(ctx context.Context) (string, error) {
	if t := strings.TrimSpace(os.Getenv("KITE_ACCESS_TOKEN")); t != "" {
		return t, nil
	}
	c := FromEnv()
	if c == nil {
		return "", errors.New("no Kite access token: set KITE_ACCESS_TOKEN, or SUPABASE_URL and SUPABASE_SERVICE_KEY to use the website's login")
	}
	s, err := c.LatestToken(ctx)
	if err != nil {
		return "", err
	}
	if !s.Found || !domain.TokenCurrent(s.CreatedAt, time.Now()) {
		return "", errors.New("no current Kite token in Supabase — click \"Log in to Kite\" on the website first")
	}
	return s.AccessToken, nil
}
