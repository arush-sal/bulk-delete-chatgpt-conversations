# Authentication

`chatgpt-bulk` now uses a login-first flow.

## Recommended flow

1. Run `chatgpt-bulk login`.
2. Choose whether the login should be:
   - permanent: save auth at the default auth file location for future runs
   - short-lived: keep auth in-memory only and open the TUI for the current session
3. Complete sign-in or any browser challenge in the opened ChatGPT window.
4. Permanent auth is stored at:
   - Linux/macOS: `~/.config/chatgpt-bulk/auth.json`
   - Windows: `%AppData%/chatgpt-bulk/auth.json`
5. Future `chatgpt-bulk` runs reuse the stored auth automatically when that file exists.

## Commands

- `chatgpt-bulk login`: browser auth, then choose permanent or short-lived auth
- `chatgpt-bulk login --permanent`: skip the prompt and save auth to the default auth file
- `chatgpt-bulk login --session-only`: skip the prompt, keep auth in-memory only, and open the TUI
- `chatgpt-bulk auth status`: inspect stored auth availability
- `chatgpt-bulk logout`: remove stored auth only
- `CHATGPT_BULK_AUTH_FILE=/path/to/auth.json`: override the default auth file location for local testing or automation

## No Token Fallbacks

`chatgpt-bulk` no longer accepts ChatGPT auth through environment variables or direct token flags.
If the auth file is missing and you run `chatgpt-bulk` in an interactive terminal, the app will:

1. tell you the auth file is missing
2. remind you that `chatgpt-bulk login` is the short-lived login path
3. ask whether to create a permanent auth file
4. continue with in-memory auth for that session if you decline

## Security

- Auth is stored with restrictive file permissions where supported.
- The tool never prints full token values.
- Browser profile data is temporary and removed when auth completes.

## Testing notes

- `go test ./...` covers auth-state persistence, command wiring, and an end-to-end-style `chatgpt-bulk login` command test using a stubbed auth client.
- For repeatable live verification, run the Go-based manual harness documented in [docs/manual-auth-e2e.md](./manual-auth-e2e.md).
- Automated tests still do not complete a real ChatGPT sign-in; browser login and challenges remain manual.
