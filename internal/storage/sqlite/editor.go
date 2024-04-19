package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/mattn/go-sqlite3"

	"github.com/GintGld/fizteh-radio/internal/models"
	"github.com/GintGld/fizteh-radio/internal/storage"
)

// SaveEditor saves editor.
func (s *Storage) SaveEditor(ctx context.Context, login string, passHash []byte) (int64, error) {
	const op = "storage.sqlite.SaveEditor"

	stmt, err := s.db.Prepare("INSERT INTO editors(login, pass_hash) VALUES(?, ?)")
	if err != nil {
		return models.ErrEditorID, fmt.Errorf("%s: %w", op, err)
	}
	defer stmt.Close()

	res, err := stmt.ExecContext(ctx, login, passHash)
	if err != nil {
		var sqliteErr sqlite3.Error
		if errors.As(err, &sqliteErr) && sqliteErr.ExtendedCode == sqlite3.ErrConstraintUnique {
			return models.ErrEditorID, fmt.Errorf("%s: %w", op, storage.ErrEditorExists)
		}
		if errors.Is(err, context.DeadlineExceeded) {
			return models.ErrEditorID, storage.ErrContextCancelled
		}

		return models.ErrEditorID, fmt.Errorf("%s: %w", op, err)
	}

	id, err := res.LastInsertId()
	if err != nil {
		return models.ErrEditorID, fmt.Errorf("%s: %w", op, err)
	}

	return id, nil
}

// Editor returns editor by login.
func (s *Storage) Editor(ctx context.Context, id int64) (models.Editor, error) {
	const op = "storage.sqlite.Editor"

	stmt, err := s.db.Prepare("SELECT id, login, pass_hash FROM editors WHERE id = ?")
	if err != nil {
		return models.Editor{}, fmt.Errorf("%s: %w", op, err)
	}
	defer stmt.Close()

	row := stmt.QueryRowContext(ctx, id)

	var editor models.Editor
	err = row.Scan(&editor.ID, &editor.Login, &editor.PassHash)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return models.Editor{}, fmt.Errorf("%s: %w", op, storage.ErrEditorNotFound)
		}
		if errors.Is(err, context.DeadlineExceeded) {
			return models.Editor{}, storage.ErrContextCancelled
		}

		return models.Editor{}, fmt.Errorf("%s: %w", op, err)
	}

	return editor, nil
}

func (s *Storage) EditorByLogin(ctx context.Context, login string) (models.Editor, error) {
	const op = "storage.sqlite.EditorByLogin"

	stmt, err := s.db.Prepare("SELECT id, login, pass_hash FROM editors WHERE login = ?")
	if err != nil {
		return models.Editor{}, fmt.Errorf("%s: %w", op, err)
	}
	defer stmt.Close()

	row := stmt.QueryRowContext(ctx, login)

	var editor models.Editor
	err = row.Scan(&editor.ID, &editor.Login, &editor.PassHash)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return models.Editor{}, fmt.Errorf("%s: %w", op, storage.ErrEditorNotFound)
		}
		if errors.Is(err, context.DeadlineExceeded) {
			return models.Editor{}, storage.ErrContextCancelled
		}

		return models.Editor{}, fmt.Errorf("%s: %w", op, err)
	}

	return editor, nil
}

// AllEditors returns all editors
//
// If error occures during parsing, returnes already parsed editors.
func (s *Storage) AllEditors(ctx context.Context) ([]models.Editor, error) {
	const op = "storage.sqlite.AllEditors"

	stmt, err := s.db.Prepare("SELECT id, login, pass_hash FROM editors")
	if err != nil {
		return []models.Editor{}, fmt.Errorf("%s: %w", op, err)
	}
	defer stmt.Close()

	rows, err := stmt.QueryContext(ctx)
	if err != nil {
		return []models.Editor{}, fmt.Errorf("%s: %w", op, err)
	}
	defer rows.Close()

	editors := make([]models.Editor, 0)
	var editor models.Editor
	for rows.Next() {
		if err = rows.Scan(&editor.ID, &editor.Login, &editor.PassHash); err != nil {
			if errors.Is(err, context.DeadlineExceeded) {
				return []models.Editor{}, storage.ErrContextCancelled
			}
			return editors, fmt.Errorf("%s: %w", op, err)
		}
		editors = append(editors, editor)
	}

	return editors, nil
}

// DeleteEditor deletes editor.
func (s *Storage) DeleteEditor(ctx context.Context, id int64) error {
	const op = "storage.sqlite.DeleteEditor"

	stmt, err := s.db.Prepare("DELETE FROM editors WHERE id = ?")
	if err != nil {
		return fmt.Errorf("%s: %w", op, err)
	}
	defer stmt.Close()

	res, err := stmt.ExecContext(ctx, id)
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			return storage.ErrContextCancelled
		}
		return fmt.Errorf("%s: %w", op, err)
	}
	affectedRows, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("%s: %w", op, err)
	}
	if affectedRows == 0 {
		return storage.ErrEditorNotFound
	}

	return nil
}
