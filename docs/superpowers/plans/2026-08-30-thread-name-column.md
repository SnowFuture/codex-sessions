# Thread Name Column Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add user-assigned Codex thread names to the session picker, fuzzy search, and table while preserving session-ID-based resume output.

**Architecture:** Extend the aggregated `Session` model with `ThreadName`, then best-effort join names from the append-only `session_index.jsonl` beside the configured sessions directory. Keep presentation concerns in `internal/ui`: render an empty name as `-`, constrain the name column by terminal cell width, and continue returning the immutable session ID on selection.

**Tech Stack:** Go 1.25, standard-library JSON/filesystem/error packages, `tview`, `tcell`, and `fuzzysearch`.

---

Implement this plan in a dedicated worktree based on commit `5d28241`. Do not change or query Codex's `state_5.sqlite`; the approved design uses only `session_index.jsonl`.

## File map

- Modify `internal/sessions/session.go`: add the thread-name field to the aggregate model.
- Modify `internal/sessions/loader.go`: resolve, parse, and best-effort join the sibling name index.
- Create `internal/sessions/loader_test.go`: cover index parsing, duplicate names, empty names, missing files, malformed lines, and custom roots.
- Modify `internal/ui/ui.go`: include names in search and add the bounded `Thread Name` table column.
- Create `internal/ui/ui_test.go`: cover search, rendering, Unicode text, and ID-based selection.
- Modify `main.go`: route `--no-resume` output through a tiny writer helper so the exact output contract is testable.
- Create `main_test.go`: assert that machine-readable selection output remains `<session-id>\n`.
- Modify `README.md`: document the new column and search field.

### Task 1: Load thread names from the sibling JSONL index

**Files:**
- Modify: `internal/sessions/session.go:12-20`
- Modify: `internal/sessions/loader.go:17-21,96-107,221-231`
- Create: `internal/sessions/loader_test.go`

- [ ] **Step 1: Write failing loader tests**

Create `internal/sessions/loader_test.go` with fixtures that use a custom Codex home and a sibling `session_index.jsonl`:

```go
package sessions

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadJoinsLatestThreadNameFromSiblingIndex(t *testing.T) {
	home := t.TempDir()
	sessionsDir := filepath.Join(home, "custom-sessions")
	writeSessionFixture(t, sessionsDir, "session-a")
	writeIndexFixture(t, home,
		`{"id":"session-a","thread_name":"old name","updated_at":"2026-08-30T08:00:00Z"}`,
		`{"id":"ignored-id","thread_name":"not loaded","updated_at":"2026-08-30T08:01:00Z"}`,
		`{"id":"session-a","thread_name":"分析架构","updated_at":"2026-08-30T08:02:00Z"}`,
	)

	got, err := Load(sessionsDir)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("Load() returned %d sessions, want 1", len(got))
	}
	if got[0].ThreadName != "分析架构" {
		t.Fatalf("ThreadName = %q, want %q", got[0].ThreadName, "分析架构")
	}
}

func TestLoadMissingThreadIndexIsNotAnError(t *testing.T) {
	home := t.TempDir()
	sessionsDir := filepath.Join(home, "sessions")
	writeSessionFixture(t, sessionsDir, "session-a")

	got, err := Load(sessionsDir)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if got[0].ThreadName != "" {
		t.Fatalf("ThreadName = %q, want empty", got[0].ThreadName)
	}
}

func TestLoadEmptyThreadNameClearsEarlierName(t *testing.T) {
	home := t.TempDir()
	sessionsDir := filepath.Join(home, "sessions")
	writeSessionFixture(t, sessionsDir, "session-a")
	writeIndexFixture(t, home,
		`{"id":"session-a","thread_name":"old name","updated_at":"2026-08-30T08:00:00Z"}`,
		`{"id":"session-a","thread_name":"","updated_at":"2026-08-30T08:01:00Z"}`,
	)

	got, err := Load(sessionsDir)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if got[0].ThreadName != "" {
		t.Fatalf("ThreadName = %q, want empty", got[0].ThreadName)
	}
}

func TestLoadMalformedThreadIndexKeepsValidEntries(t *testing.T) {
	home := t.TempDir()
	sessionsDir := filepath.Join(home, "sessions")
	writeSessionFixture(t, sessionsDir, "session-a")
	writeSessionFixture(t, sessionsDir, "session-b")
	writeIndexFixture(t, home,
		`{"id":"session-a","thread_name":"alpha","updated_at":"2026-08-30T08:00:00Z"}`,
		`{"id":`,
		`{"id":"session-b","thread_name":"beta","updated_at":"2026-08-30T08:02:00Z"}`,
	)

	got, err := Load(sessionsDir)
	if err == nil || !strings.Contains(err.Error(), "session_index.jsonl line 2") {
		t.Fatalf("Load() error = %v, want line 2 warning", err)
	}
	if len(got) != 2 {
		t.Fatalf("Load() returned %d sessions, want 2", len(got))
	}
	names := make(map[string]string, len(got))
	for _, session := range got {
		names[session.ID] = session.ThreadName
	}
	if names["session-a"] != "alpha" || names["session-b"] != "beta" {
		t.Fatalf("loaded names = %#v, want both valid records", names)
	}
}

func writeSessionFixture(t *testing.T, sessionsDir, id string) {
	t.Helper()
	if err := os.MkdirAll(sessionsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	contents := fmt.Sprintf(
		"{\"timestamp\":\"2026-08-30T08:00:00Z\",\"type\":\"session_meta\",\"payload\":{\"id\":%q,\"timestamp\":\"2026-08-30T08:00:00Z\",\"cwd\":\"/tmp/project\"}}\n",
		id,
	)
	path := filepath.Join(sessionsDir, id+".jsonl")
	if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
		t.Fatal(err)
	}
}

func writeIndexFixture(t *testing.T, home string, lines ...string) {
	t.Helper()
	contents := strings.Join(lines, "\n") + "\n"
	path := filepath.Join(home, "session_index.jsonl")
	if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
		t.Fatal(err)
	}
}
```

- [ ] **Step 2: Run the tests and verify the new field is missing**

Run:

```bash
go test ./internal/sessions -run 'TestLoad(Joins|Missing|Empty|Malformed)' -v
```

Expected: compilation fails because `Session.ThreadName` is not defined.

- [ ] **Step 3: Add the model field and index parser**

Add `ThreadName` to `Session` in `internal/sessions/session.go`:

```go
type Session struct {
	ID         string
	CreatedAt  time.Time
	UpdatedAt  time.Time
	WorkingDir string
	ThreadName string
	LastAction string
	FilePaths  []string
}
```

Add the index wire type and parser to `internal/sessions/loader.go`:

```go
type sessionIndexEntry struct {
	ID         string `json:"id"`
	ThreadName string `json:"thread_name"`
	UpdatedAt  string `json:"updated_at"`
}

func loadThreadNames(sessionsDir string) (map[string]string, error) {
	indexPath := filepath.Join(filepath.Dir(filepath.Clean(sessionsDir)), "session_index.jsonl")
	file, err := os.Open(indexPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return map[string]string{}, nil
		}
		return map[string]string{}, fmt.Errorf("open thread name index %s: %w", indexPath, err)
	}
	defer file.Close()

	names := make(map[string]string)
	var combinedErr error
	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 64*1024), maxLineSize)
	for lineNumber := 1; scanner.Scan(); lineNumber++ {
		line := bytes.TrimSpace(scanner.Bytes())
		if len(line) == 0 {
			continue
		}
		var entry sessionIndexEntry
		if err := json.Unmarshal(line, &entry); err != nil {
			combinedErr = errors.Join(combinedErr,
				fmt.Errorf("parse %s line %d: %w", filepath.Base(indexPath), lineNumber, err))
			continue
		}
		if strings.TrimSpace(entry.ID) == "" {
			combinedErr = errors.Join(combinedErr,
				fmt.Errorf("parse %s line %d: missing session id", filepath.Base(indexPath), lineNumber))
			continue
		}
		names[entry.ID] = entry.ThreadName
	}
	if err := scanner.Err(); err != nil {
		combinedErr = errors.Join(combinedErr, fmt.Errorf("read thread name index %s: %w", indexPath, err))
	}
	return names, combinedErr
}
```

After `filepath.WalkDir` succeeds in `Load`, load the index and join any warning into the existing best-effort error:

```go
	threadNames, nameErr := loadThreadNames(root)
	if nameErr != nil {
		combinedErr = errors.Join(combinedErr, nameErr)
	}

	sessions := make([]Session, 0, len(byID))
	for _, s := range byID {
		if name, ok := threadNames[s.ID]; ok {
			s.ThreadName = name
		}
		sort.Strings(s.FilePaths)
		sessions = append(sessions, *s)
	}
```

Keep the existing final update-time sort and return statement below this block.

- [ ] **Step 4: Run loader tests and all package tests**

Run:

```bash
go test ./internal/sessions -v
go test ./...
```

Expected: all tests pass; packages without tests continue to report `[no test files]` until later tasks.

- [ ] **Step 5: Commit the loader slice**

```bash
git add internal/sessions/session.go internal/sessions/loader.go internal/sessions/loader_test.go
git commit -m "feat: load Codex thread names"
```

### Task 2: Add thread names to fuzzy search and the table

**Files:**
- Modify: `internal/ui/ui.go:53-66,237-275,359-392`
- Create: `internal/ui/ui_test.go`

- [ ] **Step 1: Write failing UI tests**

Create `internal/ui/ui_test.go`:

```go
package ui

import (
	"testing"

	"github.com/Uri2001/codex-sessions/internal/sessions"
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

func TestThreadNameParticipatesInSearch(t *testing.T) {
	m := newModel([]sessions.Session{
		{ID: "session-a", ThreadName: "agent-sandbox"},
		{ID: "session-b", ThreadName: "release-check"},
	}, "/tmp/sessions", "")
	m.query = "agent-sandbox"
	m.applyFilter()

	if len(m.filtered) != 1 || m.entries[m.filtered[0]].session.ID != "session-a" {
		t.Fatalf("filtered rows = %#v, want only session-a", m.filtered)
	}
}

func TestRefreshTableShowsThreadNameAndUnnamedPlaceholder(t *testing.T) {
	m := newModel([]sessions.Session{
		{ID: "session-a", ThreadName: "分析架构"},
		{ID: "session-b"},
	}, "/tmp/sessions", "")
	m.table = tview.NewTable()
	m.applyFilter()
	m.refreshTable()

	if got := m.table.GetCell(0, 1).Text; got != "Thread Name" {
		t.Fatalf("header = %q, want Thread Name", got)
	}
	if got := m.table.GetCell(1, 1).Text; got != "分析架构" {
		t.Fatalf("named cell = %q, want 分析架构", got)
	}
	if got := m.table.GetCell(2, 1).Text; got != "-" {
		t.Fatalf("unnamed cell = %q, want -", got)
	}
	if got := m.table.GetCell(1, 1).MaxWidth; got != 32 {
		t.Fatalf("name MaxWidth = %d, want 32", got)
	}
}

func TestEnterStillSelectsSessionID(t *testing.T) {
	m := newModel([]sessions.Session{{
		ID:         "session-a",
		ThreadName: "friendly-name",
	}}, "/tmp/sessions", "")
	m.app = tview.NewApplication()
	m.applyFilter()

	m.handleEvent(tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone))

	if m.resumeID != "session-a" {
		t.Fatalf("resumeID = %q, want session-a", m.resumeID)
	}
}
```

- [ ] **Step 2: Run UI tests and verify failure**

Run:

```bash
go test ./internal/ui -run 'Test(ThreadName|RefreshTable|Enter)' -v
```

Expected: search and table tests fail because `ThreadName` is not indexed or rendered.

- [ ] **Step 3: Add search and rendering behavior**

In `newModel`, add `sess.ThreadName` to the search-key fields immediately after the ID:

```go
		key := strings.ToLower(strings.Join([]string{
			sess.ID,
			sess.ThreadName,
			sess.WorkingDir,
			sess.LastAction,
			sess.CreatedAt.Format(time.RFC3339),
			sess.UpdatedAt.Format(time.RFC3339),
		}, " "))
```

Insert the new header at column 1 and shift the other headers right:

```go
	m.table.SetCell(0, 0, tview.NewTableCell("Updated").
		SetSelectable(false).
		SetStyle(headerStyle))
	m.table.SetCell(0, 1, tview.NewTableCell("Thread Name").
		SetSelectable(false).
		SetStyle(headerStyle))
	m.table.SetCell(0, 2, tview.NewTableCell("Session ID").
		SetSelectable(false).
		SetStyle(headerStyle))
	m.table.SetCell(0, 3, tview.NewTableCell("Directory").
		SetSelectable(false).
		SetStyle(headerStyle))
	m.table.SetCell(0, 4, tview.NewTableCell("Last Action").
		SetSelectable(false).
		SetStyle(headerStyle))
```

Replace the row cell block with:

```go
		m.table.SetCell(row, 0, tview.NewTableCell(formatTimestamp(sess.UpdatedAt)).
			SetExpansion(1))
		m.table.SetCell(row, 1, tview.NewTableCell(formatThreadName(sess.ThreadName)).
			SetMaxWidth(32).
			SetExpansion(1))
		m.table.SetCell(row, 2, tview.NewTableCell(sess.ID).
			SetExpansion(1))
		m.table.SetCell(row, 3, tview.NewTableCell(abbreviatePath(sess.WorkingDir, 40)).
			SetExpansion(1))
		m.table.SetCell(row, 4, tview.NewTableCell(truncateText(sess.LastAction, 80)).
			SetExpansion(2))
```

Add this presentation helper near the existing formatting helpers:

```go
func formatThreadName(name string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		return "-"
	}
	return name
}
```

Do not pre-slice the name by bytes; `SetMaxWidth(32)` lets `tview` clip by terminal screen width and keeps Chinese text valid UTF-8.

- [ ] **Step 4: Run UI and full tests**

Run:

```bash
go test ./internal/ui -v
go test ./...
```

Expected: all tests pass, including the assertion that Enter stores `session-a`, not `friendly-name`.

- [ ] **Step 5: Commit the UI slice**

```bash
git add internal/ui/ui.go internal/ui/ui_test.go
git commit -m "feat: show thread names in session picker"
```

### Task 3: Lock down `--no-resume` output

**Files:**
- Modify: `main.go:43-45`
- Create: `main_test.go`

- [ ] **Step 1: Write the failing stdout contract test**

Create `main_test.go`:

```go
package main

import (
	"bytes"
	"testing"
)

func TestPrintSelectedIDWritesOnlyIDAndNewline(t *testing.T) {
	var output bytes.Buffer
	printSelectedID(&output, "session-a")

	if got, want := output.String(), "session-a\n"; got != want {
		t.Fatalf("output = %q, want %q", got, want)
	}
}
```

- [ ] **Step 2: Run the test and verify the helper is missing**

Run:

```bash
go test . -run TestPrintSelectedIDWritesOnlyIDAndNewline -v
```

Expected: compilation fails with `undefined: printSelectedID`.

- [ ] **Step 3: Add the minimal writer helper**

Add `io` to the imports in `main.go`, replace the `fmt.Println` branch, and define the helper:

```go
import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
)
```

```go
	if *flagNoResume {
		printSelectedID(os.Stdout, selectedID)
		return
	}
```

```go
func printSelectedID(w io.Writer, selectedID string) {
	fmt.Fprintln(w, selectedID)
}
```

Keep `runCodexResume` and its `sessionID` argument unchanged.

- [ ] **Step 4: Run main and full tests**

Run:

```bash
go test . -run TestPrintSelectedIDWritesOnlyIDAndNewline -v
go test ./...
```

Expected: all tests pass and the exact stdout contract is `session-a\n`.

- [ ] **Step 5: Commit the output-contract slice**

```bash
git add main.go main_test.go
git commit -m "test: preserve selected session id output"
```

### Task 4: Document and verify the complete feature

**Files:**
- Modify: `README.md:7-13,35-43`

- [ ] **Step 1: Update user-facing documentation**

Replace the fuzzy-search feature bullet and add a thread-name bullet in `README.md`:

```markdown
- **Fuzzy search** as you type across thread names, session IDs, working directories, timestamps, and last actions.
- **Thread names** assigned with Codex `/rename` appear in their own column; unnamed sessions display `-`.
```

After the flags table, add:

```markdown
The session list reads user-assigned names from `session_index.jsonl` next to the configured sessions directory. Generated conversation titles are not used as a fallback.
```

- [ ] **Step 2: Run formatting and static verification**

Run:

```bash
gofmt -w main.go main_test.go internal/sessions/session.go internal/sessions/loader.go internal/sessions/loader_test.go internal/ui/ui.go internal/ui/ui_test.go
go test ./...
go vet ./...
git diff --check
```

Expected: every command exits 0; all newly added tests pass.

- [ ] **Step 3: Build and verify against the real local index**

Run:

```bash
go build -o /tmp/codex-sessions-thread-name .
/tmp/codex-sessions-thread-name --no-resume
```

Expected in a wide terminal:

```text
Updated | Thread Name | Session ID | Directory | Last Action
...     | codex-sessions | 01a051db-26aa-7c81-a063-d37c56b901c2 | ... | ...
```

Select the named row and press Enter. Expected after the TUI restores the terminal: stdout contains the UUID, not `codex-sessions`.

Run the binary again, narrow the terminal to approximately 80 columns, and confirm the TUI remains interactive and clips columns without corrupting Chinese names. Exit with Esc without deleting any session.

- [ ] **Step 4: Commit documentation**

```bash
git add README.md
git commit -m "docs: describe thread names in session picker"
```

- [ ] **Step 5: Record final evidence**

Run:

```bash
git status --short --branch
git log -5 --oneline --decorate
```

Expected: the worktree is clean and the feature is represented by focused loader, UI, output-contract, and documentation commits after `5d28241`.
