# Repository Guidelines

## Project Structure & Module Organization

WolfBBS is a Go BBS with terminal, web, IRC, and mail interfaces. Executables live in `cmd/`, including `wolfbbs`, `wolfbbs-web`, `wolfbbs-irc`, and `oputil`. Shared services live in `internal/`; persistence code is in `internal/repository/`, with SQL migrations in `migrations/`. Door definitions live in `doors/`, menu configuration in `menus/`, and web assets in `cmd/wolfbbs-web/assets/`. Go tests sit alongside source files. Browser tests live in `e2e/web/`; operational documentation and automation live in `docs/` and `scripts/`.

## macOS Toolchain & Dependency Preferences

Use **fnm first for Node.js**, **Homebrew next for other dependencies**, and **official vendor binaries when needed**. Check `fnm current`, `fnm list`, and `command -v node` before assuming Node is missing or installing another runtime. Node and npm normally resolve through fnm's per-shell PATH.

Shell setup enables fnm switching on directory changes, recursive version-file lookup, `engines.node` resolution, and Corepack. Respect project version declarations; `e2e/web/.node-version` selects Node 24. For automation, use `fnm exec --using=24 <command>` or initialize the shell with `eval "$(fnm env --shell zsh)"` and run `fnm use` from the project directory. Prefer an installed fnm version over installing Homebrew Node. Use `brew` for remaining macOS dependencies before downloading standalone binaries.

For Node-related failures, check the active version and project requirements before changing code. Use fnm to compare the same failing command under the declared version and relevant older/newer versions, for example `fnm exec --using=24 npm test` from `e2e/web/`. Keep comparisons isolated from production and leave the global default unchanged. Record Node/npm versions and results; a version-specific failure is evidence to investigate, not proof that the application is broken. Use this approach to assess compatibility across users' environments and future runtimes.

## Build, Test, and Development Commands

Use the Go version and toolchain declared in `go.mod`.

- `go build ./...`: compile all packages and executables.
- `go test ./...`: run the Go test suite.
- `go run ./cmd/wolfbbs`: start the BBS using your development configuration.
- `scripts/verify.sh --fast`: run static acceptance checks without starting containers.
- `scripts/verify.sh --smoke`: start a Docker Compose stack and check service health and ports.
- `scripts/run-e2e.sh`: run terminal and Playwright browser checks.
- `scripts/build.sh --quick`: combine Go tests/build, fast verification, and terminal E2E; `--full` adds smoke and browser checks.

## Coding Style & Naming Conventions

Format Go changes with `gofmt`; use its standard tab indentation. Follow existing package boundaries, lowercase package names, exported `PascalCase` identifiers, and unexported `camelCase` identifiers. Name Go tests `*_test.go` and test functions `TestXxx`. Match surrounding shell and JavaScript style; preserve executable permissions on scripts.

## Testing Guidelines

Use Go's `testing` package for unit and integration tests, Playwright for browser flows, and the terminal automation scripts for SSH/TUI behavior. Add regression coverage for changed behavior. No numeric coverage threshold is specified in `CONTRIBUTING.md`. Run Go tests and fast verification before submitting; run smoke checks and affected UI E2E tests, or explain each skip.

## Commit & Pull Request Guidelines

Recent commits use concise imperative subjects, such as “Fix board startup” and “Brand the SSH welcome screen.” Keep changes focused. Explain the problem, resulting behavior, validation, and skipped checks in PR descriptions; link relevant issues and include screenshots for visible UI changes. Update documentation when public behavior changes. Contributions use the MIT license.

## Human-Facing Documentation

This fork required substantial recovery work to become usable. Maintain the documentation as part of maintaining the software so future users can install, recover, and operate the board without repeating that investigation. Preserve upstream credit while clearly describing this fork's behavior.

Update `README.md` and affected guides in `docs/` alongside changes to installation, configuration, user flows, or recovery. Keep instructions understandable to first-time callers and sysops: explain prerequisites, give commands in execution order, describe expected results, and provide a clear next step when something fails. Verify commands, links, UI labels, screenshots, and version claims against the current implementation. Distinguish development/test instructions from live-board operations, explain disruptive actions before presenting them, and avoid conflicting guidance across documents. Documentation changes are part of completing a behavior change, not optional follow-up work.

## Security & Configuration Tips

Use `.env.example` as a configuration reference; keep credentials, private keys, and runtime databases out of commits. Run services and smoke tests with isolated data and ports. Back up before installer or database changes, and avoid disrupting the live board.
