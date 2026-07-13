package main

import (
	"os"
	"path/filepath"
	"testing"
)

func migWrite(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}

func migRead(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func TestMigrateLegacyStateDir(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	oldDir := filepath.Join(home, ".qamax")
	newDir := filepath.Join(home, ".qmax")

	// Legacy state: config (auth), signing key, and a receipt.
	if err := os.MkdirAll(filepath.Join(oldDir, "receipts"), 0o700); err != nil {
		t.Fatal(err)
	}
	migWrite(t, filepath.Join(oldDir, "config.json"), `{"api_key":"legacy"}`)
	migWrite(t, filepath.Join(oldDir, "receipt_ed25519.seed"), "seedbytes")
	migWrite(t, filepath.Join(oldDir, "receipts", "r1.json"), "{}")

	// ~/.qmax already exists (it holds the binary) and already has a config that
	// must NOT be clobbered by the migration.
	if err := os.MkdirAll(newDir, 0o700); err != nil {
		t.Fatal(err)
	}
	migWrite(t, filepath.Join(newDir, "config.json"), `{"api_key":"current"}`)

	migrateLegacyStateDir()

	// No-clobber: pre-existing ~/.qmax/config.json is preserved.
	if got := migRead(t, filepath.Join(newDir, "config.json")); got != `{"api_key":"current"}` {
		t.Errorf("migration clobbered existing config.json: %q", got)
	}
	// Signing identity and receipts moved into ~/.qmax.
	if got := migRead(t, filepath.Join(newDir, "receipt_ed25519.seed")); got != "seedbytes" {
		t.Errorf("signing key not migrated: %q", got)
	}
	if _, err := os.Stat(filepath.Join(newDir, "receipts", "r1.json")); err != nil {
		t.Errorf("receipts dir not migrated: %v", err)
	}

	// Idempotent: a second run is a harmless no-op.
	migrateLegacyStateDir()
	if got := migRead(t, filepath.Join(newDir, "receipt_ed25519.seed")); got != "seedbytes" {
		t.Errorf("second migration corrupted state: %q", got)
	}
}

func TestMigrateNoLegacyDirIsNoop(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	// No ~/.qamax at all — must not create anything or panic.
	migrateLegacyStateDir()
	if _, err := os.Stat(filepath.Join(home, ".qamax")); !os.IsNotExist(err) {
		t.Errorf("migration created a legacy dir out of nothing")
	}
}
