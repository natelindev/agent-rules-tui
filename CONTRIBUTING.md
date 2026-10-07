# Contributing

## Development

Use Go 1.24 or newer on macOS or Linux.

```sh
git clone https://github.com/natelindev/agent-rules-tui.git
cd agent-rules-tui
go run ./cmd/agent-rules --root .
```

The command lives in `cmd/agent-rules`. Packages under `internal` handle configuration, discovery, caching, PATH installation, and the TUI. Keep changes focused, preserve keyboard and mouse behavior, and add a regression test for bug fixes when practical.

Before opening a pull request:

```sh
gofmt -w cmd internal
go test ./...
go test -race ./...
go vet ./...
go build ./cmd/agent-rules
```

Run a cache smoke check with isolated configuration and cache paths, so it does not change your normal settings:

```sh
demo_dir=$(mktemp -d)
XDG_CONFIG_HOME="$demo_dir/config" XDG_CACHE_HOME="$demo_dir/cache" \
  ./agent-rules --root . --warm-cache
rm -r "$demo_dir"
```

For TUI changes, also check navigation, project expansion, filtering, editor return, and resizing in an interactive terminal. Report what you verified and any limitations in the pull request.

## Documentation site

`docs/` is a static site. Edit `index.html`, `styles.css`, and `app.js` directly; no frontend build tool is required. Use relative asset URLs so the site works on Cloudflare Pages and under the GitHub Pages mirror's repository subpath.

```sh
python3 -m http.server 8000 --directory docs
```

Open <http://localhost:8000>. Check desktop and narrow mobile widths, keyboard focus, navigation, code copying, and the screenshot. Keep README examples and the site reference aligned with the actual CLI.

The primary [documentation site](https://agent-rules-tui.pages.dev/) is hosted on **Cloudflare Pages**, using the `agent-rules-tui` Direct Upload project. Cloudflare deployments are manual; the current project has no Git integration.

To redeploy, authenticate Wrangler to the Cloudflare account that owns the project with Pages write permission, then upload the static directory:

```sh
npx --yes wrangler@4.148.0 login
npx --yes wrangler@4.148.0 pages deploy docs --project-name agent-rules-tui --branch main
```

You can also upload a ZIP containing the contents of `docs/` through the project's Cloudflare dashboard. Put `index.html` at the ZIP root, alongside `styles.css`, `app.js`, and `assets/`.

For automated uploads from GitHub Actions, configure a Cloudflare API token with **Account → Cloudflare Pages → Edit** for the owning account. See [Cloudflare's continuous deployment guide](https://developers.cloudflare.com/pages/how-to/use-direct-upload-with-continuous-integration/). Do not store credentials in the repository.

The [GitHub Pages mirror](https://natelindev.github.io/agent-rules-tui/) remains available for existing links. The included `docs.yml` workflow refreshes that mirror when `docs/` changes on `main` and can also be run manually. For a fork, enable **Settings → Pages → Build and deployment → Source → GitHub Actions** and update its documentation links.

## Brand assets

The [logo assets and usage notes](docs/assets/brand/README.md) include SVG and transparent PNG icons and wordmarks. Keep the README, docs header, and favicon consistent when changing the identity.

## Screenshot

The README and site share `docs/assets/screenshot.png`. Capture it from the real executable against isolated example projects:

```sh
python3 -m venv /tmp/agent-rules-screenshot-tools
/tmp/agent-rules-screenshot-tools/bin/pip install -r scripts/screenshot-requirements.txt
go build -o agent-rules ./cmd/agent-rules
/tmp/agent-rules-screenshot-tools/bin/python scripts/capture-screenshot.py
```

The capture script uses a pseudo-terminal and renders its screen buffer. It does not scan your home directory, read real agent instructions, launch an editor, or modify your shell configuration. You can supply a monospaced font with `--font /path/to/font.ttf`.

## Reporting issues

Include your OS, terminal, Go version, the command you ran, and steps to reproduce. For discovery problems, include a small example directory structure and relevant config fields. Remove private file contents and personal paths before sharing output.
