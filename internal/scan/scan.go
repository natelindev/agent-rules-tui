package scan

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"agent-rules-tui/internal/config"
)

type AgentFile struct {
	Path        string
	Name        string
	RelPath     string
	ProjectRoot string
	Global      bool
	Size        int64
	ModTime     time.Time
}

type Project struct {
	Name    string
	Root    string
	Global  bool
	ModTime time.Time
	Files   []AgentFile
}

func Discover(ctx context.Context, cfg config.Config) ([]Project, error) {
	files := map[string]AgentFile{}
	home := config.HomeDir()

	for _, globalPath := range cfg.GlobalPaths {
		path, err := config.ExpandPath(globalPath)
		if err != nil {
			continue
		}
		addPath(ctx, path, cfg, home, true, files)
	}

	for _, root := range cfg.Roots {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		expanded, err := config.ExpandPath(root)
		if err != nil {
			return nil, err
		}
		if expanded == "" {
			continue
		}
		walkRoot(ctx, expanded, cfg, home, files)
	}

	return groupProjects(files, home), ctx.Err()
}

func addPath(ctx context.Context, path string, cfg config.Config, home string, global bool, files map[string]AgentFile) {
	info, err := os.Stat(path)
	if err != nil {
		return
	}
	if info.IsDir() {
		_ = filepath.WalkDir(path, func(candidate string, d fs.DirEntry, err error) error {
			if err != nil {
				return nil
			}
			if ctx.Err() != nil {
				return ctx.Err()
			}
			if d.IsDir() {
				return nil
			}
			if isAgentFile(candidate, cfg.Include) {
				recordFile(candidate, cfg, home, global || isGlobalPath(candidate, home), files)
			}
			return nil
		})
		return
	}
	if isAgentFile(path, cfg.Include) {
		recordFile(path, cfg, home, global || isGlobalPath(path, home), files)
	}
}

func walkRoot(ctx context.Context, root string, cfg config.Config, home string, files map[string]AgentFile) {
	_ = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if ctx.Err() != nil {
			return ctx.Err()
		}

		if d.IsDir() {
			if path != root && shouldSkipDir(path, d.Name(), root, cfg.SkipDirs) {
				return filepath.SkipDir
			}
			return nil
		}

		if isAgentFile(path, cfg.Include) {
			recordFile(path, cfg, home, isGlobalPath(path, home), files)
		}
		return nil
	})
}

func recordFile(path string, cfg config.Config, home string, global bool, files map[string]AgentFile) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return
	}
	abs = filepath.Clean(abs)
	if _, ok := files[abs]; ok {
		return
	}

	info, err := os.Stat(abs)
	if err != nil || info.IsDir() {
		return
	}

	projectRoot := "Global"
	if !global {
		projectRoot = findProjectRoot(abs, cfg.Roots)
	}

	relBase := projectRoot
	if global {
		relBase = home
	}
	rel, err := filepath.Rel(relBase, abs)
	if err != nil || strings.HasPrefix(rel, "..") {
		rel = abs
	}

	files[abs] = AgentFile{
		Path:        abs,
		Name:        filepath.Base(abs),
		RelPath:     filepath.ToSlash(rel),
		ProjectRoot: projectRoot,
		Global:      global,
		Size:        info.Size(),
		ModTime:     info.ModTime(),
	}
}

func groupProjects(files map[string]AgentFile, home string) []Project {
	projectsByRoot := map[string]*Project{}
	for _, file := range files {
		root := file.ProjectRoot
		name := filepath.Base(root)
		if file.Global {
			root = "Global"
			name = "Global"
		} else if root == home {
			name = "~"
		}

		project := projectsByRoot[root]
		if project == nil {
			project = &Project{Name: name, Root: root, Global: file.Global}
			projectsByRoot[root] = project
		}
		project.Files = append(project.Files, file)
		if file.ModTime.After(project.ModTime) {
			project.ModTime = file.ModTime
		}
	}

	projects := make([]Project, 0, len(projectsByRoot))
	for _, project := range projectsByRoot {
		sort.Slice(project.Files, func(i, j int) bool {
			return strings.ToLower(project.Files[i].RelPath) < strings.ToLower(project.Files[j].RelPath)
		})
		projects = append(projects, *project)
	}

	sort.Slice(projects, func(i, j int) bool {
		if !projects[i].ModTime.Equal(projects[j].ModTime) {
			return projects[i].ModTime.After(projects[j].ModTime)
		}
		left := strings.ToLower(projects[i].Root)
		right := strings.ToLower(projects[j].Root)
		return left < right
	})

	return projects
}

func shouldSkipDir(path, name, root string, skipDirs []string) bool {
	absPath := filepath.Clean(path)
	rel, err := filepath.Rel(root, absPath)
	if err != nil {
		rel = name
	}
	rel = filepath.ToSlash(filepath.Clean(rel))

	for _, skip := range skipDirs {
		skip = strings.TrimSpace(skip)
		if skip == "" {
			continue
		}
		expanded, err := config.ExpandPath(skip)
		if err == nil && filepath.IsAbs(expanded) {
			expanded = filepath.Clean(expanded)
			if absPath == expanded || strings.HasPrefix(absPath, expanded+string(os.PathSeparator)) {
				return true
			}
		}

		skip = filepath.ToSlash(filepath.Clean(skip))
		if strings.Contains(skip, "/") {
			if rel == skip || strings.HasPrefix(rel, skip+"/") {
				return true
			}
			continue
		}
		if name == skip {
			return true
		}
	}

	return false
}

func isAgentFile(path string, includes []string) bool {
	base := filepath.Base(path)
	for _, include := range includes {
		if strings.EqualFold(base, include) {
			return true
		}
	}

	slash := filepath.ToSlash(path)
	lower := strings.ToLower(slash)
	ext := strings.ToLower(filepath.Ext(path))

	if strings.HasSuffix(lower, "/.github/copilot-instructions.md") {
		return true
	}
	if strings.HasSuffix(lower, "/.codex/instructions.md") {
		return true
	}
	if strings.Contains(lower, "/.codex/") && ext == ".md" {
		return true
	}
	if strings.Contains(lower, "/.cursor/rules/") && (ext == ".md" || ext == ".mdc" || ext == ".txt") {
		return true
	}
	if strings.Contains(lower, "/.windsurf/rules/") && (ext == ".md" || ext == ".mdc" || ext == ".txt") {
		return true
	}
	if strings.Contains(lower, "/.clinerules/") && (ext == ".md" || ext == ".txt") {
		return true
	}
	if strings.Contains(lower, "/.roo/rules") && (ext == ".md" || ext == ".txt") {
		return true
	}
	if strings.Contains(lower, "/.augment/rules/") && (ext == ".md" || ext == ".txt") {
		return true
	}
	if strings.Contains(lower, "/.claude/") && strings.EqualFold(base, "memory.md") {
		return true
	}

	return false
}

func isGlobalPath(path, home string) bool {
	if home == "" {
		return false
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return false
	}
	rel, err := filepath.Rel(home, abs)
	if err != nil || strings.HasPrefix(rel, "..") || rel == "." {
		return false
	}

	rel = filepath.ToSlash(rel)
	if !strings.Contains(rel, "/") {
		return true
	}

	globalPrefixes := []string{
		".claude/",
		".codex/",
		".gemini/",
		".opencode/",
		".cursor/rules/",
		".config/opencode/",
	}
	for _, prefix := range globalPrefixes {
		if strings.HasPrefix(rel, prefix) {
			return true
		}
	}

	return false
}

func findProjectRoot(path string, roots []string) string {
	dir := filepath.Dir(path)
	limit := nearestScanRoot(dir, roots)

	for {
		if hasProjectMarker(dir) {
			return dir
		}
		if dir == limit || dir == filepath.Dir(dir) {
			break
		}
		dir = filepath.Dir(dir)
	}

	if limit != "" && isAncestor(limit, filepath.Dir(path)) {
		return nearestProjectParent(path, limit)
	}
	return filepath.Dir(path)
}

func nearestScanRoot(path string, roots []string) string {
	var best string
	for _, root := range roots {
		expanded, err := config.ExpandPath(root)
		if err != nil {
			continue
		}
		expanded = filepath.Clean(expanded)
		if path == expanded || strings.HasPrefix(path, expanded+string(os.PathSeparator)) {
			if len(expanded) > len(best) {
				best = expanded
			}
		}
	}
	if best == "" {
		home := config.HomeDir()
		if home != "" && isAncestor(home, path) {
			return home
		}
	}
	return best
}

func nearestProjectParent(path, limit string) string {
	dir := filepath.Dir(path)
	for dir != limit && dir != filepath.Dir(dir) {
		parent := filepath.Dir(dir)
		if parent == limit {
			return dir
		}
		dir = parent
	}
	return filepath.Dir(path)
}

func hasProjectMarker(dir string) bool {
	markers := []string{
		".git",
		"package.json",
		"go.mod",
		"pyproject.toml",
		"Cargo.toml",
		"deno.json",
		"deno.jsonc",
		"pnpm-workspace.yaml",
		"yarn.lock",
		"bun.lockb",
		"Makefile",
	}
	for _, marker := range markers {
		if _, err := os.Stat(filepath.Join(dir, marker)); err == nil {
			return true
		} else if !errors.Is(err, os.ErrNotExist) {
			continue
		}
	}
	return false
}

func isAncestor(parent, child string) bool {
	parent = filepath.Clean(parent)
	child = filepath.Clean(child)
	return child == parent || strings.HasPrefix(child, parent+string(os.PathSeparator))
}
