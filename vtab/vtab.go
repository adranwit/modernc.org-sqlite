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

// ConstraintOp describes the operator used in a constraint on a virtual
// table column. It loosely mirrors the op field of sqlite3_index_constraint.
type ConstraintOp int

const (
	OpEQ ConstraintOp = iota
	OpGT
	OpLE
	OpLT
	OpGE
	OpMATCH // "MATCH" operator (e.g. for FTS or KNN semantics)
)

// Constraint describes a single WHERE-clause constraint that SQLite is
// considering pushing down to the virtual table. Column is the zero-based
// column index; Op is the operator; Usable indicates whether the constraint is
// valid for the current plan. ArgIndex is the index into the argv array
// passed to Cursor.Filter when SQLite asks the vtab to execute a plan that
// uses this constraint.
type Constraint struct {
	Column   int
	Op       ConstraintOp
	Usable   bool
	ArgIndex int
}

// OrderBy describes a single ORDER BY term for a query involving a virtual
// table.
type OrderBy struct {
	Column int
	Desc   bool
}

// IndexInfo holds information about constraints and orderings for a virtual
// table query. It is the Go analogue of sqlite3_index_info. The engine's
// vtabBestIndexTrampoline is responsible for populating this structure from
// sqlite3_index_info before calling Table.BestIndex, and for writing any
// fields back after the call.
type IndexInfo struct {
	// Constraints lists the WHERE-clause constraints that may be used by the
	// virtual table to speed up lookups.
	Constraints []Constraint

	// OrderBy lists the ORDER BY terms in the query. A vtab can set
	// OrderByConsumed to true to indicate that it returns rows in this order,
	// allowing SQLite to skip a separate sort.
	OrderBy []OrderBy

	// The following fields are hints from the vtab back to SQLite.

	// IdxNum and IdxStr are an arbitrary number and string chosen by the
	// virtual table to identify the chosen plan. They are passed back to the
	// vtab in Cursor.Filter so it can distinguish between strategies.
	IdxNum int
	IdxStr string

	// OrderByConsumed indicates that the virtual table will return rows in the
	// order requested by OrderBy.
	OrderByConsumed bool

	// EstimatedCost and EstimatedRows provide planner cost hints analogous to
	// sqlite3_index_info.estimatedCost / estimatedRows.
	EstimatedCost float64
	EstimatedRows int64
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
