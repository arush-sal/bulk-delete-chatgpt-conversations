# RFC: Desktop Packaging And Distribution

## Status

Proposed

## Summary

This project currently ships as a CLI/TUI binary. That works for terminal users, but it is not a single-click application on macOS or Windows. This RFC proposes a path to ship a desktop app that users can launch directly without invoking a terminal command, while preserving the existing Go implementation for ChatGPT auth, browser automation, and conversation operations.

## Goals

- Make the app launchable by double-click on macOS and Windows.
- Remove terminal invocation as a user dependency.
- Keep the existing Go business logic reusable.
- Define a packaging and distribution UX that looks normal on each platform.
- Keep the first implementation focused on direct-download distribution rather than app stores.

## Non-Goals

- Implement the desktop app in this PR.
- Change the existing CLI/TUI flow yet.
- Redesign the app's feature set beyond what is needed for desktop launch and update UX.

## Current State

Today the release pipeline produces archives containing the `chatgpt-bulk` binary. On macOS and Windows that still leaves users with terminal-oriented setup and launch steps.

That gap matters because "download, extract, run a command" is materially different from "download, install, double-click".

## Constraints That Shape The Recommendation

- The codebase is already Go-first.
- The app opens and controls a real browser during auth, so desktop packaging must not make browser automation materially harder.
- macOS direct distribution should expect Developer ID signing and notarization to avoid Gatekeeper friction. Apple documents that software distributed outside the Mac App Store is managed by the developer and should be signed with Developer ID and notarized.
- Windows distribution should expect a signed installer. Microsoft positions MSIX as the modern packaging format, but traditional installers remain simpler for direct website/GitHub download.

The last two points are based on current platform guidance from Apple, Microsoft, and the candidate packaging frameworks linked in the references section.

## Options

### Option 1: Keep The Existing CLI Core And Add A Desktop Shell

Implement a real desktop UI that calls shared Go application services, then package it as a normal desktop app.

Candidate frameworks:

- Wails: Go backend plus webview frontend, native app packaging, Windows installer support, and code-signing guidance.
- Fyne: all-Go desktop GUI with built-in packaging commands for `.app` and `.exe`.

Pros:

- Preserves most of the current Go logic.
- Produces a real double-clickable app.
- Lets the project keep one core language for backend behavior.
- Avoids shipping a separate terminal dependency to end users.

Cons:

- Requires a new desktop UI layer; the Bubble Tea TUI is not the end-state UI.
- Requires packaging/signing work for macOS and Windows.
- Requires some refactoring so CLI commands do not directly own all application flow.

### Option 2: Package The Existing CLI/TUI As A Desktop Wrapper Only

Examples:

- macOS `.app` that launches the binary inside Terminal or a bundled terminal emulator.
- Windows installer that adds Start Menu shortcuts to a console executable.

Pros:

- Lowest implementation cost.
- Keeps nearly all existing code untouched.

Cons:

- Does not really satisfy the product goal.
- Still exposes terminal-style UX, stdout/stderr failure modes, and console windows.
- Feels broken or unfinished to non-technical users.

This option should be rejected unless the team only wants a temporary stopgap.

### Option 3: Build A Full Electron App

Move to a Node/Electron desktop shell and bridge into Go services or replace the current app shell entirely.

Pros:

- Mature ecosystem for packaging, installers, and auto-update flows.
- Rich UI flexibility.

Cons:

- Adds a second primary runtime and build stack.
- Larger app size and memory footprint.
- More operational complexity than the repo needs for its current scope.

This is viable, but it is not the best fit for a small Go-first project.

## Comparison

| Option | User UX | Reuse Existing Go Logic | Packaging Effort | Ongoing Complexity | Recommendation |
| --- | --- | --- | --- | --- | --- |
| Desktop shell on top of Go core | High | High | Medium | Medium | Yes |
| Wrapper around existing CLI/TUI | Low | Very high | Low | Low | No |
| Electron app | High | Medium | Medium | High | No |

## Recommended Direction

Build a real desktop shell on top of the current Go core, with Wails as the default choice.

Why Wails:

- It is explicitly designed for desktop apps using Go plus web technologies.
- It can package native binaries for macOS and Windows.
- It already documents Windows installer generation and signing workflows.
- It keeps the app close to the existing Go codebase instead of moving the project into a primarily Node/Electron architecture.

Why not Fyne as the default:

- Fyne is a credible fallback and keeps everything in Go.
- However, this product will likely benefit from a richer and more flexible UI than a direct widget-port of the current TUI.
- Wails gives more room for polished onboarding, auth state display, filtering, previews, and bulk-action confirmation flows.

That last point is an architectural recommendation, not a hard platform requirement.

## Recommended Packaging UX

### macOS

Ship:

- `ChatGPT Bulk.app`
- a signed and notarized DMG as the primary download artifact
- an optional ZIP of the signed `.app` for advanced users

UX target:

- User downloads a DMG from GitHub Releases.
- User drags the app to `Applications`.
- First launch succeeds without bypassing Gatekeeper because the app is signed and notarized.

Why this is the right default:

- DMG is familiar for direct-download Mac apps.
- Direct distribution avoids App Store sandboxing and review overhead.
- The current browser-automation behavior is likely simpler outside Mac App Store constraints.

The App Store is not the recommended first target. Apple notes that outside-App-Store distribution is developer-managed, while the Mac App Store adds sandboxing and store-specific distribution requirements. Given this app's browser-driven auth flow, direct distribution is the lower-risk first milestone.

### Windows

Ship:

- signed NSIS installer as the primary download artifact
- optional portable `.exe` zip for advanced users
- WinGet manifest submission after the direct-download installer is stable

UX target:

- User downloads an installer from GitHub Releases.
- Installer adds Start Menu and desktop shortcuts.
- App launches from Start Menu with no console window.

Why NSIS first instead of MSIX:

- It is a simpler fit for direct GitHub Releases distribution.
- Wails already supports NSIS installer generation.
- It avoids early store/container decisions before the desktop app UX is validated.

Why still consider MSIX later:

- Microsoft positions MSIX as the modern Windows packaging format.
- MSIX is useful if the project later wants Store-style deployment, cleaner enterprise rollout, or MSIX-native update behavior.

## Auto-Update Recommendation

Do not make auto-update part of the first desktop packaging milestone.

Phase it:

1. Ship signed desktop installers.
2. Stabilize install, launch, auth, and uninstall behavior.
3. Add update checks.
4. Only then add in-app automatic update application if still justified.

Reasoning:

- Auto-update increases signing, hosting, rollback, and support complexity.
- The project can get most of the UX win first by shipping installable desktop builds with GitHub Releases as the update source of truth.
- WinGet can provide an update path on Windows before an in-app updater exists.

When auto-update is added later:

- macOS: prefer Sparkle-style update UX or a simple signed "update available" prompt that opens the release download.
- Windows: prefer installer-driven updates first; evaluate WinSparkle or MSIX-native updating only after the installer path is proven.

## Proposed Architecture Changes For A Future Implementation PR

This PR does not implement them, but the future desktop work should likely follow this split:

- Move reusable application flows out of Cobra command handlers into an internal service layer.
- Keep `internal/chatgpt` as the backend integration layer.
- Keep the current CLI/TUI as one frontend over the shared services.
- Add a new desktop frontend as a second frontend over the same services.

That keeps the current CLI viable while enabling a staged desktop rollout.

## Suggested Delivery Plan

### Phase 1: Refactor For Shared App Services

- Extract login, auth-status, logout, list/filter/action flows from CLI command wiring into reusable services.
- Keep behavior unchanged for the CLI.

### Phase 2: Desktop MVP

- Add a Wails app shell.
- Implement desktop login flow, conversation list, search/filter, select-all, archive, and delete.
- Package `.app` and NSIS installer artifacts in CI.

### Phase 3: Signing And Trust

- Add macOS Developer ID signing and notarization.
- Add Windows code signing for installer artifacts.

### Phase 4: Distribution Polish

- Publish DMG and installer artifacts to GitHub Releases.
- Submit a WinGet manifest.
- Add release notes tailored to desktop users.

### Phase 5: Updates

- Add update checks.
- Add in-app updater only if support burden justifies it.

## Tiny Repo Changes That Would Help Later

These are optional for the eventual implementation PR, not needed now:

- Introduce a top-level `internal/app` package for shared app services.
- Separate UI-state types from direct CLI command execution paths.
- Add release artifact naming conventions for desktop builds early, even before the desktop UI lands.

## Decision

Approve a desktop-app direction based on:

- Wails desktop shell
- direct-download DMG on macOS
- direct-download NSIS installer on Windows
- signing and notarization before broad promotion
- WinGet submission after installer stability
- auto-update deferred until after desktop MVP

## References

- Apple: https://developer.apple.com/macos/distribution/
- Apple Developer ID / Gatekeeper: https://developer.apple.com/developer-id/
- Microsoft MSIX overview: https://learn.microsoft.com/en-us/windows/msix/overview
- Microsoft WinGet repository submission: https://learn.microsoft.com/en-us/windows/package-manager/package/repository
- Microsoft WinGet manifest creation: https://learn.microsoft.com/en-us/windows/package-manager/package/manifest
- Wails introduction: https://wails.io/docs/introduction/
- Wails Windows installer guide: https://wails.io/docs/guides/windows-installer/
- Wails code-signing guide: https://wails.io/docs/guides/signing/
- Fyne desktop packaging: https://docs.fyne.io/started/packaging/
- Electron distribution overview: https://www.electronjs.org/docs/latest/tutorial/distribution-overview
