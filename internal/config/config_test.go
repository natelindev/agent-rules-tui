package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDefaultPathUsesXDGConfigHome(t *testing.T) {
	configHome := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", configHome)

	got := DefaultPath()
	want := filepath.Join(configHome, "agent-rules-tui", "config.json")
	if got != want {
		t.Fatalf("DefaultPath() = %q, want %q", got, want)
	}
}

func TestLoadCreatesDefaultConfig(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")

	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Editor != defaultEditor {
		t.Fatalf("Editor = %q, want %q", cfg.Editor, defaultEditor)
	}
	if cfg.CachePath == "" {
		t.Fatal("CachePath should be set by runtime defaults")
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("expected persisted config at %s: %v", path, err)
	}
}

func TestSetIgnorePathPrompt(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	if err := WriteDefault(path, false); err != nil {
		t.Fatal(err)
	}
	if err := SetIgnorePathPrompt(path, true); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if !cfg.IgnorePathPrompt {
		t.Fatal("IgnorePathPrompt = false, want true")
	}
}
