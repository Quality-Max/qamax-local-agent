package main

import (
	"os"
	"path/filepath"
)

// legacyStateDirName is the pre-unification state directory (historical
// "QA Max"). Persistent state used to live in ~/.qamax while the binary was
// installed to ~/.qmax; the two are now unified under ~/.qmax.
const legacyStateDirName = ".qamax"

// migrateLegacyStateDir moves persistent state — config.json, the receipts/
// directory, and the ed25519 receipt signing key — from the legacy ~/.qamax
// directory into the unified ~/.qmax on first run, so existing users keep their
// login and the agent keeps its signing identity across the rename.
//
// It merges into ~/.qmax (which already exists — it holds the installed binary)
// rather than renaming, recursing into subdirectories so legacy receipts are
// never left stranded even when ~/.qmax/receipts already exists. It never
// clobbers a file already present in ~/.qmax, and is best-effort + idempotent —
// safe to call on every startup.
func migrateLegacyStateDir() {
	home, err := os.UserHomeDir()
	if err != nil {
		return
	}
	oldDir := filepath.Join(home, legacyStateDirName)
	newDir := filepath.Join(home, configDirName) // ".qmax"
	if oldDir == newDir {
		return
	}
	if _, err := os.Stat(oldDir); err != nil {
		return // no legacy dir — nothing to migrate
	}
	_ = mergeTree(oldDir, newDir)
}

// mergeTree recursively moves entries from src into dst, creating directories as
// needed. Directories are MERGED (recursed into), not skipped, so a legacy
// subdirectory survives even when the destination subdirectory already exists —
// this is what keeps old receipts reachable. A file already present in dst is
// never overwritten. Best-effort: individual failures are ignored so a partial
// migration never blocks startup, and re-running completes what remains.
func mergeTree(src, dst string) error {
	entries, err := os.ReadDir(src)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dst, 0o700); err != nil {
		return err
	}
	for _, e := range entries {
		s := filepath.Join(src, e.Name())
		d := filepath.Join(dst, e.Name())
		if e.IsDir() {
			_ = mergeTree(s, d) // recurse — merge, never skip a whole subtree
			continue
		}
		if _, err := os.Stat(d); err == nil {
			continue // file already migrated / present — never clobber
		}
		_ = os.Rename(s, d)
	}
	return nil
}
