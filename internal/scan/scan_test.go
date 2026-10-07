package scan

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/natelindev/agent-rules-tui/internal/config"
)

func TestDiscoverGroupsProjectsAndSkipsDependencyDirs(t *testing.T) {
	root := t.TempDir()
	project := filepath.Join(root, "app")
	mustMkdir(t, filepath.Join(project, ".git"))
	mustWrite(t, filepath.Join(project, "AGENTS.md"), "project")
	mustWrite(t, filepath.Join(project, "nested", "CLAUDE.md"), "nested")
	mustWrite(t, filepath.Join(project, "node_modules", "pkg", "AGENTS.md"), "noise")

	cfg := config.Default()
	cfg.Roots = []string{root}
	cfg.GlobalPaths = nil

	projects, err := Discover(context.Background(), cfg)
	if err != nil {
		t.Fatal(err)
	}
	if len(projects) != 1 {
		t.Fatalf("projects = %d, want 1: %#v", len(projects), projects)
	}
	if projects[0].Root != project {
		t.Fatalf("project root = %q, want %q", projects[0].Root, project)
	}
	if len(projects[0].Files) != 2 {
		t.Fatalf("files = %d, want 2: %#v", len(projects[0].Files), projects[0].Files)
	}
}

func TestDiscoverSortsProjectsByNewestAgentFile(t *testing.T) {
	root := t.TempDir()
	oldProject := filepath.Join(root, "old")
	newProject := filepath.Join(root, "new")
	mustMkdir(t, filepath.Join(oldProject, ".git"))
	mustMkdir(t, filepath.Join(newProject, ".git"))

	oldFile := filepath.Join(oldProject, "AGENTS.md")
	newFile := filepath.Join(newProject, "AGENTS.md")
	mustWrite(t, oldFile, "old")
	mustWrite(t, newFile, "new")

	oldTime := time.Now().Add(-24 * time.Hour)
	newTime := time.Now()
	if err := os.Chtimes(oldFile, oldTime, oldTime); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(newFile, newTime, newTime); err != nil {
		t.Fatal(err)
	}

	cfg := config.Default()
	cfg.Roots = []string{root}
	cfg.GlobalPaths = nil

	projects, err := Discover(context.Background(), cfg)
	if err != nil {
		t.Fatal(err)
	}
	if len(projects) != 2 {
		t.Fatalf("projects = %d, want 2: %#v", len(projects), projects)
	}
	if projects[0].Root != newProject {
		t.Fatalf("first project = %q, want newest project %q", projects[0].Root, newProject)
	}
	if !projects[0].ModTime.After(projects[1].ModTime) {
		t.Fatalf("projects not sorted by newest mod time: %#v", projects)
	}
}

func TestDiscoverGlobalPaths(t *testing.T) {
	root := t.TempDir()
	global := filepath.Join(root, ".claude", "CLAUDE.md")
	mustWrite(t, global, "global")

	cfg := config.Default()
	cfg.Roots = []string{filepath.Join(root, "projects")}
	cfg.GlobalPaths = []string{global}

	projects, err := Discover(context.Background(), cfg)
	if err != nil {
		t.Fatal(err)
	}
	if len(projects) != 1 {
		t.Fatalf("projects = %d, want 1", len(projects))
	}
	if !projects[0].Global {
		t.Fatalf("project should be global: %#v", projects[0])
	}
	if len(projects[0].Files) != 1 || projects[0].Files[0].Path != global {
		t.Fatalf("unexpected files: %#v", projects[0].Files)
	}
}

func mustMkdir(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(path, 0o755); err != nil {
		t.Fatal(err)
	}
}

func mustWrite(t *testing.T, path string, data string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(data), 0o644); err != nil {
		t.Fatal(err)
	}
}
