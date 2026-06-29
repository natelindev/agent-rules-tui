# CLAUDE.md

Project context for Claude-style agents:

- The app name is `agent-rules-tui`; the installed command is `agent-rules`.
- The TUI is intentionally project-oriented. Scanning may find many files, so dependency/cache directories should stay skipped by default.
- Opening a file uses Bubble Tea's terminal release/restore path and then explicitly re-enables the alt screen and mouse support.
- Do not auto-expand projects on startup. Filtering may temporarily show matching files across collapsed projects.
- Do not replace the configured editor with a hard-coded command. Support editor strings such as `nvim`, `vim`, `fresh`, and `code -w`.
- Startup should show cached discovery data immediately when available, then refresh from a background scan.
- Projects should be sorted newest-first by their newest agent instruction file.
- The PATH install prompt is intentionally non-blocking and must respect the persisted ignore choice.
