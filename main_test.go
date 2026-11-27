package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestResolveUserDataDir(t *testing.T) {
	cfg := t.TempDir()
	defaultDir := filepath.Join(cfg, "Xi")

	// Fresh install: uses the Xi directory.
	if got := resolveUserDataDir("", cfg, defaultDir); got != defaultDir {
		t.Fatalf("fresh install got %q, want %q", got, defaultDir)
	}

	// Explicit override always wins.
	override := filepath.Join(cfg, "custom")
	if got := resolveUserDataDir(override, cfg, defaultDir); got != override {
		t.Fatalf("override got %q, want %q", got, override)
	}

	// Legacy store present without new store: falls back to legacy Sisyphus dir.
	legacyDir := filepath.Join(cfg, "Sisyphus")
	if err := os.MkdirAll(legacyDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(legacyDir, "sisy-store.json"), []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := resolveUserDataDir("", cfg, defaultDir); got != legacyDir {
		t.Fatalf("legacy fallback got %q, want %q", got, legacyDir)
	}

	// A store under the previous name (526Hz) is newer than Sisyphus and wins over it.
	prevDir := filepath.Join(cfg, "526Hz")
	if err := os.MkdirAll(prevDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(prevDir, "sisy-store.json"), []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := resolveUserDataDir("", cfg, defaultDir); got != prevDir {
		t.Fatalf("526Hz fallback got %q, want %q", got, prevDir)
	}

	// Once a store exists under the current name, that directory is used.
	if err := os.MkdirAll(defaultDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(defaultDir, "sisy-store.json"), []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := resolveUserDataDir("", cfg, defaultDir); got != defaultDir {
		t.Fatalf("new store preferred got %q, want %q", got, defaultDir)
	}
}

func TestParseArgsStartHidden(t *testing.T) {
	compact, hidden, dataDir := parseArgs([]string{"--start-hidden", "--user-data", "C:\\tmp\\d"})
	if compact || !hidden || dataDir != `C:\tmp\d` {
		t.Fatalf("unexpected parseArgs result: compact=%v hidden=%v dataDir=%q", compact, hidden, dataDir)
	}
}
