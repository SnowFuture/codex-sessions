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
