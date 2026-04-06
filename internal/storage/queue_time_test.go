package storage

import (
	"testing"
	"time"
)

func TestParseSQLiteTimeFormats(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input string
	}{
		{name: "sqlite default", input: "2026-04-06 12:34:56"},
		{name: "sqlite nanos", input: "2026-04-06 12:34:56.123456789"},
		{name: "rfc3339", input: "2026-04-06T12:34:56Z"},
		{name: "rfc3339 nano", input: "2026-04-06T12:34:56.123456789Z"},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := parseSQLiteTime(tc.input)
			if got.IsZero() {
				t.Fatalf("parseSQLiteTime(%q) = zero time", tc.input)
			}
		})
	}
}

func TestParseSQLiteTimeInvalid(t *testing.T) {
	t.Parallel()

	got := parseSQLiteTime("not-a-time")
	if !got.Equal(time.Time{}) {
		t.Fatalf("parseSQLiteTime(invalid) = %v, want zero time", got)
	}
}
