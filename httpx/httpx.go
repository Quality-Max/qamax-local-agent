// Package httpx is the SINGLE egress chokepoint for the agent. Every outbound
// HTTP request in the codebase must be built and sent here, so that the
// Exposure Receipt records it and the egress policy can gate it.
//
// The static guard in guard_test.go enforces that no other package constructs
// HTTP clients/requests or imports third-party HTTP libraries — making an
// un-receipted egress path impossible to merge.
//
// Quality commitments enforced here:
//   - Stream, don't buffer: request bodies are hashed via a streaming reader as
//     the transport sends them, so multi-MB screenshots/video never double in
//     memory (io.ReadAll on the body is forbidden).
//   - Fail closed: in strict mode a disallowed request is blocked before any
//     bytes leave; on transport error the attempt is still recorded. No egress
//     attempt is ever unrecorded.
package httpx

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"hash"
	"io"
	"net/http"
	"time"

	receipt "github.com/Quality-Max/qmax-receipt"
	"github.com/Quality-Max/qmax-local-agent/exposure"
	"github.com/Quality-Max/qmax-local-agent/policy"
)

// maxResponseBody caps response reads to prevent memory exhaustion (preserves
// prior behavior from agent.go/crawl.go).
const maxResponseBody = 50 * 1024 * 1024

// NewClient returns an *http.Client whose Transport records every request into
// the active Exposure Receipt. Timeout semantics are unchanged from a plain
// &http.Client{Timeout: timeout}.
func NewClient(timeout time.Duration) *http.Client {
	return &http.Client{
		Timeout:   timeout,
		Transport: &receiptTransport{base: http.DefaultTransport},
	}
}

type receiptTransport struct{ base http.RoundTripper }

// hashingBody hashes and counts bytes as the transport reads the request body
// to send it — no full-body buffering.
type hashingBody struct {
	rc io.ReadCloser
	h  hash.Hash
	n  int64
}

func (b *hashingBody) Read(p []byte) (int, error) {
	n, err := b.rc.Read(p)
	if n > 0 {
		b.h.Write(p[:n])
		b.n += int64(n)
	}
	return n, err
}

func (b *hashingBody) Close() error { return b.rc.Close() }

func (t *receiptTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	rec := receipt.FromContext(req.Context())

	entry := receipt.Entry{
		Timestamp: time.Now().UTC(),
		Method:    req.Method,
		Host:      req.URL.Host,
		Path:      receipt.Templatize(req.URL.Path),
		Category:  exposure.Classify(req.URL.Host, req.URL.Path),
	}
	entry.Allowed, entry.Rule = policy.Check(entry)

	// Fail closed: block disallowed egress before any bytes leave the machine.
	if !entry.Allowed && policy.Strict() {
		entry.Note = "blocked-by-egress-policy"
		rec.Record(entry)
		return nil, fmt.Errorf("egress blocked by policy: %s %s (category=%s, rule=%s)",
			entry.Method, entry.Path, entry.Category, entry.Rule)
	}

	// Wrap the body so it is hashed+counted as it streams to the wire.
	var hb *hashingBody
	if req.Body != nil {
		hb = &hashingBody{rc: req.Body, h: sha256.New()}
		req.Body = hb
	} else {
		hb = &hashingBody{rc: io.NopCloser(bytes.NewReader(nil)), h: sha256.New()}
	}

	resp, err := t.base.RoundTrip(req)

	entry.ReqBytes = hb.n
	entry.ReqSHA256 = hex.EncodeToString(hb.h.Sum(nil))
	if err != nil {
		entry.Note = "transport-error: " + err.Error()
	} else if resp != nil {
		entry.RespStatus = resp.StatusCode
		entry.RespBytes = resp.ContentLength
	}
	rec.Record(entry)
	return resp, err
}

// DoJSON is the convenience egress primitive used across the agent. It marshals
// body to JSON (when non-nil), sends via a receipt-recording client, and returns
// the status code and (capped) response body. ctx routes the receipt; pass the
// receipt-bearing context from receipt.Begin for concurrent work.
func DoJSON(ctx context.Context, method, url string, body interface{}, headers map[string]string, timeout time.Duration) (int, []byte, error) {
	var reqBody io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return 0, nil, fmt.Errorf("marshal request: %w", err)
		}
		reqBody = bytes.NewReader(data)
	}

	if ctx == nil {
		ctx = context.Background()
	}
	req, err := http.NewRequestWithContext(ctx, method, url, reqBody)
	if err != nil {
		return 0, nil, fmt.Errorf("create request: %w", err)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}

	resp, err := NewClient(timeout).Do(req)
	if err != nil {
		return 0, nil, fmt.Errorf("do request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBody))
	if err != nil {
		return resp.StatusCode, nil, fmt.Errorf("read response: %w", err)
	}
	return resp.StatusCode, respBody, nil
}
