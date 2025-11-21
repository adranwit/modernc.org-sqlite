package sqlite

import (
	"database/sql"
	"testing"

	"modernc.org/sqlite/vtab"
)

// dummyModule is a minimal vtab.Module implementation used to verify that the
// vtab bridge (registration + trampolines) works end-to-end inside this
// repository, without relying on external modules.
type dummyModule struct{}

// dummyTable implements vtab.Table for the dummy module.
type dummyTable struct{}

// dummyCursor implements vtab.Cursor for the dummy module. It returns a small
// fixed result set for testing.
type dummyCursor struct {
	rows []struct {
		rowid int64
		val   string
	}
	pos int
}

// Create implements vtab.Module.Create.
func (m *dummyModule) Create(ctx vtab.Context, args []string) (vtab.Table, error) {
	_ = ctx
	_ = args
	return &dummyTable{}, nil
}

// Connect implements vtab.Module.Connect.
func (m *dummyModule) Connect(ctx vtab.Context, args []string) (vtab.Table, error) {
	_ = ctx
	_ = args
	return &dummyTable{}, nil
}

func (t *dummyTable) BestIndex(info *vtab.IndexInfo) error {
	// Choose a fixed plan ID so we can verify that IdxNum flows through
	// sqlite3_index_info into Cursor.Filter.
	info.IdxNum = 1
	return nil
}

// Open creates a new dummyCursor.
func (t *dummyTable) Open() (vtab.Cursor, error) {
	_ = t
	return &dummyCursor{}, nil
}

// Disconnect is a no-op for the dummy table.
func (t *dummyTable) Disconnect() error { return nil }

// Destroy is a no-op for the dummy table.
func (t *dummyTable) Destroy() error { return nil }

func (c *dummyCursor) Filter(idxNum int, idxStr string, vals []vtab.Value) error {
	_ = idxStr
	_ = vals
	// Ensure that the planner-provided idxNum from BestIndex is propagated.
	// If idxNum is not 1, return a different rowset so the test would fail.
	if idxNum == 1 {
		c.rows = []struct {
			rowid int64
			val   string
		}{
			{rowid: 1, val: "alpha"},
			{rowid: 2, val: "beta"},
		}
	} else {
		c.rows = []struct {
			rowid int64
			val   string
		}{
			{rowid: 1, val: "unexpected"},
		}
	}
	c.pos = 0
	return nil
}

// Next advances the cursor.
func (c *dummyCursor) Next() error {
	if c.pos < len(c.rows) {
		c.pos++
	}
	return nil
}

// Eof reports whether the cursor is past the last row.
func (c *dummyCursor) Eof() bool { return c.pos >= len(c.rows) }

// Column returns the string value for column 0 and NULL for others.
func (c *dummyCursor) Column(col int) (vtab.Value, error) {
	if c.pos < 0 || c.pos >= len(c.rows) {
		return nil, nil
	}
	if col == 0 {
		return c.rows[c.pos].val, nil
	}
	return nil, nil
}

// Rowid returns the current rowid.
func (c *dummyCursor) Rowid() (int64, error) {
	if c.pos < 0 || c.pos >= len(c.rows) {
		return 0, nil
	}
	return c.rows[c.pos].rowid, nil
}

// Close clears the cursor state.
func (c *dummyCursor) Close() error {
	c.rows = nil
	c.pos = 0
	return nil
}

// TestDummyModuleVtab verifies that a simple vtab module implemented in Go
// can be registered and queried through the modernc.org/sqlite driver.
func TestDummyModuleVtab(t *testing.T) {
	// Open an in-memory database using this driver.
	db, err := sql.Open(driverName, ":memory:")
	if err != nil {
		t.Fatalf("sql.Open failed: %v", err)
	}
	defer db.Close()

	// Register the dummy module.
	if err := vtab.RegisterModule(db, "dummy", &dummyModule{}); err != nil {
		t.Fatalf("vtab.RegisterModule failed: %v", err)
	}

	// Create a virtual table using the dummy module.
	if _, err := db.Exec(`CREATE VIRTUAL TABLE vt USING dummy(value)`); err != nil {
		t.Fatalf("CREATE VIRTUAL TABLE vt USING dummy failed: %v", err)
	}

	// Query the virtual table; expect the two static rows defined in Filter.
	rows, err := db.Query(`SELECT rowid, value FROM vt ORDER BY rowid`)
	if err != nil {
		t.Fatalf("SELECT from vt failed: %v", err)
	}
	defer rows.Close()

	var got []string
	for rows.Next() {
		var rowid int64
		var value string
		if err := rows.Scan(&rowid, &value); err != nil {
			t.Fatalf("scan failed: %v", err)
		}
		got = append(got, value)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("rows.Err: %v", err)
	}

	if len(got) != 2 {
		t.Fatalf("expected 2 rows, got %d (%v)", len(got), got)
	}
	if got[0] != "alpha" || got[1] != "beta" {
		t.Fatalf("unexpected values from vt: %v (want [alpha beta])", got)
	}
}
