# Ticket format

Each ticket is one UTF-8 Markdown file at `.wrk/<id>.md`. YAML frontmatter holds structured fields; the Markdown body explains the work. Tickets and `.wrk/config.yaml` are tracked by Git. No external service or separate database is required to understand the backlog.

## Editing contract

- People and agents may directly edit everything after the closing frontmatter delimiter: descriptions, acceptance criteria, decisions, and handoff notes.
- Create tickets and change all frontmatter through the CLI. This includes titles, statuses, parents, dependencies, priorities, labels, and custom-field values. IDs are immutable.
- The CLI validates changes before writing and preserves the existing Markdown body exactly when changing metadata. Failed validation leaves files unchanged.
- CLI writers are serialized with a persistent lock. Updates compare all validation inputs immediately before publication and reject detected changes. Direct body/config edits and Git operations must happen outside CLI mutations: an editor save between the final comparison and replacement can still be lost. See [the storage boundary](storage.md).
- These are repository conventions, not filesystem access controls. The `wrk validate` command will check repository integrity, including changes introduced by manual edits or Git merges; it cannot prove which tool made an edit.

This project's initial files were migrated before the CLI existed. The CLI now creates tickets (including parent, priority, and label overrides) and updates title/status. General relationship, priority/label, and custom-field updates remain deferred; do not change those metadata fields manually.

The body is every byte after the closing delimiter line ending; LF and CRLF are accepted, including an empty body and no terminal newline. Updates preserve exact body bytes and unrelated YAML values, including custom tags and aliases. Frontmatter formatting/comments are not byte guarantees. If preservation cannot be established, an update fails unchanged with `PRESERVATION_UNSUPPORTED`; reads remain available for otherwise valid files.

## Example

```markdown
---
id: wrk-a7f39c21
title: Add ticket creation
status: todo
parent: wrk-b2c84e16
depends_on:
  - wrk-d9e16a02
priority: high
labels:
  - cli
fields:
  customer: acme
---

## Outcome

A user can create a ticket from the terminal and immediately
find it in the project's .wrk directory.

## Context

Use the format described in docs/ticket-format.md.
Existing tickets must never be overwritten.

## Acceptance criteria

- [ ] Creating a ticket writes a valid Markdown file.
- [ ] The user can supply a title and description.
- [ ] The command prints the new ticket's ID and path.

## Notes

Record decisions, discoveries, and remaining work for the next agent.
```

The references in this example are illustrative. Actual relationships must point to existing tickets in the same `.wrk/` directory.

## Fields

| Field | Required | Meaning |
| --- | --- | --- |
| `id` | Yes | Stable identity, matching the filename without `.md`. |
| `title` | Yes | Nonempty, single-line summary. |
| `status` | Yes | `todo`, `in-progress`, `done`, or `canceled`. |
| `parent` | No | ID of the single ticket this work belongs to. Omit for a root ticket. |
| `depends_on` | No | List of prerequisite ticket IDs; absent means no dependencies. |
| `priority` | No | `low`, `normal`, `high`, or `urgent`; absent means `normal`. |
| `labels` | No | List of nonempty strings; absent means no labels. |
| `fields` | No | Map of project-specific values; absent means no custom values. |

Use only the built-in fields at the top level; custom data belongs under `fields`. Duplicate YAML keys are invalid. Empty relationship lists may be omitted. Clearing a parent removes the field.

## Identity

IDs combine the configured prefix with eight random lowercase hexadecimal characters, such as `wrk-a7f39c21`. Generate a new ID if it already exists locally. Random IDs avoid coordinating a shared counter across branches; validation must still detect duplicate IDs after a merge.

The filename is exactly `<id>.md`, without a title slug. Renaming or reparenting a ticket does not rename its file or change its ID. Changing the configured prefix affects only newly created tickets; references to existing IDs remain valid.

Only files matching the ticket ID filename pattern are tickets. `.wrk/config.yaml` and `.wrk/AGENTS.md` are supporting files.

## Relationships and workflow

A ticket can have one parent and any number of children. Store only `parent` on the child; derive the child list. There are no separate epic, story, and subtask formats, and no fixed hierarchy depth.

Dependencies express prerequisites independently of parenting. Parents and dependencies must exist in the same project. Reject self-references, duplicate dependencies, cycles in the parent graph, and cycles in the dependency graph. Check these graphs separately: a parent may legitimately depend on its children.

An unfinished dependency is a blocker. Only `done` satisfies a dependency; `canceled` does not. Show dependency blockers alongside status without introducing a `blocked` status. Explain blockers that are not tickets in the body. In the initial workflow, blockers are reported but do not prohibit an explicit status change.

Status changes are explicit. Completing children does not close a parent, completing a prerequisite does not start dependent tickets, and cancellation does not cascade. Reopening a completed ticket is allowed. Use `canceled` to retire work while keeping its history and references.

## Body conventions

Describe the intended outcome and what counts as done. Add context, code or documentation links, and checkable acceptance criteria when useful. The headings in the example are a suggested template, not a required schema; a small task can have a small body.

Keep notes useful for handoff: current decisions, discoveries, and remaining work. Git provides edit history. There is no separate comments database, assignment system, or required timestamp metadata in the initial format.
