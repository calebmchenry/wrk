# Agent docs

## Tracking work

Use this project's own CLI to track work in [`.wrk/`](../.wrk/). Follow the [daily workflow](#daily-workflow) for each task, read relevant tickets before starting work, and keep their context current as work progresses.

- [Ticket format and editing rules](ticket-format.md)
- [Project configuration](configuration.md)
- [Explicit project/config selection and path behavior](cli.md#project-selection)
- [CLI commands and output contract](cli.md)
- [Local server, read APIs, lifecycle output, and resource limits](cli.md#local-server)
- [Browser filters, navigation, Markdown policy, and browser verification](browser.md)
- [Live polling, recovery, and the dirty-draft interface](browser.md#live-updates-and-recovery)
- [Body updates](cli.md#body-updates-and-revisions) and [stale-edit revisions](storage.md#item-revisions-and-stale-edits)
- [Parent and dependency edits](cli.md#relationship-updates) and [typed custom fields](cli.md#custom-field-input-and-updates)
- [Storage, concurrency, and recovery](storage.md)
- [Release packaging, publication, and upgrade verification](releases.md)
- [v0.1.0 platform and upgrade verification evidence](release-verification.md)
- [Scoped agent burn loop and durable handoffs](burns.md)
- [Minimal command-driven Codex runner](burns.md#minimal-codex-runner)
- [Development checks and integration-test cache behavior](../README.md#developing-wrk-with-wrk)

Agents may edit ticket bodies directly. Creation and changes to frontmatter, relationships, or status go through the CLI. Project configuration may be edited directly.

The CLI supports initialization, creation with parent/dependency/priority/label/custom-field
values, list/show/validate, exact body replacement, and updates to every mutable
metadata field (including blocked status). Label replacement/clear/add/remove
also supports recursive updates.
Lists can filter labels and descendants. IDs remain immutable; change metadata through the CLI. Direct body/config edits and Git operations must occur outside CLI mutations; see the accepted [external-editor boundary](storage.md).

## Daily workflow

Run these commands from the repository root using Go 1.25 or newer (see
[build and install](../README.md#build-and-install)). Prefer `go run ./cmd/wrk`
so dogfooding exercises the current checkout. A local `./bin/wrk` is also fine if
you rebuild it after CLI changes. This repository already has `.wrk/`; initialization
is only needed for a new project.

1. Inspect active work and check the project before making changes:

   ```sh
   go run ./cmd/wrk validate
   go run ./cmd/wrk list
   go run ./cmd/wrk list --ready
   ```

   `list` includes work already in progress and blocked tickets. `--ready` only
   shows `todo` tickets with completed dependencies. Manual blocked status never
   clears automatically. Use `--all` to find closed work.

2. Read the relevant ticket with `go run ./cmd/wrk show <id>`. Reuse it if it covers
   the task. Otherwise create a ticket and use the ID printed by the CLI:

   ```sh
   go run ./cmd/wrk new "Describe the outcome"
   ```

   Add `--parent <parent-id>` when the task belongs to an existing ticket. Supply
   a body with `--body-file path` or `--body-file -` for stdin, or edit the new
   ticket's Markdown body directly to describe the outcome and acceptance criteria.

3. Start work with `go run ./cmd/wrk update <id> --status in-progress`.
   Replace `<id>` with the actual ticket ID in all examples. Keep decisions,
   discoveries, verification results, and remaining work in the ticket body.
   Record CLI gaps there or create a follow-up with `new`; unsupported metadata
   operations remain deferred until implemented.

4. When the acceptance criteria are met and relevant checks pass, update the body
   with the results, then finish through the CLI:

   ```sh
   go run ./cmd/wrk update <id> --status done
   go run ./cmd/wrk validate
   go run ./cmd/wrk show <id>
   ```

   For unfinished work, leave an accurate status and handoff notes, then validate.
   Completing a child does not complete its parent; only close a parent when its
   own criteria are met. Treat CLI errors as failures: inspect diagnostics and
   follow [recovery guidance](cli.md#discovery-and-validation) before continuing.

Use `--json` when consuming results programmatically. Workflows and sprint documents
provide detail; ticket status in `.wrk/` remains the source of truth for task progress.

## Project tickets

Use the CLI for current status and the complete backlog; these links provide context
for the initial milestones and deferred scope.

- [wrk-9d42c6a1: Define the ticket format](../.wrk/wrk-9d42c6a1.md) — done
- [wrk-3f8a21b7: Build the minimal CLI](../.wrk/wrk-3f8a21b7.md) — done
- [wrk-682f60c7: Ship the first runnable CLI for dogfooding](../.wrk/wrk-682f60c7.md) — done; first runnable CLI verified on macOS and Linux

- [wrk-1a09af55: Support parent and dependency updates](../.wrk/wrk-1a09af55.md) — done
- [wrk-e146d171: Support priority and label updates](../.wrk/wrk-e146d171.md)
- [wrk-c12ff4c6: Support custom-field creation and updates](../.wrk/wrk-c12ff4c6.md) — done
- [wrk-d965ba66: Scoped AI task burns](../.wrk/wrk-d965ba66.md) — labels, descendant scopes, blocked status, and the burn workflow

Release and upgrade work:

- [wrk-551ff6c2: Publish versioned binaries](../.wrk/wrk-551ff6c2.md) — version metadata, platform packaging, and release publication.
- [wrk-8d5b1b84: Upgrade to the latest stable release](../.wrk/wrk-8d5b1b84.md) — check, install, and verify/document the upgrade path. List both deliverables and their children with `go run ./cmd/wrk list --all --label upgrade`; implementation prerequisites are recorded as dependency edges; ticket bodies retain the sequencing rationale.

Local web workspace:

- [wrk-a1561423: Serve a local web workspace with live agent updates](../.wrk/wrk-a1561423.md) — local-only server, explicit project/config selection, bundled browser UI, live changes from agents, safe editing, and related links. Eight child tickets carry implementation and verification; the milestone contains their dependency order.
- [wrk-4036f7d4: Web workspace discovery](../.wrk/wrk-4036f7d4.md) — confirmed user intent and scope decisions. `serve` provides searchable list/detail browsing, validated read APIs, and automatic refresh; browser editing remains in the backlog.
- List the complete batch with `go run ./cmd/wrk list --all --label web`; select ready implementation work with `go run ./cmd/wrk list --ready --under wrk-a1561423`.

## Sprint plans

- [Sprint 001: First Runnable wrk CLI](sprints/SPRINT-001.md) — implementation and verification for `wrk-682f60c7`.
- [Sprint 001 execution evidence](sprints/SPRINT-001-EXECUTION.md) — phase gates and platform/build/install results.
- [Sprint ledger](sprints/ledger.tsv) — execution status; `planned` does not imply user approval.
- [Sprint 001 planning and review notes](sprints/drafts/SPRINT-001-MERGE-NOTES.md) — confirmed decisions, three independent drafts, and cross-review synthesis.
