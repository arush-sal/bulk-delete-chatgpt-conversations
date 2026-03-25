# Manual-Assisted Auth E2E Harness

This repo includes a Go-based helper for repeatable live auth checks on different machines:

```bash
go run ./cmd/chatgpt-bulk-auth-e2e
```

The harness automates the surrounding setup and verification, but it does **not** automate your ChatGPT sign-in. You still need to complete browser login and any Cloudflare or MFA challenges yourself.

With no stage flags, the harness now runs the full suite in this order:

1. missing-auth-file / no-auth-file behavior
2. temporary/session-only auth
3. permanent auth

## What It Automates

Missing-auth-file / no-auth-file flow:

1. removes the auth file under test
2. runs `chatgpt-bulk` without stored auth in a non-interactive context
3. verifies that the command reports the missing auth file and points you to `chatgpt-bulk login`

Temporary/session-only flow:

1. removes the temp auth file again
2. runs an in-memory-only browser auth through the Go client
3. confirms auth succeeds
4. confirms no auth file was written

Permanent auth-file flow:

1. builds a temporary `chatgpt-bulk` binary unless you pass `--binary`
2. points `CHATGPT_BULK_AUTH_FILE` at a temporary auth file, removing any existing file first
3. launches `chatgpt-bulk login --permanent`
4. waits for the login command to finish after you complete browser sign-in
5. verifies that the temporary auth file was created
6. runs `chatgpt-bulk auth status` and checks for `Stored auth: present`
7. verifies saved-auth reuse with the same `ResolveAuth -> New -> Authenticate` path the main app uses before the TUI starts

## What Remains Manual

- completing ChatGPT sign-in in the opened browser
- completing any MFA or anti-bot challenge
- deciding whether the browser window is using the right account

## Commands

Default full-suite check:

```bash
go run ./cmd/chatgpt-bulk-auth-e2e
```

Run only the missing-auth-file / no-auth-file stage:

```bash
go run ./cmd/chatgpt-bulk-auth-e2e --missing-auth
```

Run only the session-only stage:

```bash
go run ./cmd/chatgpt-bulk-auth-e2e --session-only
```

Run only the permanent-auth stage:

```bash
go run ./cmd/chatgpt-bulk-auth-e2e --permanent
```

Run only a selected subset of stages:

```bash
go run ./cmd/chatgpt-bulk-auth-e2e --missing-auth --permanent
```

Use a specific browser:

```bash
go run ./cmd/chatgpt-bulk-auth-e2e --chrome-path "/full/path/to/browser"
```

Keep the temp binary and auth file for inspection:

```bash
go run ./cmd/chatgpt-bulk-auth-e2e --keep-artifacts
```

Use an already-built binary:

```bash
go run ./cmd/chatgpt-bulk-auth-e2e --binary /path/to/chatgpt-bulk
```

## Flags

- `--auth-file`: use a specific auth file path instead of a temp file
- `--binary`: use an existing `chatgpt-bulk` binary instead of building one
- `--chrome-path`: forward a browser path to the launched auth flow
- `--debug`: forward `--debug` to the permanent login command
- `--headless`: forward `--headless` to the launched auth flow
- `--keep-artifacts`: keep the temp auth file and temp binary
- `--missing-auth`: include the missing-auth-file / no-auth-file stage
- `--session-only`: include the temporary/session-only stage
- `--permanent`: include the permanent-auth stage
- `--timeout`: overall timeout for each interactive auth step

## Notes

- Run the harness from the repository root.
- If you pass any stage flag, the harness runs only the explicitly selected stages in the default order.
- The permanent flow uses the real CLI command.
- The saved-auth reuse check intentionally avoids launching the full TUI. It verifies the same stored-auth resolution and client-authentication path the app uses immediately before the TUI starts.
- The temporary/session-only path uses the internal client because `chatgpt-bulk login --session-only` opens the TUI, which is harder to automate as a post-login verification step.
