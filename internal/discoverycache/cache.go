package discoverycache

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/natelindev/agent-rules-tui/internal/config"
	"github.com/natelindev/agent-rules-tui/internal/scan"
)

type Snapshot struct {
	Version   int            `json:"version"`
	SavedAt   time.Time      `json:"saved_at"`
	Signature string         `json:"signature"`
	Projects  []scan.Project `json:"projects"`
}

const version = 1

func Load(cfg config.Config) (Snapshot, bool, error) {
	if cfg.CachePath == "" {
		return Snapshot{}, false, nil
	}

	data, err := os.ReadFile(cfg.CachePath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return Snapshot{}, false, nil
		}
		return Snapshot{}, false, err
	}

	var snapshot Snapshot
	if err := json.Unmarshal(data, &snapshot); err != nil {
		return Snapshot{}, false, err
	}
	if snapshot.Version != version || snapshot.Signature != Signature(cfg) {
		return Snapshot{}, false, nil
	}
	if len(snapshot.Projects) == 0 {
		return Snapshot{}, false, nil
	}
	return snapshot, true, nil
}

func Save(cfg config.Config, projects []scan.Project) error {
	if cfg.CachePath == "" {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(cfg.CachePath), 0o755); err != nil {
		return err
	}

	snapshot := Snapshot{
		Version:   version,
		SavedAt:   time.Now(),
		Signature: Signature(cfg),
		Projects:  projects,
	}
	data, err := json.MarshalIndent(snapshot, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(cfg.CachePath, append(data, '\n'), 0o644)
}

func Signature(cfg config.Config) string {
	payload := struct {
		Roots       []string `json:"roots"`
		Include     []string `json:"include"`
		SkipDirs    []string `json:"skip_dirs"`
		GlobalPaths []string `json:"global_paths"`
	}{
		Roots:       sortedCopy(cfg.Roots),
		Include:     sortedCopy(cfg.Include),
		SkipDirs:    sortedCopy(cfg.SkipDirs),
		GlobalPaths: sortedCopy(cfg.GlobalPaths),
	}
	data, _ := json.Marshal(payload)
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func sortedCopy(values []string) []string {
	out := append([]string(nil), values...)
	sort.Strings(out)
	return out
}
