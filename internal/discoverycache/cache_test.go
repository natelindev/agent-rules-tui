package discoverycache

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/natelindev/agent-rules-tui/internal/config"
	"github.com/natelindev/agent-rules-tui/internal/scan"
)

func TestSaveLoadCache(t *testing.T) {
	cfg := config.Default()
	cfg.Roots = []string{"/tmp/projects"}
	cfg.CachePath = filepath.Join(t.TempDir(), "discovery.json")

	projects := []scan.Project{
		{
			Name:    "app",
			Root:    "/tmp/projects/app",
			ModTime: time.Now(),
			Files: []scan.AgentFile{
				{Path: "/tmp/projects/app/AGENTS.md", RelPath: "AGENTS.md", ModTime: time.Now()},
			},
		},
	}

	if err := Save(cfg, projects); err != nil {
		t.Fatal(err)
	}

	snapshot, ok, err := Load(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("expected cache hit")
	}
	if len(snapshot.Projects) != 1 || snapshot.Projects[0].Root != projects[0].Root {
		t.Fatalf("unexpected snapshot: %#v", snapshot)
	}
}

func TestLoadRejectsSignatureMismatch(t *testing.T) {
	cfg := config.Default()
	cfg.Roots = []string{"/tmp/projects"}
	cfg.CachePath = filepath.Join(t.TempDir(), "discovery.json")

	if err := Save(cfg, []scan.Project{{Name: "app", Root: "/tmp/projects/app"}}); err != nil {
		t.Fatal(err)
	}

	cfg.Roots = []string{"/tmp/other"}
	if _, ok, err := Load(cfg); err != nil {
		t.Fatal(err)
	} else if ok {
		t.Fatal("expected cache miss for different roots")
	}
}
