# Working on aiblame

aiblame is a Go CLI that reports how much of a git repository was written with
AI, using the disclosure trailers and agent identities already in the history.
One binary, one dependency (`BurntSushi/toml`), no network unless the user
passes a URL.

## Build, test, run

```sh
go build ./cmd/aiblame          # binary in ./aiblame
go test -race ./...             # unit + git-backed integration tests (needs git)
go vet ./... && gofmt -l .      # must be clean
./aiblame stats . --no-color    # dogfood
```

## Layout

- `cmd/aiblame` — `main()` only.
- `internal/cli` — subcommands (`stats`, `blame`, `log`, `badge`, `check`, `hook`, `agents`, `init`, `version`). Flags use the stdlib `flag` package; positional args may appear anywhere (`parseInterspersed`).
- `internal/attrib` — identity tables (`identities.go`), trailer parsing (`trailers.go`), commit classification (`classify.go`). **Every new identity needs a real-world example in `attrib_test.go`.**
- `internal/gitx` — thin wrapper over the `git` CLI (log with numstat, blame porcelain, ls-tree). No go-git.
- `internal/stats` — aggregation into `Report` (stable JSON, `schema_version`), gitignore-style glob matcher, default excludes.
- `internal/render` — table / Markdown / JSON / SVG badge / shields endpoint.
- `internal/hook` — `prepare-commit-msg` hook script + env detection.
- `internal/config` — `.aiblame.toml`.
- `internal/testrepo` — builds throwaway repos for tests (`testrepo.Standard`).

## Rules

- Never invent attribution. A commit is AI-touched only with disclosure evidence. Prefer a miss over a false positive.
- Keep zero-setup: no databases, daemons, editor hooks or tokens for the basic report.
- JSON field names are a contract. Add fields freely; renaming or removing bumps `stats.SchemaVersion`.
- Tests must pass on Linux, macOS and Windows. Use `filepath` for local paths and forward slashes for repo-relative paths.
- Don't add dependencies. The stdlib is enough.
- Disclose your own involvement: commits from an agent session must carry an `Assisted-by:` or `Co-authored-by:` trailer. `./aiblame hook install` does this automatically; CI runs `aiblame check --require-trailer co-authored-by` on the repo itself.
