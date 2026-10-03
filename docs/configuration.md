# Project configuration

Each project has one Git-tracked `.wrk/config.yaml`. People and agents may edit this file directly between CLI mutations and Git operations. Ticket metadata still follows the [CLI editing contract](ticket-format.md#editing-contract).

## Initial configuration

```yaml
version: 1
prefix: wrk
defaults:
  priority: normal
  labels: []
fields: {}
```

| Setting | Meaning |
| --- | --- |
| `version` | Required project format version. Initially `1`; covers both configuration and ticket conventions. Unsupported versions must produce an error without writing files. |
| `prefix` | Required prefix for newly generated IDs. Use a lowercase letter followed by up to 15 lowercase letters or digits. Existing IDs may retain an older prefix. |
| `defaults.priority` | Optional priority applied at creation; defaults to `normal`. |
| `defaults.labels` | Optional labels applied at creation; defaults to an empty list. |
| `fields` | Optional definitions for custom ticket fields; defaults to an empty map. |

New tickets default to `todo`. Explicit creation arguments override configured defaults, including an explicitly empty label list. Defaults apply only to new tickets: changing configuration never rewrites or changes the meaning of existing ticket values. IDs, parents, and dependencies do not have configurable defaults.

Updates use existing ticket values, never creation defaults. An omitted priority
always means `normal` and omitted labels always mean an empty list. Thus
`update --priority normal` or `update --no-labels` does not insert already-effective
omitted values. `update --label` replaces an existing list rather than combining
it with configured defaults; use `--add-label` / `--remove-label` for incremental
edits. See [priority and replacement updates](cli.md#priority-and-replacement-updates).

## Custom fields

Custom definitions provide descriptions and optional, lightweight value checks:

```yaml
fields:
  customer:
    type: string
    description: Customer associated with this work.
  estimate:
    type: number
    description: Rough estimate in the project's chosen units.
  needs_review:
    type: boolean
  area:
    type: enum
    options: [cli, storage, docs]
```

Definitions require a `type`: `string`, `number`, `boolean`, or `enum`. Enum definitions require a nonempty list of distinct string options. Descriptions are optional. All custom ticket fields remain optional; required fields, custom defaults, and complex schemas are deferred.

Values live under `fields` in ticket frontmatter. A configured field must match its declared type, without implicit coercion. Unconfigured custom fields are allowed and their values must be preserved when other metadata changes. Configuring or changing a definition can reveal invalid existing values; validation reports those tickets without rewriting them.

Supply values with `new/update --field 'name=YAML'` and remove them with
`update --remove-field name`. YAML types are explicit: `--field 'estimate=3'`
supplies a number, while `--field 'customer="3"'` supplies a string. These flags
never modify definitions or provide new defaults. See [custom-field input and
updates](cli.md#custom-field-input-and-updates) for quoting, aliases, and no-ops.

## CLI behavior

- Discover the project by walking upward from the working directory to the nearest entry named `.wrk`. It is the authoritative project boundary even when it is a file or forbidden symlink; do not fall through to an outer project if its configuration is missing or invalid.
- Read project configuration for each command. Reject malformed configuration, duplicate keys, unknown settings, invalid defaults or field definitions, and unsupported versions with an actionable error before writing anything.
- Initialization requires an existing target directory, exclusively creates `.wrk/`, and publishes the exact initial configuration above. It refuses any existing `.wrk` entry, including incomplete or symlinked entries; it never adopts existing data. Add `**/.wrk/.lock` and `**/.wrk/.wrk-stage-*` to that project's Git ignore rules.
- Validate existing ticket IDs independently of the current prefix, and check uniqueness across the whole project.
- Provide `wrk validate` to check configuration, tickets, and relationships without mutating files.

Configuration requires exactly one YAML document. Duplicate keys are rejected recursively; explicit null does not substitute for a typed optional setting. String and enum values require resolved string nodes, numbers require integer/float nodes without precision loss, and booleans require true/false nodes. Quoted numeric/boolean lookalikes do not match those types. Optional omissions use the defaults documented above.

Keep the initial workflow and field vocabulary fixed. Custom statuses, transition rules, user assignments, integrations, and personal preferences can be added when there is a concrete need. No credentials belong in this Git-tracked config.
