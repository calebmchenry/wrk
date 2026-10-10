# Run output verification

Acceptance evidence for [wrk-7b831fa1](../.wrk/wrk-7b831fa1.md), performed
2026-10-10 using fake commands and disposable projects. No real backlog burn or
agent invocation was needed. The contract is in [CLI output and logs](cli.md#run-output-and-logs)
and [JSON events](cli.md#run-json-events).

## Automated coverage

| Requirement | Evidence |
| --- | --- |
| Exact selection and counts | Engine and compiled-CLI tests cover intersecting ready/active/all, repeated AND labels, descendants, root exclusion, parent title/rename, current matches, processed matches, candidates, and mutually exclusive scope status counts without expanded eligibility. |
| Success versus completion | Exit failure and exit-0 status mismatch remain distinct; unchanged todo tickets can succeed once. Events include required/observed status and completion result. Deletion/invalid data still fail. |
| Retry fidelity | Execute a generated POSIX-shell rerun from another project, using config selection and a project path with spaces/apostrophes. Empty arguments, spaces, quotes, embedded newlines, metacharacters, and `{id}` round-trip correctly; status requirement and explicit ticket survive. |
| Logs and stdout isolation | A child emits 8 MiB of binary data on each stream. Separate logs match every byte; JSON remains small and parseable, and default stderr is empty. Tests compare `run.jsonl` byte-for-byte with JSON stdout and check file permissions and invocation-relative overrides. |
| Prompt lifecycle events | A gated child cannot produce output or finish until the parent test receives JSON heartbeats. Verify startup/selection/start/heartbeat/finish order, sequence/timestamps, initial absent output recency, then observed stream byte counts/time. `--verbose` uses stderr. |
| Output and log failures | Fail run-log writes, child-log creation/writes/close, final run-log close, heartbeat output, and live output while a child sleeps. Verify nonzero exit, bounded shutdown, and no subsequent ticket. Short writes fail too. |
| Real broken pipe | Close actual JSON stdout while a child and descendant are active. Verify SIGPIPE becomes an output failure, both processes stop, the terminal failure remains in the writable run log, and no second ticket runs. |
| Interruption | Existing compiled-CLI SIGINT/SIGTERM tests cover active groups and idle waits in ordinary/stream modes, retain partial changes, and now parse lifecycle NDJSON. Idle explanations remain silent on unchanged polls. |
| Terminal and redirected text | Renderer tests check in-place controls only in terminal mode, permanent milestones, readable durations, no invented output time, and distinct idle explanations. Manual PTY/captured review below supplements those checks. |

One full-suite run exposed a fixture synchronization race: file existence could be
observed between creation and writing `partial-change`, then the test interrupted
the helper too early. The interruption fixture now waits for complete marker
contents before signaling. This changes test synchronization, not product mutation
or interruption semantics.

## Manual transcript review

Built `./bin/wrk` from the checkout. Created a disposable parent, one ready child,
and one manually blocked child, with intersecting `review`/`second` labels. Launched
the binary on a real macOS PTY with `TERM=xterm-256color`, `--ready`, both labels,
`--under`, and a 500ms heartbeat. The fake shell command waited, printed to both
streams, waited again, then exited 0 without changing the ticket.

Inspected startup project/cwd/settings, exact filter/parent exclusion text, ID order,
title/priority/sequence, log paths, the replacing heartbeat line, initial “no child
output observed,” later measured output recency, permanent finish, and final
all-processed explanation. The summary reported one successful action with observed
todo, plus one ready and one manually blocked scoped ticket.

Captured plain output separately and verified it contains neither ESC nor CR.
Captured JSON and parsed every line; event order was started, selection,
action_started, heartbeats, action_finished, selection, finished. Default JSON
stderr was empty. Inspected exit-7 and exit-0/expected-done mismatch diagnostics,
including show/retry commands and child logs. A stream run stayed idle across
several unchanged polls, printed one waiting explanation, and stopped with exit
130 and an elapsed-time/count/log summary on SIGINT.

Local diagnostic artifacts are intentionally ignored:
`.wrk-runs/output-verification-20261010/` contains `terminal.txt`, plain/JSON captures,
failure captures, idle captures, and the disposable project's full run logs. These
are evidence artifacts, not a portable test fixture or a resume database.

## Platforms and checks

- macOS 15.7.7 (24G720), arm64, Go 1.25.4: full uncached Go tests and race tests,
  vet/build, formatting/diff checks, all 6 burn-script tests, and all 15 browser
  model tests passed. CLI/integration tests were rerun after the fixture fix and
  final summary wording change.
- Debian 12.12, Linux 5.10.124-linuxkit, arm64, Go 1.25.4: full uncached Go tests
  and race tests passed, as did vet/build, formatting, project validation, all
  6 burn-script tests, and all 15 browser model tests.
  Container checks used `golang:1.25.4-bookworm`, Docker `--init`, UID 1000,
  a read-only source mount copied into `/work/wrk`, and temporary projects outside
  repository ancestors. Final CLI/integration normal and race reruns also passed.

No browser assets, release packaging, or real Codex execution changed in this
ticket. Burn migration and its integrated acceptance are recorded in
[wrk-ef04f589](../.wrk/wrk-ef04f589.md). The subsequent migration and
[parent acceptance review](burn-verification.md#parent-acceptance-review) are complete.
