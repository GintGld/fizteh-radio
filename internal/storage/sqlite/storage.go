package mysql

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

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

// SaveUser saves user to db.
func (s *Storage) SaveUser(ctx context.Context, login string, passHash []byte) (int64, error) {
	const op = "storage.sqlite.SaveUser"

	stmt, err := s.db.Prepare("INSERT INTO editors(login, pass_hash) VALUES(?, ?)")
	if err != nil {
		return 0, fmt.Errorf("%s: %w", op, err)
	}

	res, err := stmt.ExecContext(ctx, login, passHash)
	if err != nil {
		var sqliteErr sqlite3.Error
		if errors.As(err, &sqliteErr) && sqliteErr.ExtendedCode == sqlite3.ErrConstraintUnique {
			return 0, fmt.Errorf("%s: %w", op, storage.ErrUserExists)
		}

		return 0, fmt.Errorf("%s: %w", op, err)
	}

	id, err := res.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("%s: %w", op, err)
	}

	return id, nil
}

// User returns user by login.
func (s *Storage) User(ctx context.Context, id int64) (models.User, error) {
	const op = "storage.sqlite.User"

	stmt, err := s.db.Prepare("SELECT id, login, pass_hash FROM editors WHERE id = ?")
	if err != nil {
		return models.User{}, fmt.Errorf("%s: %w", op, err)
	}

	row := stmt.QueryRowContext(ctx, id)

	var user models.User
	err = row.Scan(&user.ID, &user.Login, &user.PassHash)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return models.User{}, fmt.Errorf("%s: %w", op, storage.ErrUserNotFound)
		}

		return models.User{}, fmt.Errorf("%s: %w", op, err)
	}

	return user, nil
}

// DeleteUser deletes user
func (s *Storage) DeleteUser(ctx context.Context, id int64) error {
	const op = "storage.sqlite.DeleteUser"

	stmt, err := s.db.Prepare("DELETE FROM editors WHERE id = ?")
	if err != nil {
		return fmt.Errorf("%s: %w", op, err)
	}

	_, err = stmt.ExecContext(ctx, id)
	if err != nil {
		var sqliteErr sqlite3.Error
		if errors.As(err, &sqliteErr) && sqliteErr.ExtendedCode == sqlite3.ErrConstraintUnique {
			return fmt.Errorf("%s: %w", op, storage.ErrUserExists)
		}

		return fmt.Errorf("%s: %w", op, err)
	}

	return nil
}

// SaveMedia saves necessary information
// about media file to db.
func (s *Storage) SaveMedia(ctx context.Context, name string, author string, duration time.Duration) (int64, error) {
	const op = "storage.sqlite.SaveMedia"

	stmt, err := s.db.Prepare("INSERT INTO library(name, author, duration) VALUES(?, ?, ?)")
	if err != nil {
		return 0, fmt.Errorf("%s: %w", op, err)
	}

	res, err := stmt.ExecContext(ctx, name, author, (int64)(duration))
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

// Media return media file by id
func (s *Storage) Media(ctx context.Context, id int64) (models.Media, error) {
	const op = "storage.sqlite.Media"

	stmt, err := s.db.Prepare("SELECT id, name, author, duration FROM editors WHERE id = ?")
	if err != nil {
		return models.Media{}, fmt.Errorf("%s: %w", op, err)
	}

	row := stmt.QueryRowContext(ctx, id)

	var media models.Media
	var durationInt int64

	err = row.Scan(&media.ID, &media.Name, &media.Author, &durationInt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return models.Media{}, fmt.Errorf("%s: %w", op, storage.ErrMediaNotFound)
		}

		return models.Media{}, fmt.Errorf("%s: %w", op, err)
	}

	media.Duration = time.Duration(durationInt)

	return media, nil
}

// DeleteMedia deletes media by id
func (s *Storage) DeleteMedia(ctx context.Context, id int64) error {
	const op = "storage.sqlite.DeleteMedia"

	stmt, err := s.db.Prepare("DELETE FROM library WHERE id = ?")
	if err != nil {
		return fmt.Errorf("%s: %w", op, err)
	}

	_, err = stmt.ExecContext(ctx, id)
	if err != nil {
		var sqliteErr sqlite3.Error
		if errors.As(err, &sqliteErr) && sqliteErr.ExtendedCode == sqlite3.ErrConstraintUnique {
			return fmt.Errorf("%s: %w", op, storage.ErrUserExists)
		}

		return fmt.Errorf("%s: %w", op, err)
	}

	return nil
}

// SaveSegment
func (s *Storage) SaveSegment(ctx context.Context, mediaID int64, utc_time time.Time, begin time.Duration, end time.Duration) (int64, error) {
	const op = "storage.sqlite.SaveSegment"

	utc_begin := utc_time.Unix()
	utc_end := utc_time.Unix() + (int64)(end) - (int64)(begin)

	// TODO: transaction settings in BeginTx
	// TODO: move each execution call for corresponding function

	// Start transaction
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, fmt.Errorf("%s: %w", op, err)
	}
	defer tx.Rollback()

	// Check for intersections with already placed segments
	stmt, err := s.db.Prepare(`
		DECLARE @A AS INTEGER;
		DECLARE @B AS INTEGER;
		SET @A = ?;
		SET @B = ?;
		SELECT COUNT(*) FROM schedule WHERE (@A < utc_time AND utc_time < @B) OR (@A < utc_time + end - begin AND utc_time + end - begin  < @B)`)
	if err != nil {
		return 0, fmt.Errorf("%s: %w", op, err)
	}

	res, err := stmt.ExecContext(ctx, utc_begin, utc_end)
	if err != nil {
		return 0, fmt.Errorf("%s: %w", op, err)
	}
	count, err := res.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("%s: %w", op, err)
	}
	if count != 0 {
		return 0, fmt.Errorf("%s: %w", op, storage.ErrSegmentIntersect)
	}

	// TODO: shift segments placed after incerting one
	stmt, err = s.db.Prepare("UPDATE schedule SET period = period + 1 WHERE utc_time > ?")
	if err != nil {
		return 0, fmt.Errorf("%s: %w", op, err)
	}

	res, err = stmt.ExecContext(ctx, utc_end)
	if err != nil {
		return 0, fmt.Errorf("%s: %w", op, err)
	}

	// Insert new segment
	stmt, err = s.db.Prepare("INSERT INTO schedule(media_id, period, utc_time, begin, end) VALUES(?, ?, ?, ?, ?)")
	if err != nil {
		return 0, fmt.Errorf("%s: %w", op, err)
	}

	res, err = stmt.ExecContext(ctx, mediaID, (int64)(begin), (int64)(end))
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

	if err = tx.Commit(); err != nil {
		return 0, fmt.Errorf("%s: %w", op, err)
	}

	return id, nil
}

// TODO: Segment()

// TODO: DeleteSegment()

func (s *Storage) checkSegmentIntersection(ctx context.Context, utc_time time.Time, begin time.Duration, end time.Duration) error {
	// TODO
	panic("not implemented")
}

func (s *Storage) getPeriodForNewSegment(ctx context.Context, utc_time time.Time, begin time.Duration, end time.Duration) (int64, error) {
	// TODO
	panic("not implemented")
}

func (s *Storage) shiftPeriods(ctx context.Context, startPeriod int64) error {
	// TODO
	panic("not implemented")
}

func (s *Storage) insertNewSegment(ctx context.Context, mediaID int64, utc_time time.Time, begin time.Duration, end time.Duration) error {
	// TODO
	panic("not implemented")
}
