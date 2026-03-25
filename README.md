<!-- ![Logo](./assets/transparent-background.png) -->
<p align="center">
    <img src="./assets/transparent-background.png" alt="logo" height="200" />
</p>

![GitHub stars](https://img.shields.io/github/stars/arush-sal/bulk-delete-chatgpt-conversations?style=social)
![GitHub forks](https://img.shields.io/github/forks/arush-sal/bulk-delete-chatgpt-conversations?style=social)
![License](https://img.shields.io/github/license/arush-sal/bulk-delete-chatgpt-conversations)
![Last Commit](https://img.shields.io/github/last-commit/arush-sal/bulk-delete-chatgpt-conversations)

<p align="center">
  <image src="assets/demo-regular.gif" width="780" height="480"/>
</p>

A browser-assisted Go TUI for cleaning up your ChatGPT history without clicking through the sidebar one conversation at a time.

## What It Does

- Lists all conversations in your ChatGPT account with pagination
- Lets you filter, sort, and multi-select conversations in a TUI
- Supports bulk archive and bulk delete actions
- Uses a real browser session up front to avoid the `403`/bot-protection problems common with plain HTTP scripts
- Shows auth/debug progress directly inside the TUI when `--debug` is enabled

It opens a temporary Chrome window, validates your ChatGPT session there, captures an access token, closes that browser window automatically, and then lets you bulk archive or bulk delete conversations from a terminal interface.

## Supported Platforms

| Platform | Architecture | Status |
|----------|-------------|--------|
| macOS | Apple Silicon (arm64) | Supported |
| macOS | Intel (amd64) | Supported |
| Linux | amd64 | Supported |
| Linux | arm64 | Supported |

## Requirements

- Go 1.24+ (only if building from source)
- Chrome, Edge, Chromium, or Brave installed
- A valid ChatGPT web login session

## Installation

### Pre-built binaries

Download the latest release for your platform from the [Releases](https://github.com/arush-sal/bulk-delete-chatgpt-conversations/releases) page:

| Platform | File |
|----------|------|
| macOS Apple Silicon | `chatgpt-bulk_*_darwin_arm64.tar.gz` |
| macOS Intel | `chatgpt-bulk_*_darwin_amd64.tar.gz` |
| Linux amd64 | `chatgpt-bulk_*_linux_amd64.tar.gz` |
| Linux arm64 | `chatgpt-bulk_*_linux_arm64.tar.gz` |

**Recommended one-line install via [bin](https://github.com/marcosnils/bin) and GitHub Releases**

```bash
curl -fsSL https://raw.githubusercontent.com/arush-sal/bulk-delete-chatgpt-conversations/master/scripts/install-via-bin.sh | sh
```

The script:

- installs `bin` into `~/.local/bin` when `bin` is not already available
- downloads `chatgpt-bulk` from this repo's latest GitHub release into `~/.local/bin/chatgpt-bulk` through `bin install`
- prefixes `~/.local/bin` into `PATH` while bootstrapping so first-time `bin` installs stay non-interactive

If you already have `bin`, install directly from releases with:

```bash
mkdir -p "$HOME/.local/bin" && bin install github.com/arush-sal/bulk-delete-chatgpt-conversations "$HOME/.local/bin/chatgpt-bulk"
```

If GitHub rate-limits anonymous release API requests in your environment, export `GITHUB_AUTH_TOKEN` before running `bin install`.

If you want `bin` bootstrapped somewhere else, set `BIN_INSTALL_DIR` before running the script:

```bash
curl -fsSL https://raw.githubusercontent.com/arush-sal/bulk-delete-chatgpt-conversations/master/scripts/install-via-bin.sh | BIN_INSTALL_DIR="$HOME/bin" sh
```

**macOS / Linux/ Windows:**

```bash
# Download and extract (replace the filename for your platform)
tar -xzf chatgpt-bulk_*_darwin_arm64.tar.gz
chmod +x chatgpt-bulk

# macOS only: remove quarantine flag if Gatekeeper blocks it
xattr -d com.apple.quarantine ./chatgpt-bulk

# Verify
./chatgpt-bulk --version
```

**Optional:** move the binary to a directory in your `$PATH`:

```bash
sudo mv chatgpt-bulk /usr/local/bin/
```

### From source

```bash
go install github.com/arush-sal/bulk-delete-chatgpt-conversations/cmd/chatgpt-bulk@latest
```

Or clone and build:

```bash
git clone https://github.com/arush-sal/bulk-delete-chatgpt-conversations.git
cd bulk-delete-chatgpt-conversations
make build
./chatgpt-bulk --version
```

## Authentication

This tool does **not** use an OpenAI API key. It authenticates against the ChatGPT web backend using your ChatGPT browser session.

> - Runs locally in your browser.
> - No extensions required.
> - No data leaves your machine.
> - Open source and inspectable.

### Recommended flow

Run:

```bash
chatgpt-bulk login
```

The tool opens Chrome or a compatible browser, asks whether you want a permanent auth file, then waits for you to sign in or finish any challenge.

- Choose permanent auth to save the minimum required auth state locally for future runs.
- Choose the short-lived option to keep auth in-memory only and open the TUI for the current session.

Stored auth location:

- Linux/macOS: `~/.config/chatgpt-bulk/auth.json`
- Windows: `%AppData%/chatgpt-bulk/auth.json`

Useful auth commands:

```bash
chatgpt-bulk login
chatgpt-bulk login --permanent
chatgpt-bulk login --session-only
chatgpt-bulk auth status
chatgpt-bulk logout
```

Additional auth details are in [docs/authentication.md](./docs/authentication.md).
For repeatable manual-assisted live verification, use the harness in [docs/manual-auth-e2e.md](./docs/manual-auth-e2e.md). With no stage flags it runs the full missing-auth, session-only, and permanent-auth suite.

## Usage Guide

### Quick start

```bash
chatgpt-bulk login
chatgpt-bulk
```

Or if running from source:

```bash
go run ./cmd/chatgpt-bulk --help
go run ./cmd/chatgpt-bulk login
go run ./cmd/chatgpt-bulk
```

### What happens on launch

1. **Stored auth check** -- The app first looks for the default auth file.
2. **Missing auth handling** -- If the auth file is missing and the terminal is interactive, the app tells you the file is missing, reminds you to use `chatgpt-bulk login` for short-lived sessions, and asks whether to create a permanent auth file.
3. **Browser refresh if needed** -- A temporary Chrome window opens and navigates to `chatgpt.com`.
4. **Session validation** -- The app waits for ChatGPT to return a valid access token. If Chrome shows a login page or challenge, complete it there.
5. **Browser closes** -- Once a valid access token is captured, the temporary Chrome window closes automatically.
6. **TUI loads** -- Your conversations are fetched and displayed in the terminal.

### CLI flags

| Flag | Description | Default |
|------|-------------|---------|
| `--chrome-path` | Path to Chrome/Edge/Brave executable | Auto-detected |
| `--headless` | Run Chrome without a visible window | `false` |
| `--debug` | Show verbose debug logs in the TUI | `false` |
| `--version` | Print version and exit | |
| `--help` | Print help and exit | |

Login flags:

| Flag | Description |
|------|-------------|
| `--permanent` | Save auth to the default auth file without prompting |
| `--session-only` | Keep auth in-memory only and open the TUI for this session |

### Auth command examples

```bash
# Prompt for permanent vs short-lived auth through an interactive browser login
chatgpt-bulk login

# Save local auth without prompting
chatgpt-bulk login --permanent

# Open a short-lived in-memory session
chatgpt-bulk login --session-only

# Inspect stored auth availability
chatgpt-bulk auth status

# Remove stored auth
chatgpt-bulk logout
```

### Examples

```bash
# Basic usage
chatgpt-bulk

# With debug output to see what's happening
chatgpt-bulk --debug

# Point to a specific browser
chatgpt-bulk --chrome-path "/Applications/Brave Browser.app"

# Headless mode (no browser window appears)
chatgpt-bulk --headless

# macOS with Chrome in default location
chatgpt-bulk --chrome-path "/Applications/Google Chrome.app"

# Linux with Chromium
chatgpt-bulk --chrome-path /usr/bin/chromium

```

## TUI Controls

### Selection screen

| Key | Action |
|-----|--------|
| `↑` / `k` | Move cursor up |
| `↓` / `j` | Move cursor down |
| `space` | Toggle selection on current conversation |
| `a` | Select all / deselect all |
| `/` | Start typing a filter |
| `s` | Cycle sort mode (Date descending, Date ascending, Title A-Z, Title Z-A) |
| `enter` | Proceed to action picker |
| `q` / `Ctrl+C` | Quit |

### Filter mode (after pressing `/`)

| Key | Action |
|-----|--------|
| Type text | Filter conversations by title |
| `enter` | Accept filter and return to selection |
| `esc` | Cancel filter |
| `backspace` | Delete last character |

### Action picker

| Key | Action |
|-----|--------|
| `↑` / `k` | Move up |
| `↓` / `j` | Move down |
| `enter` | Choose highlighted action |
| `esc` | Go back to selection |

### Available actions

- **Archive** -- Hides selected conversations without deleting them. They can be restored from the ChatGPT sidebar under "Archived chats".
- **Delete** -- Marks conversations as not visible. This may be difficult to recover.
- **Cancel** -- Return to the selection screen.

### Confirmation screen

| Key | Action |
|-----|--------|
| `y` / `enter` | Confirm and execute |
| `n` / `esc` | Go back |

## How It Works

1. The app launches a temporary Chrome window with a fresh profile directory.
2. It seeds the `__Secure-next-auth.session-token` cookie using the Chrome DevTools Protocol.
3. It navigates to `chatgpt.com` and polls the session API until a valid access token is returned.
4. Once the access token is obtained, the Chrome window closes automatically.
5. All subsequent API calls (listing, archiving, deleting) use standard HTTP requests with the captured access token.
6. The temporary Chrome profile directory is cleaned up on exit.

This tool does not use an OpenAI API key. It talks to the ChatGPT web backend (`chatgpt.com/backend-api`).

## Browser Detection

If `--chrome-path` is not provided, the app searches these locations automatically:

**macOS (Apple Silicon and Intel):**

- `/Applications/Google Chrome.app/Contents/MacOS/Google Chrome`
- `/Applications/Google Chrome Canary.app/Contents/MacOS/Google Chrome Canary`
- `/Applications/Chromium.app/Contents/MacOS/Chromium`
- `/Applications/Microsoft Edge.app/Contents/MacOS/Microsoft Edge`
- `/Applications/Brave Browser.app/Contents/MacOS/Brave Browser`

**Linux:**

- `google-chrome`, `google-chrome-stable`, `chromium`, `chromium-browser` (via `$PATH`)
- `/usr/bin/google-chrome`, `/usr/bin/chromium`, `/snap/bin/chromium`

## Troubleshooting

### Chrome launches but auth does not complete

- Keep the terminal open.
- Finish logging in inside the temporary Chrome window that appeared.
- Rerun with `--debug` to see step-by-step logs in the TUI.

### Session token expired or invalid

- ChatGPT web sessions expire periodically. If you get a `401` error, rerun `chatgpt-bulk login` and complete the browser flow again.
- If you want the next launch to reuse auth automatically, choose the permanent auth-file option during login.

### 403 Forbidden / Cloudflare challenge

- This usually means the session needs a browser challenge. Run without `--headless` so the Chrome window is visible, complete any challenges, and let the app capture the token.

### Chrome is installed in a non-standard location

```bash
chatgpt-bulk --chrome-path "/full/path/to/browser"
```

On macOS, `--chrome-path` accepts either the `.app` bundle path or the executable inside
`Contents/MacOS`.

### macOS: "App is damaged" or Gatekeeper warning

If you download a pre-built binary and macOS blocks it:

```bash
xattr -d com.apple.quarantine ./chatgpt-bulk
```

### macOS: Chrome not found

If Chrome is installed but not detected, pass the path explicitly:

```bash
chatgpt-bulk --chrome-path "/Applications/Google Chrome.app"
```

### I want more visibility during startup

```bash
chatgpt-bulk --debug
```

This shows timestamped debug logs directly in the TUI, including cookie seeding, navigation steps, session polling, and API responses.

## Building from Source

```bash
git clone https://github.com/arush-sal/bulk-delete-chatgpt-conversations.git
cd bulk-delete-chatgpt-conversations
```

| Command | Description |
|---------|-------------|
| `make build` | Build binary for current platform |
| `make install` | Install to `$GOPATH/bin` |
| `make test` | Run tests |
| `make vet` | Run `go vet` |
| `make fmt` | Format code |
| `make snapshot` | Build release archives for all platforms (requires goreleaser) |
| `make clean` | Remove build artifacts |

### Cross-compiling

The project uses `CGO_ENABLED=0`, so cross-compilation works out of the box:

```bash
# Build for macOS Apple Silicon from any platform
CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 go build -o chatgpt-bulk ./cmd/chatgpt-bulk

# Build for macOS Intel
CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 go build -o chatgpt-bulk ./cmd/chatgpt-bulk

# Build for Linux
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o chatgpt-bulk ./cmd/chatgpt-bulk
```

## Notes

- Archive uses `PATCH /backend-api/conversation/{id}` with `{"is_archived": true}`.
- Delete uses `PATCH /backend-api/conversation/{id}` with `{"is_visible": false}`.
- ChatGPT's internal web APIs may change without notice.
- The access token is kept in memory only and never written to disk.
- The temporary Chrome profile is deleted when the app exits.

## Reference

Inspired by [`iacob28/chatgpt_python_bulk_update`](https://github.com/iacob28/chatgpt_python_bulk_update), but implemented as a native Go TUI with a browser-assisted auth handoff.
