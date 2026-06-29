package install

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"agent-rules-tui/internal/config"
)

const commandName = "agent-rules"

type Notice struct {
	Command    string
	Executable string
	LinkPath   string
	InstallDir string
	ShellRC    string
	NeedsPath  bool
	Reason     string
}

func Detect(cfg config.Config) (*Notice, error) {
	if cfg.IgnorePathPrompt {
		return nil, nil
	}

	executable, err := os.Executable()
	if err != nil {
		return nil, err
	}
	executable, err = filepath.EvalSymlinks(executable)
	if err != nil {
		return nil, err
	}
	if filepath.Base(executable) != commandName {
		return nil, nil
	}

	if found, err := exec.LookPath(commandName); err == nil {
		found, evalErr := filepath.EvalSymlinks(found)
		if evalErr == nil && sameFile(executable, found) {
			return nil, nil
		}
	}

	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return nil, err
	}

	installDir := filepath.Join(home, ".local", "bin")
	notice := &Notice{
		Command:    commandName,
		Executable: executable,
		InstallDir: installDir,
		LinkPath:   filepath.Join(installDir, commandName),
		ShellRC:    shellRCPath(),
		NeedsPath:  !pathContains(installDir),
		Reason:     fmt.Sprintf("%s is not available as this executable in PATH", commandName),
	}
	return notice, nil
}

func Add(notice Notice) error {
	if notice.Executable == "" || notice.LinkPath == "" {
		return errors.New("install notice is incomplete")
	}
	if err := os.MkdirAll(notice.InstallDir, 0o755); err != nil {
		return err
	}

	if err := ensureSymlink(notice.Executable, notice.LinkPath); err != nil {
		return err
	}
	if notice.NeedsPath {
		return ensurePathInShellRC(notice.InstallDir, notice.ShellRC)
	}
	return nil
}

func sameFile(left, right string) bool {
	leftInfo, leftErr := os.Stat(left)
	rightInfo, rightErr := os.Stat(right)
	if leftErr != nil || rightErr != nil {
		return false
	}
	return os.SameFile(leftInfo, rightInfo)
}

func ensureSymlink(target, link string) error {
	if current, err := os.Readlink(link); err == nil {
		resolved := current
		if !filepath.IsAbs(resolved) {
			resolved = filepath.Join(filepath.Dir(link), resolved)
		}
		resolved, _ = filepath.EvalSymlinks(resolved)
		targetEval, _ := filepath.EvalSymlinks(target)
		if resolved == targetEval {
			return nil
		}
		if err := os.Remove(link); err != nil {
			return err
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		info, statErr := os.Stat(link)
		if statErr == nil && !info.IsDir() {
			return fmt.Errorf("%s exists and is not a symlink", link)
		}
		return err
	}
	return os.Symlink(target, link)
}

func ensurePathInShellRC(dir, rcPath string) error {
	if rcPath == "" {
		return nil
	}

	block := pathBlock(dir, rcPath)
	data, err := os.ReadFile(rcPath)
	if err == nil && strings.Contains(string(data), dir) {
		return nil
	}
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(rcPath), 0o755); err != nil {
		return err
	}

	f, err := os.OpenFile(rcPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()

	if len(data) > 0 && !strings.HasSuffix(string(data), "\n") {
		if _, err := f.WriteString("\n"); err != nil {
			return err
		}
	}
	_, err = f.WriteString(block)
	return err
}

func pathBlock(dir, rcPath string) string {
	switch filepath.Base(rcPath) {
	case "config.fish":
		return fmt.Sprintf("\n# Added by agent-rules-tui\nfish_add_path -g %s\n", shellQuote(dir))
	default:
		return fmt.Sprintf("\n# Added by agent-rules-tui\nexport PATH=%s:$PATH\n", shellQuote(dir))
	}
}

func shellRCPath() string {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return ""
	}
	shell := filepath.Base(os.Getenv("SHELL"))
	switch shell {
	case "zsh":
		return filepath.Join(home, ".zshrc")
	case "bash":
		return filepath.Join(home, ".bashrc")
	case "fish":
		return filepath.Join(home, ".config", "fish", "config.fish")
	default:
		return filepath.Join(home, ".profile")
	}
}

func pathContains(dir string) bool {
	dir = filepath.Clean(dir)
	for _, entry := range filepath.SplitList(os.Getenv("PATH")) {
		if filepath.Clean(entry) == dir {
			return true
		}
	}
	return false
}

func shellQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "'\"'\"'") + "'"
}
