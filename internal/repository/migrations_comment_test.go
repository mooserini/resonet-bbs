package repository

import (
	"strings"
	"testing"
)

// execSQLBatch splits on every ";", including ones inside comments, so a
// semicolon in a migration comment turns the rest of the line into a broken
// statement and the board fails to start.
func TestMigrationCommentsHaveNoSemicolons(t *testing.T) {
	entries, err := migrationFS.ReadDir("migrations")
	if err != nil {
		t.Fatalf("read migrations: %v", err)
	}
	for _, e := range entries {
		body, err := migrationFS.ReadFile("migrations/" + e.Name())
		if err != nil {
			t.Fatalf("read %s: %v", e.Name(), err)
		}
		for i, line := range strings.Split(string(body), "\n") {
			if idx := strings.Index(line, "--"); idx >= 0 && strings.Contains(line[idx:], ";") {
				t.Errorf("%s:%d: semicolon in comment breaks the migration runner", e.Name(), i+1)
			}
		}
	}
}
