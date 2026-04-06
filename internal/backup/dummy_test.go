package backup

import (
	"context"
	"io"
	"strings"
	"testing"
)

type dummyDumper struct{}

func (d *dummyDumper) Dump(ctx context.Context, w io.Writer, cfg DumpConfig) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	payload := "CREATE TABLE users (id INT, name TEXT);\n" +
		"INSERT INTO users VALUES (1, 'alice');\n" +
		"INSERT INTO users VALUES (2, 'bob');\n"
	_, err := io.Copy(w, strings.NewReader(payload))
	return err
}

func TestDummyDumperWritesRecords(t *testing.T) {
	t.Parallel()

	d := &dummyDumper{}
	var sb strings.Builder

	err := d.Dump(context.Background(), &sb, DumpConfig{Type: "postgres", Database: "dummy"})
	if err != nil {
		t.Fatalf("Dump() error = %v", err)
	}

	out := sb.String()
	if !strings.Contains(out, "INSERT INTO users VALUES (1, 'alice')") {
		t.Fatalf("expected alice row in dump output: %q", out)
	}
	if !strings.Contains(out, "INSERT INTO users VALUES (2, 'bob')") {
		t.Fatalf("expected bob row in dump output: %q", out)
	}
}
