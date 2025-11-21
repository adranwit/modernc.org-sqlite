package extfunc

import (
	"database/sql"
	"database/sql/driver"
	"testing"

	_ "modernc.org/sqlite"
	sqlite "modernc.org/sqlite"
)

func TestRegisterScalar_add2(t *testing.T) {
	db, err := sql.Open("sqlite", "file::memory:")
	if err != nil {
		t.Fatalf("failed to open database: %v", err)
	}
	defer db.Close()

	if err := RegisterScalar(FunctionSpec{
		Name:          "add2",
		NArgs:         1,
		Deterministic: true,
		Impl: func(_ *sqlite.FunctionContext, args []driver.Value) (driver.Value, error) {
			if len(args) != 1 {
				return nil, nil
			}
			if v, ok := args[0].(int64); ok {
				return v + 2, nil
			}
			return nil, nil
		},
	}); err != nil {
		t.Fatalf("RegisterScalar failed: %v", err)
	}

	row := db.QueryRow("select add2(40)")
	var got int
	if err := row.Scan(&got); err != nil {
		t.Fatalf("scan failed: %v", err)
	}
	if got != 42 {
		t.Fatalf("unexpected result: got %d, want %d", got, 42)
	}
}
