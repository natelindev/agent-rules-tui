package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"

	tea "github.com/charmbracelet/bubbletea"

	"agent-rules-tui/internal/config"
	"agent-rules-tui/internal/discoverycache"
	"agent-rules-tui/internal/install"
	"agent-rules-tui/internal/scan"
	"agent-rules-tui/internal/tui"
)

type multiFlag []string

func (m *multiFlag) String() string {
	return fmt.Sprint([]string(*m))
}

func (m *multiFlag) Set(value string) error {
	*m = append(*m, value)
	return nil
}

func main() {
	var roots multiFlag
	var editor string
	var configPath string
	var initConfig bool
	var printConfig bool
	var warmCache bool

	defaultConfigPath := config.DefaultPath()
	flag.Var(&roots, "root", "root directory to scan; may be passed more than once")
	flag.StringVar(&editor, "editor", "", "editor command to use, for example nvim, vim, code -w, fresh")
	flag.StringVar(&configPath, "config", defaultConfigPath, "config file path")
	flag.BoolVar(&initConfig, "init-config", false, "write a default config file and exit")
	flag.BoolVar(&printConfig, "print-config", false, "print the effective config file path and exit")
	flag.BoolVar(&warmCache, "warm-cache", false, "scan, update the discovery cache, print a summary, and exit")
	flag.Parse()

	if printConfig {
		fmt.Println(configPath)
		return
	}

	if initConfig {
		if err := config.WriteDefault(configPath, false); err != nil {
			exitErr(err)
		}
		fmt.Printf("wrote %s\n", configPath)
		return
	}

	cfg, err := config.Load(configPath)
	if err != nil {
		exitErr(err)
	}
	cfg.ConfigPath = configPath

	if len(roots) > 0 {
		cfg.Roots = roots
	}
	if editor != "" {
		cfg.Editor = editor
	}
	cfg = config.ApplyRuntimeDefaults(cfg)

	for i, root := range cfg.Roots {
		resolved, err := config.ExpandPath(root)
		if err != nil {
			exitErr(fmt.Errorf("resolve root %q: %w", root, err))
		}
		cfg.Roots[i] = filepath.Clean(resolved)
	}

	if warmCache {
		projects, err := scan.Discover(context.Background(), cfg)
		if err != nil {
			exitErr(err)
		}
		if err := discoverycache.Save(cfg, projects); err != nil {
			exitErr(err)
		}
		fmt.Printf("cached %d files in %d projects at %s\n", countFiles(projects), len(projects), cfg.CachePath)
		return
	}

	pathNotice, _ := install.Detect(cfg)

	program := tea.NewProgram(
		tui.New(cfg, pathNotice),
		tea.WithAltScreen(),
		tea.WithMouseCellMotion(),
	)
	if _, err := program.Run(); err != nil {
		if !errors.Is(err, tea.ErrProgramKilled) {
			exitErr(err)
		}
	}
}

func exitErr(err error) {
	fmt.Fprintf(os.Stderr, "agent-rules: %v\n", err)
	os.Exit(1)
}

func countFiles(projects []scan.Project) int {
	count := 0
	for _, project := range projects {
		count += len(project.Files)
	}
	return count
}
