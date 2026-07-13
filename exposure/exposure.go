// Package exposure holds the qmax CLI's traffic taxonomy — the category
// constants and the Classify function that maps a (host, path) pair to an
// exposure category. It is deliberately agent-specific: the receipt schema
// (github.com/Quality-Max/qmax-receipt) treats Entry.Category as a free-form
// string, and each agent supplies its own taxonomy. qmax-code carries its own
// (llm-prompt / llm-completion / cloud-api / …).
//
// Path templatization is shared, so Classify delegates to receipt.Templatize.
package exposure

import (
	"strings"

	receipt "github.com/Quality-Max/qmax-receipt"
)

// Category constants — the qmax CLI taxonomy (see docs/EXPOSURE_RECEIPT_DESIGN.md §3).
const (
	CatControl       = "control"             // metadata only: register, heartbeat, status, polls
	CatTestPlan      = "test-plan"           // script IDs, exec/browser config
	CatBehavioral    = "behavioral-snapshot" // DOM, a11y tree, form metadata, screenshots
	CatTestArtifact  = "test-artifact"       // test output, errors, screenshots, video
	CatAuthMaterial  = "auth-material"       // cookies / localStorage (plaintext) — sharpest edge
	CatSourceDerived = "source-derived"      // SAST findings, repo import, requirements text
	CatTelemetry     = "telemetry"           // system metrics, skill feedback
	CatCloaked       = "cloaked"             // body produced by Cloak (reduced-exposure)
	CatUncategorized = "uncategorized"       // unknown host/path — flagged so it can't hide
)

// Classify maps a (host, path) pair to an exposure category. It takes plain
// strings so this package never imports net/http. Path should be the raw URL
// path; classification is done against the templatized form.
func Classify(host, path string) string {
	p := receipt.Templatize(path)
	switch {
	case strings.HasSuffix(p, "/snapshot") && strings.Contains(p, "/crawl/"):
		return CatBehavioral
	case strings.HasSuffix(p, "/result") && strings.Contains(p, "/assignments/"):
		return CatTestArtifact
	case strings.Contains(p, "/user-data/"):
		return CatAuthMaterial
	case strings.Contains(p, "/sast/"),
		strings.Contains(p, "/repositories/import"),
		strings.Contains(p, "/import/document"):
		return CatSourceDerived
	case strings.HasSuffix(p, "/heartbeat"),
		strings.HasSuffix(p, "/skill-feedback"):
		return CatTelemetry
	case strings.HasSuffix(p, "/register"),
		strings.Contains(p, "/assignments"), // status/pending (result handled above)
		strings.Contains(p, "/crawl/pending"),
		strings.HasSuffix(p, "/crawl/{id}/error"),
		strings.Contains(p, "/workflow/status"):
		return CatControl
	case strings.Contains(p, "/ai-crawl/"),
		strings.Contains(p, "/playwright-execution/"),
		strings.Contains(p, "/automation/"),
		strings.Contains(p, "/test-cases/"),
		strings.Contains(p, "/projects"),
		strings.Contains(p, "/repositories"):
		return CatTestPlan
	default:
		return CatUncategorized
	}
}
