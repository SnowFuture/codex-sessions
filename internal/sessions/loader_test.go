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
