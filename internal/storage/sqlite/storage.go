package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/GintGld/fizteh-radio/internal/lib/utils/pointers"
	"github.com/GintGld/fizteh-radio/internal/models"
	"github.com/GintGld/fizteh-radio/internal/storage"
	"github.com/mattn/go-sqlite3"
)

type Storage struct {
	db *sql.DB
}

func New(storagePath string) (*Storage, error) {
	const op = "storage.sqlite.New"

	db, err := sql.Open("sqlite3", storagePath)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", op, err)
	}

	return &Storage{db: db}, nil
}

func (s *Storage) Stop() error {
	return s.db.Close()
}

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

	editors := make([]models.Editor, 0)
	var editor models.Editor
	for rows.Next() {
		if err = rows.Scan(&editor.ID, &editor.Login, &editor.PassHash); err != nil {
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
		media.ID = &id
		media.Name = &name
		media.Author = &author
		media.Duration = pointers.Pointer(time.Duration(durationMs) * time.Millisecond)

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

	res, err := stmt.ExecContext(ctx, *media.Name, *media.Author, media.Duration.Milliseconds(), *media.SourceID)
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
		durationMs   int64
	)

	err = row.Scan(&name, &author, &durationMs, &sourceID)
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
	media.Duration = pointers.Pointer(time.Duration(durationMs) * time.Millisecond)

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

// ScheduelCut returns all segments lying in given time interval.
func (s *Storage) ScheduleCut(ctx context.Context, start time.Time, stop time.Time) ([]models.Segment, error) {
	const op = "storage.sqlite.ScheduleCut"

	stmt, err := s.db.Prepare("SELECT id, media_id, start_ms, begin_cut, stop_cut FROM schedule")
	if err != nil {
		return []models.Segment{}, fmt.Errorf("%s: %w", op, err)
	}
	defer stmt.Close()

	rows, err := stmt.QueryContext(ctx)
	if err != nil {
		return []models.Segment{}, fmt.Errorf("%s: %w", op, err)
	}

	segments := make([]models.Segment, 0)
	var (
		segment                               models.Segment
		id, mediaID, startMs, beginMs, stopMs int64
	)
	for rows.Next() {
		if err = rows.Scan(&id, &mediaID, &startMs, &beginMs, &stopMs); err != nil {
			return segments, fmt.Errorf("%s: %w", op, err)
		}
		segment.ID = &id
		segment.MediaID = &mediaID
		segment.Start = pointers.Pointer(time.Unix(startMs/1000, startMs%1000))
		segment.BeginCut = pointers.Pointer(time.Duration(beginMs) * time.Millisecond)
		segment.StopCut = pointers.Pointer(time.Duration(stopMs) * time.Millisecond)

		segments = append(segments, segment)

	}

	return segments, nil
}

// SaveSegment saves segment to schedule.
func (s *Storage) SaveSegment(ctx context.Context, segment models.Segment) (int64, error) {
	const op = "storage.sqlite.SaveSegment"

	if segment.MediaID == nil {
		return 0, fmt.Errorf("%s: media is not defined", op)
	}
	if segment.Start == nil {
		return 0, fmt.Errorf("%s: start is not defined", op)
	}
	if segment.BeginCut == nil {
		return 0, fmt.Errorf("%s: begin cut is not defined", op)
	}
	if segment.StopCut == nil {
		return 0, fmt.Errorf("%s: stop cut is not defined", op)
	}

	stmt, err := s.db.Prepare("INSERT INTO schedule(media_id, start, begin_cut, stop_cut) VALUES(?, ?, ?, ?)")
	if err != nil {
		return 0, fmt.Errorf("%s: %w", op, err)
	}
	defer stmt.Close()

	res, err := stmt.ExecContext(ctx, *segment.MediaID, segment.Start.UnixMilli(), *segment.BeginCut, *segment.StopCut)
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

// Segment returns segment by id.
func (s *Storage) Segment(ctx context.Context, id int64) (models.Segment, error) {
	const op = "storage.sqlite.Segment"

	stmt, err := s.db.Prepare("SELECT media_id, start_ms, begin_cut, stop_cut FROM schedule WHERE id = ?")
	if err != nil {
		return models.Segment{}, fmt.Errorf("%s: %w", op, err)
	}

	var (
		segment                        models.Segment
		mediaID, msec, beginMs, stopMs int64
	)

	row := stmt.QueryRowContext(ctx, id)
	err = row.Scan(&mediaID, &msec, &beginMs, &stopMs)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return models.Segment{}, fmt.Errorf("%s: %w", op, storage.ErrSegmentNotFound)
		}

		return models.Segment{}, fmt.Errorf("%s: %w", op, err)
	}

	segment.ID = &id
	segment.MediaID = &mediaID
	segment.Start = pointers.Pointer(time.Unix(msec/1000, msec%1000))
	segment.BeginCut = pointers.Pointer(time.Duration(beginMs) * time.Millisecond)
	segment.StopCut = pointers.Pointer(time.Duration(stopMs) * time.Millisecond)

	return segment, nil
}

// DeleteSegment deletes segmnt by its period.
func (s *Storage) DeleteSegment(ctx context.Context, period int64) error {
	const op = "storage.sqlite.DeleteSegment"

	stmt, err := s.db.Prepare("DELETE FROM schedule WHERE period = ?")
	if err != nil {
		return fmt.Errorf("%s: %w", op, err)
	}
	defer stmt.Close()

	_, err = stmt.ExecContext(ctx, period)
	if err != nil {
		var sqliteErr sqlite3.Error
		if errors.As(err, &sqliteErr) && sqliteErr.ExtendedCode == sqlite3.ErrConstraintUnique {
			return fmt.Errorf("%s: %w", op, storage.ErrSegmentNotFound)
		}

		return fmt.Errorf("%s: %w", op, err)
	}

	return nil
}
