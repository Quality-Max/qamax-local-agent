package policy

import (
	"testing"

	receipt "github.com/Quality-Max/qmax-receipt"
	"github.com/Quality-Max/qmax-local-agent/exposure"
)

func TestAllowsStandardCategories(t *testing.T) {
	e := receipt.Entry{Host: "app.qualitymax.io", Category: exposure.CatBehavioral}
	allowed, rule := Check(e)
	if !allowed {
		t.Fatalf("behavioral-snapshot to qualitymax.io should be allowed, got rule=%s", rule)
	}
}

func TestRejectsUnknownHost(t *testing.T) {
	e := receipt.Entry{Host: "evil.example.com", Category: exposure.CatControl}
	if allowed, _ := Check(e); allowed {
		t.Fatal("unknown host must not be allowed")
	}
}

func TestRejectsUncategorized(t *testing.T) {
	e := receipt.Entry{Host: "app.qualitymax.io", Category: exposure.CatUncategorized}
	if allowed, _ := Check(e); allowed {
		t.Fatal("uncategorized egress must not be allowed")
	}
}

func TestAuthMaterialNeedsOptIn(t *testing.T) {
	SetAllowAuthCapture(false)
	defer SetAllowAuthCapture(false)

	e := receipt.Entry{Host: "app.qualitymax.io", Category: exposure.CatAuthMaterial}
	if allowed, rule := Check(e); allowed {
		t.Fatalf("auth-material must be gated, got rule=%s", rule)
	}
	SetAllowAuthCapture(true)
	if allowed, _ := Check(e); !allowed {
		t.Fatal("auth-material should be allowed once opted in")
	}
}

func TestHostSuffixMatchingWithPort(t *testing.T) {
	e := receipt.Entry{Host: "app.qualitymax.io:443", Category: exposure.CatControl}
	if allowed, _ := Check(e); !allowed {
		t.Fatal("host:port should match the suffix rule")
	}
}

func TestStrictToggle(t *testing.T) {
	defer SetMode(ModeWarn)
	SetMode(ModeStrict)
	if !Strict() {
		t.Fatal("expected strict mode")
	}
	SetMode(ModeWarn)
	if Strict() {
		t.Fatal("expected warn mode")
	}
}
