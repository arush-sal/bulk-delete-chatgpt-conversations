# ChatGPT Bulk Conversation TUI

A browser-assisted Go TUI for cleaning up your ChatGPT history without clicking through the sidebar one conversation at a time.

It opens a temporary Chrome window, validates your ChatGPT session there, captures an access token, closes that browser window automatically, and then lets you bulk archive or bulk delete conversations from a terminal interface.

## What It Does

- Lists all conversations in your ChatGPT account with pagination
- Lets you filter, sort, and multi-select conversations in a TUI
- Supports bulk archive and bulk delete actions
- Uses a real browser session up front to avoid the `403`/bot-protection problems common with plain HTTP scripts
- Shows auth/debug progress directly inside the TUI when `--debug` is enabled

## Supported Platforms

| Platform | Architecture | Status |
|----------|-------------|--------|
| macOS | Apple Silicon (arm64) | Supported |
| macOS | Intel (amd64) | Supported |
| Linux | amd64 | Supported |
| Linux | arm64 | Supported |
| Windows via WSL | amd64 | Supported |

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

**macOS / Linux:**

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

This tool does **not** use an OpenAI API key. It authenticates against the ChatGPT web backend using your browser session cookie.

### Step 1: Get your session token

1. Open [https://chatgpt.com](https://chatgpt.com) in your browser and sign in.
2. Open developer tools (`F12` or `Cmd+Option+I` on macOS).
3. Go to **Application** (Chrome/Edge) or **Storage** (Firefox) > **Cookies** > `https://chatgpt.com`.
4. Find the cookie named `__Secure-next-auth.session-token` and copy its value.

### Step 2: Configure the token

**Option A: `.env` file (recommended)**

```bash
cp .env.example .env
```

Edit `.env` and paste your token:

```
CHATGPT_SESSION_TOKEN=eyJhbGciOiJkaXIiLCJlbmMiOi...
```

**Option B: Environment variable**

```bash
export CHATGPT_SESSION_TOKEN="eyJhbGciOiJkaXIiLCJlbmMiOi..."
```

### Optional: CSRF token

Some accounts may need the CSRF token as well. If authentication fails, also copy `__Host-next-auth.csrf-token` from your cookies:

```
CHATGPT_CSRF_TOKEN=abc123...
```

## Usage Guide

### Quick start

```bash
chatgpt-bulk
```

Or if running from source:

```bash
go run ./cmd/chatgpt-bulk
```

### What happens on launch

1. **Browser launch** -- A temporary Chrome window opens and navigates to `chatgpt.com`.
2. **Session validation** -- Your session token is injected as a cookie. The app waits for ChatGPT to return a valid access token. If the session is expired, Chrome will show a login page -- sign in there.
3. **Browser closes** -- Once the access token is captured, the temporary Chrome window closes automatically.
4. **TUI loads** -- Your conversations are fetched and displayed in the terminal.

### CLI flags

| Flag | Description | Default |
|------|-------------|---------|
| `--chrome-path` | Path to Chrome/Edge/Brave executable | Auto-detected |
| `--headless` | Run Chrome without a visible window | `false` |
| `--debug` | Show verbose debug logs in the TUI | `false` |
| `--version` | Print version and exit | |
| `--help` | Print help and exit | |

### Examples

```bash
# Basic usage
chatgpt-bulk

# With debug output to see what's happening
chatgpt-bulk --debug

# Point to a specific browser
chatgpt-bulk --chrome-path "/Applications/Brave Browser.app/Contents/MacOS/Brave Browser"

# Headless mode (no browser window appears)
chatgpt-bulk --headless

# macOS with Chrome in default location
chatgpt-bulk --chrome-path "/Applications/Google Chrome.app/Contents/MacOS/Google Chrome"

# Linux with Chromium
chatgpt-bulk --chrome-path /usr/bin/chromium

# Windows via WSL
chatgpt-bulk --chrome-path "/mnt/c/Program Files/Google/Chrome/Application/chrome.exe"
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

**Windows via WSL:**
- `/mnt/c/Program Files/Google/Chrome/Application/chrome.exe`
- `/mnt/c/Program Files (x86)/Google/Chrome/Application/chrome.exe`
- `/mnt/c/Program Files/Microsoft/Edge/Application/msedge.exe`

## Troubleshooting

### Chrome launches but auth does not complete

- Keep the terminal open.
- Finish logging in inside the temporary Chrome window that appeared.
- Rerun with `--debug` to see step-by-step logs in the TUI.

### Session token expired or invalid

- Session tokens expire periodically. If you get a `401` error, grab a fresh token from your browser cookies.
- Make sure you copy the full value of `__Secure-next-auth.session-token` -- it is a long JWT string.

### 403 Forbidden / Cloudflare challenge

- This usually means the session needs a browser challenge. Run without `--headless` so the Chrome window is visible, complete any challenges, and let the app capture the token.
- You can also try providing both `CHATGPT_SESSION_TOKEN` and `CHATGPT_CSRF_TOKEN`.

### Chrome is installed in a non-standard location

```bash
chatgpt-bulk --chrome-path "/full/path/to/browser"
```

### macOS: "App is damaged" or Gatekeeper warning

If you download a pre-built binary and macOS blocks it:

```bash
xattr -d com.apple.quarantine ./chatgpt-bulk
```

### macOS: Chrome not found

If Chrome is installed but not detected, pass the path explicitly:

```bash
chatgpt-bulk --chrome-path "/Applications/Google Chrome.app/Contents/MacOS/Google Chrome"
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
