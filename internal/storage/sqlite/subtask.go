package sqlite

import (
	"context"
	"database/sql"
)

// All methods with "Sub" in name means
// "<Main method called from outside>Sub<Subtask called from main method>"
// The purpose is to keep one sql query per one function.

type statementBuilder interface {
	PrepareContext(ctx context.Context, query string) (*sql.Stmt, error)
}
