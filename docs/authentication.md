# Authentication

`chatgpt-bulk` now uses a login-first flow.

## Recommended flow

1. Run `chatgpt-bulk login`.
2. Complete sign-in or any browser challenge in the opened ChatGPT window.
3. The tool stores the minimum required auth state at:
   - Linux/macOS: `~/.config/chatgpt-bulk/auth.json`
   - Windows: `%AppData%/chatgpt-bulk/auth.json`
4. Future `chatgpt-bulk` runs reuse the stored auth automatically.

## Commands

- `chatgpt-bulk login`: launch browser auth and save local auth state
- `chatgpt-bulk auth status`: inspect stored auth and env-var fallback availability
- `chatgpt-bulk logout`: remove stored auth only
- `CHATGPT_BULK_AUTH_FILE=/path/to/auth.json`: override the default auth file location for local testing or automation

## Non-interactive fallback

Automation can still provide:

- `CHATGPT_SESSION_TOKEN`
- `CHATGPT_CSRF_TOKEN` (optional)

Resolution order is:

1. Stored local auth
2. Environment variables
3. If neither is available, run `chatgpt-bulk login`

## Security

- Auth is stored with restrictive file permissions where supported.
- The tool never prints full token values.
- Browser profile data is temporary and removed when auth completes.

## Testing notes

- `go test ./...` covers auth-state persistence, command wiring, and an end-to-end-style `chatgpt-bulk login` command test using a stubbed auth client.
- Automated tests do not launch a real browser; live browser verification still requires running `chatgpt-bulk login` manually.
