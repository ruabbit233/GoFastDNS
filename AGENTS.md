# AGENTS.md

## Project Overview

GoFastDNS is a Go CLI tool for DNS latency benchmarking.

The CLI supports two modes:

- `dns-ping`: ping DNS server endpoints directly.
- `resolve-ping`: resolve the same domains through different DNS servers, then ping the resolved IPs to estimate CDN locality.

Configuration can come from command-line flags, `config.yaml`, or defaults. Command-line flags override config file values.

## Repository Layout

- `main.go`: thin process entrypoint; delegates to `internal/cli`.
- `internal/cli`: flag parsing, config loading, config override behavior, exit codes.
- `internal/config`: YAML config model, defaults, validation.
- `internal/benchmark`: mode dispatch, benchmark runners, result sorting, HTML and Excel output.
- `internal/dns`: DNS resolver implementations for UDP, TCP, and DoT.
- `internal/ping`: ping options, IP selection, and ping execution.
- `config.yaml`: example/default local config.
- `README.md`: user-facing usage documentation.

## Common Commands

Use a workspace-local Go build cache in restricted environments:

```bash
GOCACHE=$(pwd)/.tmp-gocache go test ./...
GOCACHE=$(pwd)/.tmp-gocache go vet ./...
```

Run the CLI:

```bash
go run . -c config.yaml
go run . -mode dns-ping -dns udp://8.8.8.8,udp://1.1.1.1
go run . -mode resolve-ping -dns udp://8.8.8.8,tls://dns.google -domains baidu.com,bilibili.com
```

Format Go code before finishing changes:

```bash
gofmt -w <changed-go-files>
```

## Testing Notes

Default tests must not require live DNS, network access, or ICMP permissions.

The ping integration tests are behind the `integration` build tag:

```bash
GOCACHE=$(pwd)/.tmp-gocache go test -tags=integration ./internal/ping
```

Only run integration tests when the task explicitly needs live network behavior. They may require elevated privileges depending on the platform and ping mode.

## CLI and Config Rules

- Keep command-line flags backward-compatible when possible.
- Command-line flags must override values loaded from `config.yaml`.
- Add new config fields to:
  - `internal/config/config.go`
  - `config.yaml`
  - `README.md`
  - relevant CLI flags in `internal/cli/cli.go`
  - unit tests
- Config validation should return errors rather than calling `log.Fatal` or exiting directly.
- `main.go` should stay minimal and should not contain benchmark logic.

## Benchmark Behavior

`resolve-ping` currently supports `ping.ip_selection`:

- `all`: ping all resolved A records and average successful RTTs.
- `first`: ping only the first resolved A record to approximate clients that try the first returned address first.

When adding new benchmark behavior:

- Preserve complete DNS answer data in results when possible.
- Keep the actual ping target list visible in output.
- Do not treat failed pings as valid zero-latency results.
- Keep mode-specific result types explicit when shared structs would hide meaning.

## Output Rules

Generated benchmark output files are `.html` or `.xlsx` files and should not be committed.

The repository also uses `.tmp-gocache/` as an optional local Go build cache in restricted environments. Do not commit it.

If adding a new output format, update output dispatch, validation, README examples, `config.yaml`, and tests.

## Code Style

- Prefer the existing package boundaries over introducing new abstractions.
- Use `gofmt` for formatting.
- Return errors from library/config/runner layers; only CLI code should convert errors into exit codes.
- Keep network-dependent behavior isolated behind integration tests or injectable helpers.
- Avoid broad refactors unless they directly support the requested change.
