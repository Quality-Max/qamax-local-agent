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
// It merges entry-by-entry rather than renaming the directory, because ~/.qmax
// already exists (it holds the installed binary). It never clobbers anything
// already present in ~/.qmax, and is best-effort and idempotent — safe to call
// on every startup.
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
	entries, err := os.ReadDir(oldDir)
	if err != nil {
		return // no legacy dir (or unreadable) — nothing to migrate
	}
	if err := os.MkdirAll(newDir, 0o700); err != nil {
		return
	}
	for _, e := range entries {
		dst := filepath.Join(newDir, e.Name())
		if _, err := os.Stat(dst); err == nil {
			continue // already migrated / present — never clobber
		}
		_ = os.Rename(filepath.Join(oldDir, e.Name()), dst)
	}
}
