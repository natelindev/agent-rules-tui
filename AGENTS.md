# AGENTS.md

This repository builds `agent-rules`, a Go Bubble Tea terminal UI for discovering and editing agent instruction files.

Development notes:

- Keep the default editor as `nvim`, with config/env/flag overrides.
- Keep projects collapsed by default; project rows toggle expansion, file rows open the editor.
- Persist config under `~/.config/agent-rules-tui/config.json`, honoring `XDG_CONFIG_HOME`.
- Use stale-while-revalidate discovery cache under `~/.cache/agent-rules-tui/discovery.json`, honoring `XDG_CACHE_HOME`.
- Keep project rows sorted newest-first by the newest agent file in each project.
- If the running binary is not available in PATH, prompt in the TUI to add a symlink under `~/.local/bin`; support ignore-once and remembered-ignore choices.
- Prefer focused tests for scan behavior, config behavior, and TUI row/selection state.
- Run `gofmt -w` on edited Go files, then `go test ./...`, `go build ./cmd/agent-rules`, and use `agent-rules --warm-cache` for non-interactive scan/cache smoke checks.
