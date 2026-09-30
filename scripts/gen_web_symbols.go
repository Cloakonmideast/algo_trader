//go:build ignore

// Regenerates web/src/lib/nse-symbols.json, the Trades tab's stock suggestions,
// from NSE's official list of listed equities. Run from the repo root:
//
//	go run ./scripts/gen_web_symbols.go
package main

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"regexp"
	"sort"
	"strings"
	"time"
)

const (
	equityListURL = "https://archives.nseindia.com/content/equities/EQUITY_L.csv"
	outPath       = "web/src/lib/nse-symbols.json"
)

// symbolRE is the website's symbol rule (web/src/lib/trades.ts SYMBOL_RE).
var symbolRE = regexp.MustCompile(`^[A-Z0-9&-]{1,30}$`)

func main() {
	req, err := http.NewRequest(http.MethodGet, equityListURL, nil)
	if err != nil {
		fail("build request: %v", err)
	}
	// NSE refuses requests without a browser-like User-Agent.
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/125.0.0.0 Safari/537.36")
	resp, err := (&http.Client{Timeout: 30 * time.Second}).Do(req)
	if err != nil {
		fail("download %s: %v", equityListURL, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		fail("download %s: HTTP %d", equityListURL, resp.StatusCode)
	}

	records, err := csv.NewReader(resp.Body).ReadAll()
	if err != nil {
		fail("parse CSV: %v", err)
	}
	if len(records) < 2 || strings.TrimSpace(records[0][0]) != "SYMBOL" {
		fail("unexpected CSV header %v", records[0])
	}

	seen := map[string]bool{}
	var out [][2]string
	for _, r := range records[1:] {
		if len(r) < 2 {
			continue
		}
		symbol := strings.ToUpper(strings.TrimSpace(r[0]))
		name := strings.TrimSpace(r[1])
		if !symbolRE.MatchString(symbol) || name == "" || seen[symbol] {
			continue
		}
		seen[symbol] = true
		out = append(out, [2]string{symbol, name})
	}
	sort.Slice(out, func(i, j int) bool { return out[i][0] < out[j][0] })

	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false) // keep "M&M" readable instead of "M&M"
	if err := enc.Encode(out); err != nil {
		fail("encode: %v", err)
	}
	if err := os.WriteFile(outPath, buf.Bytes(), 0o644); err != nil {
		fail("write %s: %v", outPath, err)
	}
	fmt.Printf("wrote %d symbols to %s\n", len(out), outPath)
}

func fail(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "gen_web_symbols: "+format+"\n", args...)
	os.Exit(1)
}
