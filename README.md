# agent-rules-tui

A small terminal UI for finding and opening agent instruction and memory files such as `AGENTS.md`, `CLAUDE.md`, `GEMINI.md`, `.cursorrules`, Cursor rule files, Cline/Roo/Windsurf rules, Codex instructions, and related config files.

It groups matches by project, keeps known global memory paths in a `Global` group, and opens files in a configurable editor. Projects are collapsed by default; click a project or press `enter` on it to expand.

Discovery results are cached, so the app can show the previous result set immediately and refresh it in the background.

## Install

```sh
go install ./cmd/agent-rules
```

From this checkout you can also run it directly:

```sh
go run ./cmd/agent-rules
```

## Usage

```sh
agent-rules
agent-rules --root ~/work --root ~/src
agent-rules --editor vim
agent-rules --editor "code -w"
agent-rules --editor fresh
```

Keys:

- `a`: when prompted, add `agent-rules` to `~/.local/bin` and update your shell PATH if needed
- `i`: dismiss the shell install prompt for this session
- `I`: remember that you want to ignore the shell install prompt
- `click` on a project or `enter`: expand/collapse the project
- `click` on a file or `enter`: open the selected file
- `j/k` or arrow keys: move
- `/`: filter
- `r`: rescan
- `c`: open the config file
- `q`: quit

## Config

The config is persisted automatically on first run. By default it lives at:

```text
~/.config/agent-rules-tui/config.json
```

If `XDG_CONFIG_HOME` is set, the app uses:

```text
$XDG_CONFIG_HOME/agent-rules-tui/config.json
```

Useful config commands:

```sh
agent-rules --init-config
agent-rules --print-config
agent-rules --warm-cache
```

Example:

```json
{
  "editor": "nvim",
  "roots": ["~"],
  "include": ["AGENTS.md", "CLAUDE.md", "GEMINI.md"],
  "skip_dirs": ["node_modules", ".git", "Library"],
  "global_paths": ["~/.claude/CLAUDE.md", "~/.codex/AGENTS.md"]
}
```

Cache files are stored at `~/.cache/agent-rules-tui/discovery.json`, or `$XDG_CACHE_HOME/agent-rules-tui/discovery.json` when `XDG_CACHE_HOME` is set. The cache is stale-while-revalidate: cached results render first, then a background scan updates the UI and cache.

Use `agent-rules --warm-cache` to refresh the discovery cache without opening the TUI.

If the running `agent-rules` binary is not available from your shell `PATH`, the TUI prompts you to add it. Accepting creates or updates a symlink at `~/.local/bin/agent-rules` and appends a small PATH block to your shell rc file when `~/.local/bin` is not already in `PATH`. You can ignore once or remember the ignore choice in config with `ignore_path_prompt`.

Editor precedence is:

1. `--editor`
2. config file `editor`
3. `AGENT_RULES_EDITOR`
4. `AGENT_MEM_EDITOR`
5. `VISUAL`
6. `EDITOR`
7. `nvim`

Use `vscode` as shorthand for `code -w`, and `neovim` as shorthand for `nvim`.
