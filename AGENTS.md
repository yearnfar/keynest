# AGENTS.md

This file is the working guide for AI coding agents in this repository. Read it before changing code.

## Project Summary

Keynest is a local desktop password manager compatible with KeePass KDBX 4.1 files. It uses:

- Go 1.25 and Wails v2 for the desktop shell and backend API.
- React 19, TypeScript, and Vite for the UI.
- `gokeepasslib/v3` for KDBX parsing and encryption.
- `lucide-react` for interface icons.

Vault files stay at paths chosen by the user. The app does not have a cloud backend. Master passwords are not persisted; the recent-vault store contains file paths only.

## Repository Map

- `main.go`: Wails application bootstrap, window options, embedded frontend assets, and backend binding.
- `app.go`: public Wails bridge. It owns file dialogs, maps frontend DTOs to the vault package, and exposes methods to JavaScript.
- `darwin.go`: macOS-specific cgo linker configuration.
- `internal/keynest/`: KDBX domain and persistence layer.
  - `service.go`: synchronized vault lifecycle and mutations.
  - `database.go`: credentials, KDBX encode/decode, atomic writes, and model mapping.
  - `recycle_bin.go`: recycle-bin traversal, retention, restore, and deletion helpers.
  - `recent.go`: recent database paths in the OS user config directory.
  - `types.go`: package-level request and response types.
  - `service_test.go`: backend integration tests using temporary KDBX files.
- `internal/updater/`: GitHub Release discovery, version comparison, platform asset selection, checksum verification, and executable replacement.
- `frontend/src/App.tsx`: unlocked-vault UI and most application state/actions.
- `frontend/src/Launcher.tsx`: open, create, and unlock flows; browser-only demo fallback.
- `frontend/src/PasswordGenerator.tsx`: password generation using Web Crypto.
- `frontend/src/KeePassGroupIcon.tsx`: KeePass group icon mapping.
- `frontend/src/App.css` and `frontend/src/style.css`: application styling.
- `frontend/wailsjs/`: generated Wails bindings. Do not edit these files by hand.
- `frontend/dist/` and `build/bin/`: generated build output; both are ignored by Git.
- `build/`: Wails platform metadata and application icons.

## Runtime Flow

The main data path is:

1. React calls a generated function from `frontend/wailsjs/go/main/App`.
2. A public method in `app.go` validates UI-specific input or opens a native dialog.
3. The method delegates to `internal/keynest.Service`.
4. The service holds its mutex, mutates the in-memory KDBX database, and persists it.
5. Go DTOs are returned over the Wails bridge and applied to React state.

Keep UI concerns in the frontend, native-dialog and bridge concerns in `app.go`, and KDBX rules in `internal/keynest`.

## Security And Data Invariants

Treat these as hard constraints:

- Never log, persist, add fixtures containing, or send elsewhere a master password, key-file content, entry password, or decrypted vault data.
- Do not add real `.kdbx`, `.key`, or `.keyx` files to Git. Tests must create data under `t.TempDir()`.
- Keep password fields protected in KDBX. `writeDatabase` must lock protected entries before encoding and unlock them afterward.
- Preserve atomic vault writes: encode to a temporary file beside the destination, set mode `0600`, flush and close it, then rename it over the destination.
- Keep all mutable `keynest.Service` state behind its mutex. Do not expose the underlying database to callers.
- Preserve support for password-only, key-file-only, and combined credentials.
- A changed password creates a recycle-bin copy of the old entry. Preserve `PreviousParentGroup`, timestamps, restore behavior, permanent-delete metadata, and the 30-day retention rule.
- Do not allow new entries or groups to target the recycle-bin subtree.
- Recent-vault persistence may contain local paths, but never credentials or decrypted entry fields.
- Password generation must continue to use `crypto.getRandomValues`; do not replace it with `Math.random`.
- Automatic updates must remain HTTPS-only, restricted to trusted GitHub hosts, and verified against the matching `.sha256` asset before executable replacement.

## Generated Code And Contracts

The exported methods and DTOs in `app.go` define the frontend/backend contract. When that contract changes:

1. Update the Go bridge and the vault package types/mapping together.
2. Update frontend types and call sites.
3. Regenerate `frontend/wailsjs/` through the normal Wails development or build workflow. Do not patch generated bindings manually.
4. Verify both Go tests and the TypeScript production build.

The browser preview has no Wails bridge. Preserve the `hasWailsBridge()` checks and demo behavior so `npm run dev` remains useful.

## Development Commands

Run the desktop app with hot reload:

```bash
wails dev
```

Run only the browser frontend:

```bash
cd frontend
npm run dev
```

Required checks for backend or shared-contract changes:

```bash
gofmt -w <changed-go-files>
go test ./...
cd frontend && npm run build
```

For desktop integration or release-facing changes, also run:

```bash
wails build
```

There is currently no frontend test script. Do not claim frontend tests passed when only `npm run build` was run.

## Change Guidelines

- Prefer a focused change that follows existing package and component boundaries.
- Add or extend Go tests for KDBX mutations, persistence, credential handling, group operations, or recycle-bin behavior.
- Add user-facing frontend text to `frontend/src/locales/zh-CN.ts` and `frontend/src/locales/en.ts`; keep Simplified Chinese wording consistent with the existing interface.
- Preserve JSON field names used by the Wails bridge, especially camelCase fields such as `groupId`, `parentId`, and `iconId`.
- Use existing Lucide icons and established CSS patterns for UI additions.
- Keep keyboard, focus, disabled, loading, empty, error, and browser-demo states working when changing a workflow.
- Avoid unrelated formatting or refactors in `App.tsx`; it is large and broad edits make behavioral review difficult.
- Do not edit ignored build artifacts or commit local vault files.

## Verification By Change Type

- Vault logic: add focused cases in `internal/keynest/service_test.go`, then run `go test ./...`.
- Bridge/API shape: run `go test ./...`, regenerate Wails bindings, then run the frontend build.
- Frontend behavior or styling: run `cd frontend && npm run build`; use `wails dev` when the workflow depends on native APIs.
- Password generation: verify every enabled character class is represented, length is exact, the all-disabled guard remains valid, and randomness still comes from Web Crypto.
- Packaging or platform metadata: run `wails build` on the affected platform.
- Updater behavior: run `go test ./internal/updater`; release assets must keep the names documented in `README.md` and `.github/workflows/release.yml`.

## Definition Of Done

A change is complete when behavior is implemented at the correct layer, relevant tests cover regressions, generated bindings are current when needed, no secrets or local vault data were introduced, and the applicable commands above pass.
