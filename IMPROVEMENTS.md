# configurer — Improvement Proposals

Proposed enhancements for `github.com/thalesfsp/configurer`, grounded in the
current architecture (`IProvider` + base `Provider`, `parser`/`parsers/*`,
`option/` functional options, reflection hydration in `util/`, Cobra CLI in `cmd/`).

Each new provider follows the established pattern: embed `*provider.Provider`,
implement `Load`/`Write`, expose `New`/`NewWithConfig` factories, and validate on
construction (`thalesfsp/validation` tags). Errors via `thalesfsp/customerror`,
logging via `thalesfsp/sypl`.

---

## 1. New Providers

| Provider | Value | Notes |
|---|---|---|
| GCP Secret Manager | Multi-cloud parity (big gap today) | Only AWS/Vault exist now |
| Azure Key Vault | Multi-cloud parity | Common enterprise ask |
| AWS AppConfig | Dynamic config + feature flags | AWS SDK already vendored |
| Doppler | Popular dev-secrets SaaS | Simple REST + token auth |
| 1Password Connect | Team secret sourcing | `op` Connect API |
| Kubernetes Secrets | In-cluster loading | client-go; read from namespace |
| Consul KV | Pairs with existing Vault/mole stack | hashicorp client already present |

## 2. Provider Capabilities / Interface

| Idea | Why |
|---|---|
| Add optional `Delete`/`List` via capability interfaces | Today only Load/Write; rotation & cleanup need delete |
| Bidirectional GitHub (`Load`) | Currently write-only (`ErrNotSupported`); read secret metadata |
| Secret versioning / `--version` flag | Vault & AWS support it; not exposed |
| Batch/parallel Load across multiple providers | Merge several sources in one run |

## 3. CLI & Developer Experience

| Idea | Why |
|---|---|
| `--dry-run` / `--diff` on write | Preview changes before mutating remote secrets |
| `configurer migrate <src> <dst>` | Move secrets between providers (dotenv→vault, etc.) |
| Shell completion + `configurer init` scaffold | Cobra completion is cheap; improves onboarding |
| Redact secrets in logs by default | Avoid leakage via sypl output |
| Consistent `--export` to JSON/YAML/dotenv | `util.DumpTo*` exists; expose uniformly |

## 4. Parsers / Formats

| Idea | Why |
|---|---|
| HCL parser | Fits Vault/Consul ecosystem |
| `.ini` parser | Common legacy config |
| `.properties` parser | JVM apps |

## 5. Quality / Hardening

| Idea | Why |
|---|---|
| Add `parsers/*` test coverage | Historically none (noted in project memory) |
| Re-enable disabled v2 linters as a dedicated PR | godoclint, wsl_v5, usetesting, noinlineerr, embeddedstructfieldcheck, funcorder — clean up then drop the disables |
| Add `govulncheck` to CI | Go 1.25 in place; CI can reach vuln.go.dev |
| Fuzz tests for parsers | Untrusted file input is a good fuzz target |
| Integration tests via testcontainers | Vault/localstack in `make ci-integration` |

## 6. Architecture

| Idea | Why |
|---|---|
| Provider registry + plugin discovery | Adding a provider now touches `cmd/` per-file; a registry decouples wiring |
| Caching/TTL layer in front of `Load` | Avoid hammering remote APIs on repeated runs |
| Secret templating (`{{ .DB_URL }}`) | sprig already vendored; render values referencing other keys |

---

## Suggested priority (impact ÷ effort)

1. **GCP Secret Manager + Azure Key Vault** — closes the multi-cloud gap.
2. **`parsers/*` test coverage** — known debt, low risk.
3. **`--dry-run`/`--diff` on write** — safety for destructive ops.
4. **Provider registry** — unlocks faster provider additions afterward.
5. **Re-enable linters PR** — pays down the curation debt from the v2 migration.
