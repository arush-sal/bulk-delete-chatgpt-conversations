# ChatGPT Bulk Conversation TUI

A Go terminal UI that mirrors the flow from [`iacob28/chatgpt_python_bulk_update`](https://github.com/iacob28/chatgpt_python_bulk_update): authenticate with your ChatGPT browser session, load all conversations, select multiple entries, then bulk archive or bulk delete them.

## Features

- Fetches all conversations with pagination from `https://chatgpt.com/backend-api/conversations`
- Uses your browser session token, but executes requests from a real Chrome session to reduce `403` bot-protection failures
- Bubble Tea TUI for multi-select, action choice, confirmation, and results
- Supports bulk archive and bulk delete
- Optional `.env` loading and debug output

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
HEADLESS=false
CHROME_PATH=/mnt/c/Program Files/Google/Chrome/Application/chrome.exe
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

If your browser is elsewhere, set `CHROME_PATH`.

## Controls

- `↑` / `k`: move up
- `↓` / `j`: move down
- `space`: toggle selection
- `a`: select all / clear all
- `enter`: continue / confirm
- `esc`: go back
- `q`: quit

## Notes

- Archive sends `PATCH /backend-api/conversation/{id}` with `{"is_archived": true}` from inside Chrome.
- Delete sends `PATCH /backend-api/conversation/{id}` with `{"is_visible": false}` from inside Chrome.
- ChatGPT’s internal web API can change without notice.
