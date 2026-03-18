# ChatGPT Bulk Conversation TUI

A browser-assisted Go TUI for cleaning up your ChatGPT history without clicking through the sidebar one conversation at a time.

It opens a temporary Chrome window, validates your ChatGPT session there, captures an access token, closes that browser window automatically, and then lets you bulk archive or bulk delete conversations from a terminal interface.

## What It Does

- Lists all conversations in your ChatGPT account with pagination
- Lets you filter, sort, and multi-select conversations in a TUI
- Supports bulk archive and bulk delete actions
- Uses a real browser session up front to avoid the `403`/bot-protection problems common with plain HTTP scripts
- Shows auth/debug progress directly inside the TUI when `--debug` is enabled

## Authentication

This tool does not use an OpenAI API key. It authenticates against the ChatGPT web backend using your ChatGPT browser cookies and launches Chrome so the requests happen in a real browser context.

Required environment variable:

```bash
CHATGPT_SESSION_TOKEN=...
```

Optional:

```bash
CHATGPT_CSRF_TOKEN=...
DEBUG=true
```

To get the session token:

1. Open `https://chatgpt.com` and sign in.
2. Open browser developer tools.
3. Go to cookies for `https://chatgpt.com`.
4. Copy `__Secure-next-auth.session-token`.

## Run

```bash
cp .env.example .env
go run ./cmd/chatgpt-bulk
```

On Windows + WSL, the app will try to launch Chrome from:

- `/mnt/c/Program Files/Google/Chrome/Application/chrome.exe`
- `/mnt/c/Program Files (x86)/Google/Chrome/Application/chrome.exe`
- `/mnt/c/Program Files/Microsoft/Edge/Application/msedge.exe`

If your browser is elsewhere, set it through `--chrome-path` flag.

## How It Works

1. The app launches a temporary Chrome window.
2. It verifies or refreshes the ChatGPT web session there.
3. It captures the short-lived access token needed for backend requests and saves it in-memory only.
4. It closes the temporary browser window automatically.
5. The TUI loads your conversations and lets you bulk manage them.

This tool does not use an OpenAI API key. It talks to the ChatGPT web backend.

## Requirements

- Go 1.24+
- Chrome or Edge installed
- A valid ChatGPT web login credentials
- Native Linux Chrome paths is supported, Windows/WSL/Mac support would be next

## Quick Start

Run the app with your ChatGPT session token:

```bash
go run ./cmd/chatgpt-bulk
```

Useful optional flags:

```bash
--chrome-path "/mnt/c/Program Files/Google/Chrome/Application/chrome.exe"
--headless
--debug
--version
```

Example:

```bash
go run ./cmd/chatgpt-bulk --debug
```

## What You’ll See

During startup, the TUI will show status updates such as:

- `Launching Chrome window...`
- `Waiting for Chrome debugger on port ...`
- `Chrome debugger connected. Initializing browser session...`
- `Applying ChatGPT cookies in Chrome...`
- `Opening ChatGPT in Chrome...`
- `Checking ChatGPT session in Chrome...`
- `Closing temporary Chrome window...`
- `Fetching conversations from ChatGPT...`

If the temporary Chrome window shows a login page, sign in there and leave the terminal open.

## TUI Controls

- `↑` / `k`: move up
- `↓` / `j`: move down
- `space`: toggle selection
- `a`: select all / clear all
- `/`: start filtering
- `enter`: continue / confirm
- `esc`: back / cancel
- `q`: quit

## Browser Detection

If `--chrome-path` is not provided, the app will look for Chrome or Edge in common locations, including:

- Linux: `google-chrome`, `google-chrome-stable`, `chromium`, `chromium-browser`
- Windows via WSL:
  - `/mnt/c/Program Files/Google/Chrome/Application/chrome.exe`
  - `/mnt/c/Program Files (x86)/Google/Chrome/Application/chrome.exe`
  - `/mnt/c/Program Files/Microsoft/Edge/Application/msedge.exe`

## Notes

- Archive uses `PATCH /backend-api/conversation/{id}` with `{"is_archived": true}`.
- Delete uses `PATCH /backend-api/conversation/{id}` with `{"is_visible": false}`.
- ChatGPT’s internal web APIs may change without notice.

## Troubleshooting

### Chrome launches but auth does not complete

- Keep the terminal open.
- Finish logging in inside the temporary Chrome window.
- Rerun with `--debug` to see step-by-step logs in the TUI.

### Chrome is installed in a non-standard location

- Pass `--chrome-path "/full/path/to/browser"`.

### I want more visibility during startup

- Run with `--debug`.

## Reference

Inspired by [`iacob28/chatgpt_python_bulk_update`](https://github.com/iacob28/chatgpt_python_bulk_update), but implemented as a native Go TUI with a browser-assisted auth handoff.
