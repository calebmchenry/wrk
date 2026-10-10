# Developing wrk

Use Go 1.25 or newer on macOS or Linux. The verified development toolchain is
Go 1.25.4 on arm64 for both platforms. See [installation](install.md) for setup.

Use wrk for this repository's own work. Follow the [daily workflow](index.md#daily-workflow)
to find or create a ticket, mark it in progress, record results in its body, and
complete it through the CLI. Run the current checkout from the repository root:

```sh
go run ./cmd/wrk validate
go run ./cmd/wrk list
```

No initialization is needed here: the backlog already lives in [`.wrk/`](../.wrk/).
Prefer `go run ./cmd/wrk` over an installed binary that may be stale, or rebuild
`./bin/wrk` after CLI changes. Read the [ticket editing contract](ticket-format.md#editing-contract)
before editing tickets.

For code changes, run the development checks:

```sh
gofmt -l cmd internal test
go test -count=1 ./...
go test -race -count=1 ./...
go vet ./...
go build -o ./bin/wrk ./cmd/wrk
python3 -B -m unittest discover -s scripts -p 'test_ticket_burn.py' -v
node --test internal/web/model.test.mjs
```

Formatting must report no files. Use `-count=1` so the integration package reruns
its compiled CLI: it builds the binary in `TestMain`, and Go's test cache does not
track that runtime build as a source dependency. Run the checks on both macOS and
Linux local filesystems; cross-compilation alone does not exercise storage.
Tests use disposable projects and retain the
immutable original data in `testdata/compat/.wrk/`. [Sprint 001 evidence](sprints/SPRINT-001-EXECUTION.md)
records macOS/Linux verification and clean-source build/install runs.

For browser changes, also run `npm ci`, `npx playwright install chromium`, and
`npm run test:browser`. These are development dependencies only; the executable
embeds the complete UI. See [browser checks](browser.md#verification).
Run `python3 scripts/verify-workspace.py ./bin/wrk` after building to verify the
embedded module graph and API/CLI persistence outside the source tree with an
empty executable PATH. Run browser timing checks after CPU-heavy builds finish.
After packaging, `python3 scripts/verify-release.py dist` also starts the extracted
native executable away from the checkout, checks every embedded UI module, and
exercises API/CLI writes with no tools on the executable's PATH.
