# Run output verification

## Compact output review (2026-10-15)

Acceptance evidence for [wrk-e0296fd9](../.wrk/wrk-e0296fd9.md). The current
contract is in [CLI output and logs](cli.md#run-output-and-logs). Selection,
completion, and retry policy are unchanged.

### Reproduction and decisions

Built the previous renderer from commit `6e7182b` in a disposable source copy and
ran fake children on a 36-column macOS PTY. Confirmed three copies of a multiline
argument (template, expanded command, retry). With `OPOST` disabled, bare LF left
the next record at the current column, producing staircase indentation. With
`--verbose`, a child ending in `partial-child-output` produced
`partial-child-outputFinished ...` without a separating newline. Without verbose,
the heartbeat erased only the current physical row, even though its text exceeded
36 columns. These are reproducible rendering defects; they do **not** establish
which terminal settings or wrapper produced the user's original transcript.

The new renderer uses append-only heartbeats, with no row erasure or cursor-up
logic. It therefore does not need to guess how a terminal reflows after a resize.
Capable terminal records use explicit CRLF; after verbose child LF it returns to
column zero, and after an unterminated child line it inserts a newline. Redirected
and dumb-terminal records remain plain LF. Full prompts stay in `run.jsonl`, and
the exact retry is in private `retry.sh` plus the existing JSON `retry_command`.
The runner never executes that artifact automatically.

Representative abbreviated **before** (prompt contents omitted here):

```text
Action template: 'codex' 'exec' '<large multiline prompt>'
Success: exit 0 and valid existing ticket; persisted status done
Starting wrk-8f9b4f7e: #1 ...
Command: 'codex' 'exec' '<large multiline expanded prompt>'
Running #1 wrk-8f9b4f7e; elapsed 15m54s; last child output 4s ago
...
Inspect partial changes, then retry from the beginning:
'wrk' '--project' '/project' 'run' '--ticket' 'wrk-8f9b4f7e' ... '<large multiline prompt>'
```

Representative **after**, using the reported duration/statuses and shortened paths
for illustration (the actual disposable tests ran about one second):

```text
RUN  codex (2 arguments; full argv in run log)
Project: /project
Select: ready: todo with all dependencies done AND all labels: "queued"
Mode: ordinary; exit when drained | limit unlimited | heartbeat 1m0s
Success: child exit 0 + valid existing ticket; status done
Run log (commands and details):
  /project/.wrk-runs/example/run.jsonl
QUEUE  1 matches | 0 processed | 1 candidates (ID order)
START  wrk-8f9b4f7e | #1 | Prototype orthographic follow camera
RUNNING  wrk-8f9b4f7e | elapsed 15m54s | last output 4s ago
FAILED  wrk-8f9b4f7e | child exit 0 | elapsed 15m54s | completion check failed
  Status: in-progress (expected done)
FAILED  0 successful actions | elapsed 15m54s
  RUN_COMPLETION: expected status done; observed in-progress
  Ticket: wrk-8f9b4f7e | child exit 0 | elapsed 15m54s
Logs (files in this directory):
  /project/.wrk-runs/example
  Run:    run.jsonl
  Stdout: 0001-wrk-8f9b4f7e.stdout.log
  Stderr: 0001-wrk-8f9b4f7e.stderr.log
Recovery:
  Show ticket: 'wrk' '--project' '/project' 'show' 'wrk-8f9b4f7e'
  Inspect partial changes before retrying from the beginning.
  Exact retry (view/copy): cat '/project/.wrk-runs/example/retry.sh'
```

### Verification

- Focused Go regressions check compact multiline output, unchanged template and
  expanded argv, private retry-file contents/permissions, one primary failure
  diagnostic, partial verbose line separation, escaped untrusted text, and semantic
  style resets. Existing output/log/broken-pipe failure tests still pass; retry-file
  creation failure now has explicit coverage.
- The compiled-CLI retry test executes the generated script from another project,
  preserving config-selected project paths with spaces/apostrophes, empty args,
  quotes, newlines, shell metacharacters, `{id}`, and the required persisted status.
  Existing large binary log checks and JSON/verbose isolation checks pass unchanged.
- `python3 scripts/verify-run-output.py ./bin/wrk` passes on macOS and Linux. Its
  twelve cases cover a long multiline argument and long project/log paths; 36/100
  columns and live 100→36→120→52 resize; repeated silent/noisy heartbeats;
  successful action, exit 7, exit-0/expected-done/observed-in-progress failure;
  verbose partial lines and raw-terminal child newlines; `NO_COLOR`, `TERM=dumb`,
  both redirected streams, each mixed terminal/redirected direction, JSON plus
  verbose, and unchanged idle polls followed by SIGINT. It asserts exact argv/raw
  log bytes, parseable JSON equal to the run log, per-destination styling, and no
  cursor erasure or unexpected terminal controls.
- Manually inspected PTY and redirected captures, then rendered captured SGR spans
  with representative light/dark ANSI palettes, bold/default foreground, a
  36-column layout, and no-color output. State labels and changing durations are
  easy to distinguish; supporting text remains legible without low-contrast dim
  styling. Actual colors remain controlled by the user's terminal theme.
- Full uncached Go tests and race tests, vet/build, formatting, four burn-wrapper
  tests, and seventeen browser model tests pass on macOS arm64 (15.8.1, Go 1.25.4)
  and Linux arm64 (`golang:1.25.4-bookworm`, unprivileged UID 1000, `--init`, local
  source copy). CLI/integration checks and PTY cases were rerun after the final
  child-newline boundary adjustment. The isolated Linux snapshot validates with 45 tickets; final checkout validation
  passes with 46 tickets after concurrent backlog additions.

Ignored local artifacts: `.wrk-runs/compact-output-final-20261015/` contains
baseline reproductions; `.wrk-runs/compact-output-accepted-20261015/` contains the
final per-stream captures and `theme-review.html`/`theme-review.png`. The harness
prints the disposable project and capture locations so they can be inspected or
deleted later. No real agent was invoked.

## Original output review (2026-10-10)

Historical acceptance evidence for [wrk-7b831fa1](../.wrk/wrk-7b831fa1.md), performed
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
