# Conditional commands

Jobs can define **multiple candidate templates per command tab**. At launch time the
server evaluates branch conditions against the user's query-parameter values,
runs the first matching branch's template, and **skips** commands with no match (no
pane is spawned).

## JSON shape

A command is either a **simple template** (legacy) or a **conditional branch list**.
Both forms are backward compatible — existing jobs with only `template` continue to
work unchanged.

```json
{
  "label": "Deploy",
  "branches": [
    { "when": { "env": "^prod$" },    "template": "kubectl --context=prod apply -f prod.yaml" },
    { "when": { "env": "^staging$" }, "template": "kubectl --context=staging apply -f staging.yaml" },
    { "default": true,                "template": "docker compose up" }
  ]
}
```

```json
{ "label": "Always run", "template": "echo done" }
```

### `CommandBranch` fields

| Field       | Description |
|-------------|-------------|
| `when`      | Map of variable name → Go `regexp` pattern. **All** entries must fully match the variable value (AND). Patterns are auto-anchored as `^(?:pattern)$` at evaluation time, so `prod` matches only `prod`, not `preprod`. Omitted or empty on a non-default branch → never matches. |
| `default`   | When `true`, this is the **else** branch. Evaluated only after every non-default branch fails. At most one per command. Must not include `when`. |
| `template`  | Shell command template with `{{var}}` placeholders (required on every branch). |

### `Command` fields

| Field      | Description |
|------------|-------------|
| `label`    | Tab title in the terminal UI. |
| `template` | Used when `branches` is empty (simple mode). |
| `branches` | Ordered list of conditional branches. When non-empty, `template` is ignored. |

## Evaluation (launch time)

1. Validate every query-param value against its variable regex (`validate.Vars`).
2. For each command in order, call `validate.ResolveCommand`:
   - **No branches** → use `template` (same as today).
   - Walk non-default branches **top to bottom**; first branch whose `when` map fully matches wins.
   - If none match, use the single `default: true` branch if present.
   - Otherwise **skip** the command (no pane).
3. Substitute `{{var}}` in the resolved template and spawn a PTY.
4. If **every** command is skipped → **422** with an HTML error page; no session is created.

Pane tab labels use the **job command index** (`cmd_index`), not the pane ordinal, so
labels stay correct when earlier commands are skipped.

## Authoring validation (save time)

`validate.Command` runs when a job is created or updated:

| Rule | Error |
|------|-------|
| `branches` empty | `template` required |
| `branches` non-empty | each branch needs a non-empty `template` |
| Each `when` regex | must compile |
| At most one | `default: true` per command |
| Default branch | must not have `when` clauses |
| Non-default branch | must have at least one `when` entry |

## Security

Unchanged from simple templates:

- Query-param values are validated by per-variable regex **before** branch evaluation.
- Branch `template` strings are static job definition — never derived from raw user input.
- Branch `when` patterns are compiled at job save; evaluation only reads already-validated values.
- Substituted commands are never sent to the browser.

## Out of scope (v1)

- Computed/derived variables
- Inline `{{if}}` template DSL
- OR logic within a single branch (use separate branches instead)
- Skipped-command placeholder tabs (skipped = no pane)
