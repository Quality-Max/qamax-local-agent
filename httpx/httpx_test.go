package httpx

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	receipt "github.com/Quality-Max/qmax-receipt"
	"github.com/Quality-Max/qmax-local-agent/exposure"
	"github.com/Quality-Max/qmax-local-agent/policy"
)

func TestDoJSONRecordsEntry(t *testing.T) {
	defer policy.SetMode(policy.ModeWarn)
	policy.SetMode(policy.ModeWarn)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer srv.Close()

	rec := receipt.NewCurrent("test:dojson")

	status, body, err := DoJSON(context.Background(), "POST", srv.URL+"/api/agent/register",
		map[string]string{"hello": "world"}, nil, 5*time.Second)
	if err != nil {
		t.Fatalf("DoJSON: %v", err)
	}
	if status != http.StatusOK {
		t.Fatalf("status = %d", status)
	}
	if len(body) == 0 {
		t.Fatal("empty response body")
	}
	if len(rec.Entries) != 1 {
		t.Fatalf("expected 1 recorded entry, got %d", len(rec.Entries))
	}
	e := rec.Entries[0]
	if e.Method != "POST" || e.Path != "/api/agent/register" {
		t.Errorf("bad entry: %+v", e)
	}
	if e.Category != exposure.CatControl {
		t.Errorf("category = %q, want control", e.Category)
	}
	if e.ReqBytes == 0 || e.ReqSHA256 == "" {
		t.Errorf("request body not hashed/sized: %+v", e)
	}
}

func TestStrictModeBlocksUnlistedEgress(t *testing.T) {
	defer policy.SetMode(policy.ModeWarn)
	policy.SetMode(policy.ModeStrict)

	hit := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hit = true
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	rec := receipt.NewCurrent("test:strict")

	// httptest host (127.0.0.1) is not on the allow-list → must be blocked.
	_, _, err := DoJSON(context.Background(), "POST", srv.URL+"/api/agent/register", nil, nil, 5*time.Second)
	if err == nil {
		t.Fatal("expected strict mode to block the request")
	}
	if hit {
		t.Fatal("request reached the server despite strict block — not fail-closed")
	}
	if len(rec.Entries) != 1 || rec.Entries[0].Note != "blocked-by-egress-policy" {
		t.Fatalf("blocked egress was not recorded: %+v", rec.Entries)
	}
}
