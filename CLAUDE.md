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
- Go pinned to **1.25.0** (`.tool-versions`); `go.mod` `go 1.25.0`, `toolchain go1.25.0`.
  Both workflows use `go-version: "1.25"`. All deps at latest (`go get -u ./...`).
- **Release flow:** merge PR → push a tag `vX.Y.Z` to `main` → `.github/workflows/release_build.yml`
  runs GoReleaser (+cosign) and the bot publishes the GitHub release. `resources/install.sh`
  downloads released binaries. Latest tag: **v1.3.33** (next would be v1.3.34). Nothing is
  installable until merged + tagged.

### golangci-lint (v2, aligned)
- `.golangci.yml` is **v2 format**. `Makefile` pins `v2.5.0` (install path `.../v2/cmd/...`);
  `go.yml` runs `golangci-lint-action@v6.5.0` with `version: v2.5.0`. Local `make ci` == CI.
- v2 `default: all` enables opinionated linters the repo never adopted; DISABLED in
  `.golangci.yml`: `godoclint`, `wsl_v5`, `usetesting`, `noinlineerr`,
  `embeddedstructfieldcheck`, `funcorder`. Re-enable + clean up only as a dedicated PR.

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
Plus `build(deps)`: ALL deps bumped to latest via `go get -u ./...` on **Go 1.25** (x/crypto
v0.52, x/net v0.55, x/text v0.37, go-jose/v4 v4.1.4, aws-sdk-go-v2 latest, vault/api v1.23,
…). Covers CVE-2025-22869/22870/22872/27144 and everything newer. golangci-lint aligned to
v2.5.0 with opinionated linters disabled (see above). `make ci` + `go test -race` green.

Tests added: `util/parse_test.go`, `util/process_error_test.go`, `cmd/flags_test.go`,
`github/keyfor_test.go`, `config/config_empty_test.go`.

## Environment notes (sandbox)
- Go module proxy is reachable; `vuln.go.dev` (govulncheck) and GitHub Dependabot-alerts API
  are NOT reachable from this env, so vulnerabilities can't be enumerated automatically here —
  rely on `go list -m -u` + known CVE knowledge, or ask the user to paste the Dependabot list.
