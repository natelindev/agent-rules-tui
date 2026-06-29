package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const defaultEditor = "nvim"
const appName = "agent-rules-tui"

type Config struct {
	ConfigPath       string   `json:"-"`
	Editor           string   `json:"editor"`
	Roots            []string `json:"roots"`
	Include          []string `json:"include"`
	SkipDirs         []string `json:"skip_dirs"`
	GlobalPaths      []string `json:"global_paths"`
	CachePath        string   `json:"cache_path,omitempty"`
	IgnorePathPrompt bool     `json:"ignore_path_prompt"`
}

func Default() Config {
	return Config{
		Editor: defaultEditor,
		Roots:  []string{"~"},
		Include: []string{
			"AGENTS.md",
			"CLAUDE.md",
			"GEMINI.md",
			"QWEN.md",
			"CODEX.md",
			"OPENAI.md",
			"AIDER.md",
			"CURSOR.md",
			"CODY.md",
			"REPLIT.md",
			"WARP.md",
			".cursorrules",
			".windsurfrules",
			".clinerules",
			".aider.conf.yml",
			".aider.model.settings.yml",
			".aiderignore",
			"opencode.json",
			"agents.json",
		},
		SkipDirs: []string{
			".git",
			".hg",
			".svn",
			"node_modules",
			"vendor",
			".venv",
			"venv",
			".tox",
			".mypy_cache",
			".pytest_cache",
			".ruff_cache",
			".next",
			".nuxt",
			"dist",
			"build",
			"target",
			".gradle",
			".cache",
			".npm",
			".pnpm-store",
			".bun/install/cache",
			".cargo/registry",
			"go/pkg/mod",
			"Library",
			"Applications",
			"Movies",
			"Music",
			"Pictures",
			".codex",
			".claude",
			".gemini",
			".cursor",
			".config",
		},
		GlobalPaths: []string{
			"~/AGENTS.md",
			"~/CLAUDE.md",
			"~/GEMINI.md",
			"~/QWEN.md",
			"~/CODEX.md",
			"~/.claude/CLAUDE.md",
			"~/.codex/AGENTS.md",
			"~/.codex/CLAUDE.md",
			"~/.codex/instructions.md",
			"~/.gemini/GEMINI.md",
			"~/.opencode/AGENTS.md",
			"~/.opencode/CLAUDE.md",
			"~/.config/opencode/AGENTS.md",
			"~/.config/opencode/CLAUDE.md",
			"~/.cursor/rules",
			"~/.aider.conf.yml",
			"~/.aider.model.settings.yml",
			"~/.aiderignore",
			"~/.cursorrules",
			"~/.windsurfrules",
			"~/.clinerules",
		},
	}
}

func DefaultPath() string {
	dir := os.Getenv("XDG_CONFIG_HOME")
	if dir == "" {
		home, err := os.UserHomeDir()
		if err != nil || home == "" {
			return appName + ".json"
		}
		dir = filepath.Join(home, ".config")
	}
	return filepath.Join(dir, appName, "config.json")
}

func DefaultCachePath() string {
	dir := os.Getenv("XDG_CACHE_HOME")
	if dir == "" {
		home, err := os.UserHomeDir()
		if err != nil || home == "" {
			return filepath.Join(".cache", appName, "discovery.json")
		}
		dir = filepath.Join(home, ".cache")
	}
	return filepath.Join(dir, appName, "discovery.json")
}

func Load(path string) (Config, error) {
	cfg := Default()
	cfg.ConfigPath = path
	cfg.Editor = ""

	if path == "" {
		return ApplyRuntimeDefaults(cfg), nil
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			if err := WriteDefault(path, true); err != nil {
				return Config{}, err
			}
			return ApplyRuntimeDefaults(cfg), nil
		}
		return Config{}, err
	}

	var fileCfg Config
	if err := json.Unmarshal(data, &fileCfg); err != nil {
		return Config{}, fmt.Errorf("parse config %s: %w", path, err)
	}

	if fileCfg.Editor != "" {
		cfg.Editor = fileCfg.Editor
	}
	if fileCfg.Roots != nil {
		cfg.Roots = fileCfg.Roots
	}
	if fileCfg.Include != nil {
		cfg.Include = fileCfg.Include
	}
	if fileCfg.SkipDirs != nil {
		cfg.SkipDirs = fileCfg.SkipDirs
	}
	if fileCfg.GlobalPaths != nil {
		cfg.GlobalPaths = fileCfg.GlobalPaths
	}
	if fileCfg.CachePath != "" {
		cfg.CachePath = fileCfg.CachePath
	}
	cfg.IgnorePathPrompt = fileCfg.IgnorePathPrompt

	return ApplyRuntimeDefaults(cfg), nil
}

func ApplyRuntimeDefaults(cfg Config) Config {
	if cfg.Editor == "" {
		cfg.Editor = firstNonEmpty(
			os.Getenv("AGENT_RULES_EDITOR"),
			os.Getenv("AGENT_MEM_EDITOR"),
			os.Getenv("VISUAL"),
			os.Getenv("EDITOR"),
			defaultEditor,
		)
	}
	if len(cfg.Roots) == 0 {
		cfg.Roots = Default().Roots
	}
	if len(cfg.Include) == 0 {
		cfg.Include = Default().Include
	}
	if len(cfg.SkipDirs) == 0 {
		cfg.SkipDirs = Default().SkipDirs
	}
	if len(cfg.GlobalPaths) == 0 {
		cfg.GlobalPaths = Default().GlobalPaths
	}
	if cfg.CachePath == "" {
		cfg.CachePath = DefaultCachePath()
	}
	return cfg
}

func WriteDefault(path string, overwrite bool) error {
	cfg := Default()
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	if path == "" {
		return errors.New("config path is empty")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	if !overwrite {
		if _, err := os.Stat(path); err == nil {
			return fmt.Errorf("config already exists: %s", path)
		} else if !errors.Is(err, os.ErrNotExist) {
			return err
		}
	}
	return os.WriteFile(path, append(data, '\n'), 0o644)
}

func SetEditor(path string, editor string) error {
	editor = strings.TrimSpace(editor)
	if editor == "" {
		return errors.New("editor is empty")
	}

	cfg, err := loadForUpdate(path)
	if err != nil {
		return err
	}
	cfg.Editor = editor
	return writeConfig(path, cfg)
}

func SetIgnorePathPrompt(path string, ignored bool) error {
	cfg, err := loadForUpdate(path)
	if err != nil {
		return err
	}
	cfg.IgnorePathPrompt = ignored
	return writeConfig(path, cfg)
}

func loadForUpdate(path string) (Config, error) {
	if path == "" {
		return Config{}, errors.New("config path is empty")
	}

	cfg := Default()
	data, err := os.ReadFile(path)
	if err == nil {
		if err := json.Unmarshal(data, &cfg); err != nil {
			return Config{}, fmt.Errorf("parse config %s: %w", path, err)
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return Config{}, err
	}

	cfg.ConfigPath = ""
	return cfg, nil
}

func writeConfig(path string, cfg Config) error {
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, append(data, '\n'), 0o644)
}

func ExpandPath(path string) (string, error) {
	if path == "" {
		return "", nil
	}

	expanded := os.ExpandEnv(path)
	if expanded == "~" || strings.HasPrefix(expanded, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		if expanded == "~" {
			expanded = home
		} else {
			expanded = filepath.Join(home, expanded[2:])
		}
	}

	return filepath.Abs(expanded)
}

func HomeDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return home
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}
