# CLAUDE.md — configurer

Project memory for Claude Code. Read this first; it captures architecture, conventions,
gotchas, and the state of in-flight work so we don't re-learn the codebase each session.

## What this is
`configurer` is a Go CLI + library that loads secrets from providers into environment
variables, optionally runs a command with that environment, and can write secrets back.
Module: `github.com/thalesfsp/configurer`. Entry point: `main.go` → `cmd.Execute()` (Cobra).

## Layout
- `cmd/` — Cobra commands. `load.go` (alias `l`) + per-provider subcommands (`vault.go`,
  `awssm.go`, `awsssm.go`, `dotenv.go`, `github-w.go`, `noop.go`, `text.go`). `write.go`
  (alias `w`) for write subcommands. `utils.go` has `runCommand`/`ConcurrentRunner` (runs
  the child process, optional ElasticSearch log output). `start-local.go`/`bridge.go` =
  SSH tunnel ("bridge") via `thalesfsp/mole`.
- `provider/` — `IProvider` interface + base `Provider` (Logger/Name/Override/RawValue) +
  `ExportToEnvVar` (respects Override/RawValue; existing env vars win unless Override).
- Providers: `vault/`, `awssm/` (Secrets Manager), `awsssm/` (SSM Param Store), `dotenv/`,
  `github/` (write-only: Load returns `ErrNotSupported`), `noop/`.
- `parser/` + `parsers/{env,jsonp,toml,yaml}` — file/text format parsers (each has `New()`
  + `Read(ctx, io.Reader) (map[string]any, error)`).
- `util/` — reflection config hydration (`SetDefault`/`SetEnv`/`SetID`/`Dump`, `process()`
  walker, per-kind parsers in `process.go`) + `ParseContent`/`ParseFile`/`ParseFromText` +
  `DumpToEnv/JSON/YAML`.
- `option/` — functional options: `LoadKeyFunc` (key transforms: prefixer/suffixer/caser)
  and `WriteFunc` (target/environment/variable/httpVerb for GitHub).
- `config/` — generic YAML `LoadConfiguration[T]` (read-or-write-default).
- `internal/{logging,version}`.

## Conventions / style
- Errors wrapped with `github.com/thalesfsp/customerror` (`NewFailedToError`,
  `NewRequiredError`, `NewInvalidError`, `WithError`). Logging via `thalesfsp/sypl`.
- Validation via `thalesfsp/validation` (struct `validate:"..."` tags) run in every `New()`.
- Every provider: embeds `*provider.Provider`, implements `Load`/`Write`, `New`/`NewWithConfig`
  factory, validates on construction.
- Inline error handling (`if err := f(); err != nil`) is the dominant idiom (the v2 linter's
  `noinlineerr` flags it everywhere — pre-existing, not worth churning).
- Section banners `//////` between Vars/Methods/Factory.

## Build / test / lint / release  (Makefile)
- `make test` → `VENDOR_ENVIRONMENT=testing go test -timeout 30s -short -v -race -cover ...`
- `make ci` → `lint test coverage`. `make ci-integration` runs integration tests.
- Go pinned to **1.23.1** (`.tool-versions`); `go.mod` `go 1.23.0`, `toolchain go1.23.1`.
  Do NOT let `go get` bump the toolchain directive past 1.23.1 (it sets it to whatever ran
  it). Reset it back if it does. Newer x/crypto/x/net (≥0.40-ish) require Go 1.24+ — avoid.
- **Release flow:** merge PR → push a tag `vX.Y.Z` to `main` → `.github/workflows/release_build.yml`
  runs GoReleaser (+cosign) and the bot publishes the GitHub release. `resources/install.sh`
  downloads released binaries. Latest tag: **v1.3.33** (next would be v1.3.34). Nothing is
  installable until merged + tagged.

### ⚠️ golangci-lint version mismatch (known tooling debt)
- `.golangci.yml` is **v2 format** (`version: "2"`, `default: all` minus a disable-list).
- But `Makefile` pins `GOLANGCI_VERSION := v1.61.0` and `.github/workflows/go.yml` runs
  `golangci-lint-action@v6.1.0` with `version: v1.61.0`. GitHub CI ("Go build" check) is
  green regardless (v1.61 effectively ignores the v2-only linters / config).
- Running `make ci` locally with a **v2.x** golangci-lint installed surfaces ~44 issues in
  PRE-EXISTING files (wsl_v5, godoclint, funcorder, embeddedstructfieldcheck, noinlineerr,
  usetesting). These are NOT regressions and don't fail GitHub CI. To make local `make ci`
  match CI, either install golangci-lint v1.61.0, or (better, future work) bump the
  Makefile/workflow pin to v2.x AND clean up the ~44 findings as a dedicated PR.

## Important gotchas
- `util.ParseContent` switch is keyed by format string; the `yaml/yml` branch must use the
  `parsers/yaml` parser (it once wrongly used the env parser — env parser splits on `=` and
  rejects any real YAML). There is **no** test coverage for `parsers/*` historically.
- `ExportToStruct(v)` calls `util.Dump(v)` (hydrates struct from env; name is a bit
  misleading but intentional — Load sets env first, then Dump reads it back).
- GitHub provider fetches BOTH Actions and Codespaces public keys; `Write` must encrypt with
  the key matching `options.Target` (default target is `actions`). Using the wrong key →
  GitHub rejects (key_id/ciphertext mismatch).
- `cmd/utils.go` ES detection must use `strings.Contains(outputs,"elasticsearch")`, NOT
  `ContainsAny` (which matches any single letter).
- `util/process.go` `process()` must propagate the callback error (it previously swallowed
  parse errors from bad `default`/`env` tag values).

## In-flight work (PR #4, branch `claude/codebase-analysis-patterns-7Gpc5`)
Bug-fix sweep from a codebase analysis. Fixed + added a regression test per fix:
1. YAML routed through `env` parser → use `yaml.New()` (`util/util.go`).
2. `--key-suffixer` bound to `keyPrefixerOptions` → `keySuffixerOptions` (`cmd/load.go`).
3. GitHub encrypted with Codespaces key regardless of target → `publicKeyForTarget()`
   (`github/github.go`); also guarded nil `repository` in `constructRequestDetails`.
4. `ContainsAny` ES detection → `Contains` via `shouldUseElasticsearch()` (`cmd/utils.go`).
5. `process()` swallowed setter errors → propagate (`util/process.go`).
6. `config.LoadConfiguration` empty-file variable shadowing → return default (`config/config.go`).
7. Minor: `dotenv.New` `rawValuebool`→`rawValue`; `WithKeyCaser` simplified; dup comment.
Plus `build(deps)`: x/crypto v0.32→0.36 (CVE-2025-22869), x/net v0.34→0.38
(CVE-2025-22870/22872), go-jose/v4 v4.0.4→4.0.5 (CVE-2025-27144).

Tests added: `util/parse_test.go`, `util/process_error_test.go`, `cmd/flags_test.go`,
`github/keyfor_test.go`, `config/config_empty_test.go`.

## Environment notes (sandbox)
- Go module proxy is reachable; `vuln.go.dev` (govulncheck) and GitHub Dependabot-alerts API
  are NOT reachable from this env, so vulnerabilities can't be enumerated automatically here —
  rely on `go list -m -u` + known CVE knowledge, or ask the user to paste the Dependabot list.
