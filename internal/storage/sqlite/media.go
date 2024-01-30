package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/mattn/go-sqlite3"

	ptr "github.com/GintGld/fizteh-radio/internal/lib/utils/pointers"
	"github.com/GintGld/fizteh-radio/internal/models"
	"github.com/GintGld/fizteh-radio/internal/storage"
)

// AllMedia return all media in library.
func (s *Storage) AllMedia(ctx context.Context) ([]models.Media, error) {
	const op = "storage.sqlite.AllMedia"

	stmt, err := s.db.Prepare("SELECT id, name, author, duration FROM library")
	if err != nil {
		return []models.Media{}, fmt.Errorf("%s: %w", op, err)
	}
	defer stmt.Close()

	rows, err := stmt.QueryContext(ctx)
	if err != nil {
		return []models.Media{}, fmt.Errorf("%s: %w", op, err)
	}

	library := make([]models.Media, 0)
	var (
		media        models.Media
		id           int64
		name, author string
		durationMs   int64
	)
	for rows.Next() {
		if err = rows.Scan(&id, &name, &author, &durationMs); err != nil {
			return library, fmt.Errorf("%s: %w", op, err)
		}
		media.ID = ptr.Ptr(id)
		media.Name = ptr.Ptr(name)
		media.Author = ptr.Ptr(author)
		media.Duration = ptr.Ptr(time.Duration(durationMs) * time.Microsecond)

		library = append(library, media)

	}

	return library, nil
}

// SaveMedia saves necessary information
// about media file to db.
func (s *Storage) SaveMedia(ctx context.Context, media models.Media) (int64, error) {
	const op = "storage.sqlite.SaveMedia"

	stmt, err := s.db.Prepare("INSERT INTO library(name, author, duration, source_id) VALUES(?, ?, ?, ?)")
	if err != nil {
		return 0, fmt.Errorf("%s: %w", op, err)
	}
	defer stmt.Close()

	res, err := stmt.ExecContext(ctx, *media.Name, *media.Author, media.Duration.Microseconds(), *media.SourceID)
	if err != nil {
		var sqliteErr sqlite3.Error
		if errors.As(err, &sqliteErr) && sqliteErr.ExtendedCode == sqlite3.ErrConstraintUnique {
			return 0, fmt.Errorf("%s: %w", op, storage.ErrMediaExists)
		}

		return 0, fmt.Errorf("%s: %w", op, err)
	}

	id, err := res.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("%s: %w", op, err)
	}

	return id, nil
}

// Media return media file by id.
func (s *Storage) Media(ctx context.Context, id int64) (models.Media, error) {
	const op = "storage.sqlite.Media"

	stmt, err := s.db.Prepare("SELECT name, author, duration, source_id FROM library WHERE id = ?")
	if err != nil {
		return models.Media{}, fmt.Errorf("%s: %w", op, err)
	}
	defer stmt.Close()

	row := stmt.QueryRowContext(ctx, id)

	var (
		sourceID     int64
		name, author string
		durationMuS  int64
	)

	err = row.Scan(&name, &author, &durationMuS, &sourceID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return models.Media{}, fmt.Errorf("%s: %w", op, storage.ErrMediaNotFound)
		}

		return models.Media{}, fmt.Errorf("%s: %w", op, err)
	}

	var media models.Media

	media.ID = &id
	media.SourceID = &sourceID
	media.Name = &name
	media.Author = &author
	media.Duration = ptr.Ptr(time.Duration(durationMuS) * time.Microsecond)

	return media, nil
}

// DeleteMedia deletes media by id.
func (s *Storage) DeleteMedia(ctx context.Context, id int64) error {
	const op = "storage.sqlite.DeleteMedia"

	stmt, err := s.db.Prepare("DELETE FROM library WHERE id = ?")
	if err != nil {
		return fmt.Errorf("%s: %w", op, err)
	}
	defer stmt.Close()

	res, err := stmt.ExecContext(ctx, id)
	if err != nil {
		return fmt.Errorf("%s: %w", op, err)
	}
	affectedRows, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("%s: %w", op, err)
	}
	if affectedRows == 0 {
		return storage.ErrMediaNotFound
	}

	return nil
}
