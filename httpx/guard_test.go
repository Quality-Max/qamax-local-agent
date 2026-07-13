package httpx

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestNoEgressOutsideHttpx is the static Egress Guard. It walks the whole agent
// module and fails if any package other than httpx constructs an HTTP
// client/request/transport or imports a third-party HTTP library. This makes an
// un-receipted egress path impossible to merge — the load-bearing half of the
// "Receipts, not promises" guarantee (docs/EXPOSURE_RECEIPT_DESIGN.md §6a).
//
// net/http may still be IMPORTED elsewhere for types and status constants
// (e.g. http.StatusOK); only the egress-creating symbols below are forbidden.
func TestNoEgressOutsideHttpx(t *testing.T) {
	// Egress-creating symbols. Their only legitimate home is this package.
	forbiddenSymbols := []string{
		"http.Client{",
		"http.NewRequest(",
		"http.NewRequestWithContext(",
		"http.Get(",
		"http.Post(",
		"http.PostForm(",
		"http.Head(",
		"http.DefaultClient",
		"http.DefaultTransport",
	}
	// Third-party HTTP clients that would bypass net/http entirely.
	forbiddenImports := []string{
		"go-resty", "levigross/grequests", "h2non/gentleman",
		"valyala/fasthttp", "imroc/req", "jmcvetta/napping",
	}

	root := ".." // module root: local-agent/go
	var violations []string

	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			// Skip this package (the sanctioned chokepoint) and hidden/vendor dirs.
			if info.Name() == "httpx" || info.Name() == "vendor" || strings.HasPrefix(info.Name(), ".") {
				if path != root {
					return filepath.SkipDir
				}
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		data, rerr := os.ReadFile(path)
		if rerr != nil {
			return rerr
		}
		for i, line := range strings.Split(string(data), "\n") {
			// Ignore comments to avoid false positives in prose.
			code := line
			if idx := strings.Index(code, "//"); idx >= 0 {
				code = code[:idx]
			}
			for _, sym := range forbiddenSymbols {
				if strings.Contains(code, sym) {
					violations = append(violations,
						relf(path)+":"+itoa(i+1)+"  forbidden egress symbol "+sym)
				}
			}
			for _, imp := range forbiddenImports {
				if strings.Contains(line, imp) {
					violations = append(violations,
						relf(path)+":"+itoa(i+1)+"  forbidden HTTP library import "+imp)
				}
			}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk failed: %v", err)
	}
	if len(violations) > 0 {
		t.Fatalf("egress guard: %d violation(s) outside httpx — route all outbound HTTP through httpx.NewClient/DoJSON:\n%s",
			len(violations), strings.Join(violations, "\n"))
	}
}

func relf(p string) string {
	if r, err := filepath.Rel("..", p); err == nil {
		return r
	}
	return p
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b [20]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	return string(b[i:])
}
