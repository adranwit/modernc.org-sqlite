package extfunc

import (
	"database/sql/driver"

	sqlite "modernc.org/sqlite"
)

// RegisterScalar registers one or more scalar SQL functions on the provided
// *sql.DB. It is a small convenience wrapper around sqlite.RegisterFunction
// and FunctionImpl.Scalar, intended for external consumers such as
// github.com/viant/sqlite-vec.
//
// Each function is registered with deterministic semantics when the
// Deterministic field of FunctionSpec is set. Functions are registered on the
// underlying driver and are therefore visible to all future connections opened
// by this process using the same driver.

// RegisterScalar registers one or more scalar functions with the underlying
// sqlite driver. Once registered, functions are available to all future
// connections opened via database/sql using this driver (including DB).
//
// Registration is process-global and is safe to call multiple times, but
// attempts to register the same function name more than once will return an
// error from the underlying driver.
func RegisterScalar(specs ...FunctionSpec) error {
	for _, spec := range specs {
		impl := &sqlite.FunctionImpl{
			NArgs:         spec.NArgs,
			Deterministic: spec.Deterministic,
			Scalar: func(ctx *sqlite.FunctionContext, args []driver.Value) (driver.Value, error) {
				return spec.Impl(ctx, args)
			},
		}
		if err := sqlite.RegisterFunction(spec.Name, impl); err != nil {
			return err
		}
	}

	return nil
}

// FunctionSpec describes a scalar SQL function to be registered via
// RegisterScalar.
type FunctionSpec struct {
	// Name is the SQL function name.
	Name string

	// NArgs is the required number of arguments that the function accepts.
	// If NArgs is negative, then the function is variadic.
	NArgs int32

	// Deterministic controls whether SQLITE_DETERMINISTIC is set for the
	// function, indicating that given the same inputs it will always produce
	// the same output.
	Deterministic bool

	// Impl is invoked when the SQL function is executed.
	Impl func(ctx *sqlite.FunctionContext, args []driver.Value) (driver.Value, error)
}
