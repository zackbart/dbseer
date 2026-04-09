package db

import "errors"

// ErrReadonly is returned when a write operation is attempted against a pool
// opened in read-only mode.
var ErrReadonly = errors.New("server is in read-only mode")

// ErrTableReadonly is returned when a write operation is attempted against a
// table that is not editable (view, materialized view, or no primary key).
var ErrTableReadonly = errors.New("table is read-only")
