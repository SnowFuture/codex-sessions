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
