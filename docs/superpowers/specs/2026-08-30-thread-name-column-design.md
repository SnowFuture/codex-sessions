# Thread Name Column Design

## Goal

Show the user-assigned Codex thread name in the session picker without changing resume behavior or treating Codex's generated conversation title as a user-assigned name.

The table will become:

```text
Updated | Thread Name | Session ID | Directory | Last Action
```

An unnamed session displays `-`. The selected value returned by the picker remains the session ID, including when `--no-resume` is used.

## Source of truth

Read `session_index.jsonl` next to the sessions directory. For the default sessions directory this is:

```text
~/.codex/session_index.jsonl
~/.codex/sessions/
```

For `--sessions-dir /path/sessions`, use `/path/session_index.jsonl`. This keeps custom Codex homes working without introducing another command-line flag.

Each valid index record has this shape:

```json
{"id":"<session-id>","thread_name":"<user-name>","updated_at":"<timestamp>"}
```

The file is append-only. Process records in file order and let the last valid record for an ID replace earlier values. `updated_at` is informational and does not override append order. A valid empty `thread_name` clears the name and therefore displays as `-`.

Do not read `state_5.sqlite`. Its schema is an internal Codex implementation detail, it may be locked by a running Codex process, and using it would require an additional SQLite dependency. Do not use the generated `threads.title` value as a fallback.

## Data flow and component changes

Extend `sessions.Session` with `ThreadName string`.

During `sessions.Load`:

1. Discover and parse rollout JSONL files exactly as today.
2. Read the sibling `session_index.jsonl` into a map keyed by session ID.
3. Apply indexed names to the aggregated sessions before returning them.
4. Ignore index entries whose IDs have no discovered session.
5. Keep the existing final sort by session update time.

The UI will add `ThreadName` to each row's lower-cased fuzzy-search key. The table will insert `Thread Name` between `Updated` and `Session ID`. Name cells will use `-` for an empty value and a maximum screen width of 32 cells so `tview` handles Unicode-aware terminal clipping. The remaining columns retain their current contents and relative expansion behavior; `Last Action` remains the most expandable column.

## Error handling and compatibility

A missing `session_index.jsonl` is normal and produces unnamed rows without a warning. This preserves compatibility with older Codex installations and arbitrary fixture directories.

For an unreadable index or malformed non-empty line, continue loading rollout sessions and join the index error into the existing best-effort load error. `main` will continue showing the warning on stderr and in the TUI status line. Valid lines before and after a malformed line remain usable.

The index reader will use the same 16 MiB per-line safety limit as rollout parsing. Empty lines are ignored. A JSON object without a non-empty `id` is malformed; a missing or empty `thread_name` is valid and represents an unnamed session.

No behavior changes are made to deletion, `codex resume`, extra resume arguments, or `--no-resume`. Deleting a session will not rewrite Codex's append-only name index.

## Testing

Add focused tests covering:

- missing index file produces unnamed sessions without an error;
- one indexed name is joined to the matching session;
- the last valid duplicate ID wins;
- an empty later name clears an earlier name;
- malformed index lines return a warning while valid names and sessions still load;
- a custom sessions directory resolves the sibling index path;
- thread names participate in fuzzy search;
- an empty thread name renders as `-` and a named row populates the new column;
- session selection and `--no-resume` continue returning the ID, not the name.

Run `go test ./...` and `go vet ./...`, then manually verify the TUI at narrow and wide terminal widths with both Chinese and ASCII thread names.
