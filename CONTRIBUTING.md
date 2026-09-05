# Contributing to aiblame

Thanks for helping. aiblame is small on purpose: one binary, no runtime
dependencies, deterministic output. Please keep it that way.

## Ground rules

- **Zero-setup stays zero-setup.** Nothing may require a database, a daemon,
  an editor hook, network access or a token to produce the basic report.
- **Never invent attribution.** A commit is AI-touched only when it discloses
  it (trailer, agent identity, message marker). False positives are worse than
  misses. New detection rules need a real-world example in the tests.
- **Stable JSON.** `schema_version` is bumped for incompatible changes; adding
  fields is fine.
- **AI-assisted contributions are welcome and must be disclosed** with an
  `Assisted-by:` or `Co-authored-by:` trailer. Run `aiblame hook install` in
  your clone and it happens automatically. Yes, this repo dogfoods itself.

## Development

```sh
go test -race ./...     # everything, including git-backed integration tests
make lint               # gofmt + go vet
make build && ./aiblame # try it on this repo
```

Go 1.25+ and git 2.23+ are required. There is exactly one dependency
(`BurntSushi/toml`); please do not add more without a very good reason.

## Adding an agent identity

1. Add an entry to `KnownAgents` in `internal/attrib/identities.go` with the
   exact author e-mail / trailer name the tool emits. Link the source in
   `docs/DETECTION.md`.
2. Add a case to `TestClassifyRealWorld` in `internal/attrib/attrib_test.go`
   using the real string.
3. If the agent sets an environment variable in subprocesses, add it to
   `EnvRules` in `internal/hook/hook.go` so the hook can disclose it.

## Pull requests

- One topic per PR, tests included.
- Keep the README examples truthful: if you change output, regenerate them.
- CI runs on Linux, macOS and Windows; all three must pass.
