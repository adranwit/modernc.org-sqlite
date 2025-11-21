package vtab

import (
	"database/sql"
	"database/sql/driver"
	"errors"
)

// Value is the value type passed to and from virtual table cursors. It
// aliases database/sql/driver.Value to avoid exposing low-level details to
// module authors while remaining compatible with the driver.
type Value = driver.Value

// Context carries information that a Module may need when creating or
// connecting a table instance. It intentionally does not expose *sql.DB to
// avoid leaking database/sql internals into the vtab API. Additional fields
// may be added in the future as needed.
type Context struct{}

// Module represents a virtual table module, analogous to sqlite3_module in
// the SQLite C API. Implementations are responsible for creating and
// connecting table instances.
type Module interface {
	// Create is called to create a new virtual table. args corresponds to the
	// argv array passed to xCreate in the SQLite C API: it contains the module
	// name, the database name, the table name, and module arguments.
	Create(ctx Context, args []string) (Table, error)

	// Connect is called to connect to an existing virtual table. Its
	// semantics mirror xConnect in the SQLite C API.
	Connect(ctx Context, args []string) (Table, error)
}

// Table represents a single virtual table instance (the Go analogue of
// sqlite3_vtab and its associated methods).
type Table interface {
	// BestIndex allows the virtual table to inform SQLite about which
	// constraints and orderings it can efficiently support. The IndexInfo
	// structure mirrors sqlite3_index_info.
	BestIndex(info *IndexInfo) error

	// Open creates a new cursor for scanning the table.
	Open() (Cursor, error)

	// Disconnect is called to disconnect from a table instance (xDisconnect).
	Disconnect() error

	// Destroy is called when a table is dropped (xDestroy).
	Destroy() error
}

// Cursor represents a cursor over a virtual table (sqlite3_vtab_cursor).
type Cursor interface {
	// Filter corresponds to xFilter. idxNum and idxStr are the chosen index
	// number and string; vals are the constraint arguments.
	Filter(idxNum int, idxStr string, vals []Value) error

	// Next advances the cursor to the next row (xNext).
	Next() error

	// Eof reports whether the cursor is past the last row (xEof != 0).
	Eof() bool

	// Column returns the value of the specified column in the current row
	// (xColumn).
	Column(col int) (Value, error)

	// Rowid returns the current rowid (xRowid).
	Rowid() (int64, error)

	// Close closes the cursor (xClose).
	Close() error
}

// IndexInfo holds information about constraints and orderings for a virtual
// table query. It is intentionally minimal at this stage and can be expanded
// as the vtab integration is implemented.
type IndexInfo struct {
	// Placeholder for future fields reflecting sqlite3_index_info.
}

// ErrNotImplemented is returned by RegisterModule when the underlying engine
// has not yet installed a registration hook. External projects can depend on
// the vtab API surface before the low-level bridge to sqlite3_create_module
// is fully wired; once the engine sets the hook via SetRegisterFunc,
// RegisterModule will forward calls to it.
var ErrNotImplemented = errors.New("vtab: RegisterModule not wired into engine")

// registerHook is installed by the engine package (modernc.org/sqlite) via
// SetRegisterFunc. It is invoked by RegisterModule to perform the actual
// module registration.
var registerHook func(name string, m Module) error

// SetRegisterFunc is intended to be called by the engine package to provide
// the concrete implementation of module registration. External callers
// should use RegisterModule instead.
func SetRegisterFunc(fn func(name string, m Module) error) {
	registerHook = fn
}

// RegisterModule registers a virtual table module with the provided *sql.DB.
//
// In future revisions, this will:
//   - bridge the Module implementation to a sqlite3_module struct,
//   - call sqlite3_create_module_v2 on the underlying connection, and
//   - make the module available to CREATE VIRTUAL TABLE statements.
func RegisterModule(db *sql.DB, name string, m Module) error {
	_ = db // currently unused; kept for potential future per-DB registration.
	if registerHook == nil {
		return ErrNotImplemented
	}
	if name == "" {
		return errors.New("vtab: module name must be non-empty")
	}
	if m == nil {
		return errors.New("vtab: module implementation is nil")
	}
	return registerHook(name, m)
}
