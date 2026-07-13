// Package policy is the runtime egress guard: it decides whether a recorded
// outbound request is permitted by the declared allow-list. In "warn" mode
// (default) a disallowed request is flagged but still sent; in "strict" mode it
// is blocked at the httpx transport before any bytes leave the machine.
//
// This is the runtime half of the Egress Guard. The other half — the static
// guard that forbids un-instrumented egress paths from existing at all — lives
// in httpx/guard_test.go and is the load-bearing piece (see
// docs/EXPOSURE_RECEIPT_DESIGN.md §6).
package policy

import (
	"strings"
	"sync"

	receipt "github.com/Quality-Max/qmax-receipt"
	"github.com/Quality-Max/qmax-local-agent/exposure"
)

// Mode controls what happens to a request that isn't on the allow-list.
type Mode int

const (
	ModeWarn   Mode = iota // record a violation, allow the request through (default)
	ModeStrict             // block the request and record the violation
)

var (
	mu               sync.RWMutex
	mode             = ModeWarn
	allowAuthCapture = false
)

// SetMode sets the global egress mode (wired to --strict-egress).
func SetMode(m Mode) {
	mu.Lock()
	mode = m
	mu.Unlock()
}

// Strict reports whether the guard is in blocking mode.
func Strict() bool {
	mu.RLock()
	defer mu.RUnlock()
	return mode == ModeStrict
}

// SetAllowAuthCapture toggles the high-sensitivity auth-material category
// (cookies/localStorage), gated behind an explicit --allow-auth-capture flag.
func SetAllowAuthCapture(v bool) {
	mu.Lock()
	allowAuthCapture = v
	mu.Unlock()
}

// rule is one allow-list entry: a host suffix plus the categories permitted to it.
type rule struct {
	name       string
	hostSuffix string
	categories map[string]bool
}

// defaultRules is the embedded baseline allow-list. A file-based override
// (~/.qmax/egress-policy.yaml) is a documented follow-up; the embedded default
// is deliberately conservative.
var defaultRules = []rule{
	{
		name:       "qm-backend/standard",
		hostSuffix: "qualitymax.io",
		categories: set(
			exposure.CatControl,
			exposure.CatTestPlan,
			exposure.CatBehavioral,
			exposure.CatTestArtifact,
			exposure.CatSourceDerived,
			exposure.CatTelemetry,
			exposure.CatCloaked,
		),
	},
	{
		name:       "qm-backend/auth-material",
		hostSuffix: "qualitymax.io",
		categories: set(exposure.CatAuthMaterial),
	},
}

// Check evaluates an entry against the allow-list. It returns whether the
// request is permitted and the name of the matching rule (or a reason it
// wasn't). Auth-material additionally requires the explicit opt-in flag.
func Check(e receipt.Entry) (allowed bool, ruleName string) {
	mu.RLock()
	authOK := allowAuthCapture
	mu.RUnlock()

	for _, r := range defaultRules {
		if !hostMatches(e.Host, r.hostSuffix) {
			continue
		}
		if !r.categories[e.Category] {
			continue
		}
		if e.Category == exposure.CatAuthMaterial && !authOK {
			return false, "auth-material/needs --allow-auth-capture"
		}
		return true, r.name
	}
	return false, "no-matching-rule"
}

func hostMatches(host, suffix string) bool {
	// Strip a possible :port before suffix-matching.
	if i := strings.LastIndex(host, ":"); i >= 0 && !strings.Contains(host[i:], "]") {
		host = host[:i]
	}
	return host == suffix || strings.HasSuffix(host, "."+suffix)
}

func set(items ...string) map[string]bool {
	m := make(map[string]bool, len(items))
	for _, it := range items {
		m[it] = true
	}
	return m
}
