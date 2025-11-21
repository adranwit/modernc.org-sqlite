// Package vtab defines a Go-facing API for implementing SQLite virtual table
// modules on top of the modernc.org/sqlite driver.
//
// NOTE: At this stage, the RegisterModule function is a placeholder that
// documents the intended API surface but does not yet bridge to the
// underlying sqlite3_create_module/_v2 functions.
package vtab
