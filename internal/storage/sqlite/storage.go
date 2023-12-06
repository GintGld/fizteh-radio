package sqlite

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
	defer stmt.Close()

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
	defer stmt.Close()

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
	defer stmt.Close()

	_, err = stmt.ExecContext(ctx, id)
	if err != nil {
		var sqliteErr sqlite3.Error
		if errors.As(err, &sqliteErr) && sqliteErr.ExtendedCode == sqlite3.ErrConstraintUnique {
			return fmt.Errorf("%s: %w", op, storage.ErrUserNotFound)
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
	defer stmt.Close()

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
	defer stmt.Close()

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
	defer stmt.Close()

	_, err = stmt.ExecContext(ctx, id)
	if err != nil {
		var sqliteErr sqlite3.Error
		if errors.As(err, &sqliteErr) && sqliteErr.ExtendedCode == sqlite3.ErrConstraintUnique {
			return fmt.Errorf("%s: %w", op, storage.ErrMediaNotFound)
		}

		return fmt.Errorf("%s: %w", op, err)
	}

	return nil
}

// SaveSegment saves segment to schedule after
// checking for no intersections with already
// placed segments
func (s *Storage) SaveSegment(ctx context.Context, mediaID int64, start time.Time, beginCut time.Duration, stopCut time.Duration) (int64, error) {
	const op = "storage.sqlite.SaveSegment"

	// absolute time for segment end
	end := start.Add(stopCut - beginCut)

	// TODO: transaction settings in BeginTx

	// Start transaction
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, fmt.Errorf("%s: %w", op, err)
	}
	defer tx.Rollback()

	// check for intersections with already placed segments
	if err := s.checkSegmentIntersection(ctx, start, end); err != nil {
		return 0, fmt.Errorf("%s: %w", op, storage.ErrSegmentIntersect)
	}

	// get period that will given to new segment
	insertPeriod, err := s.getPeriodForNewSegment(ctx, start)
	if err != nil {
		return 0, fmt.Errorf("%s: %w", op, err)
	}

	// shift period for segments placed after inserting one
	if err := s.shiftPeriods(ctx, insertPeriod); err != nil {
		return 0, fmt.Errorf("%s: %w", op, err)
	}

	// insert new segment
	id, err := s.insertNewSegment(ctx, mediaID, insertPeriod, start, beginCut, stopCut)
	if err != nil {
		return 0, fmt.Errorf("%s: %w", op, err)
	}

	// commit successful transaction
	if err = tx.Commit(); err != nil {
		return 0, fmt.Errorf("%s: %w", op, err)
	}

	return id, nil
}

// Segment returns segment by period
func (s *Storage) Segment(ctx context.Context, period int64) (models.Segment, error) {
	const op = "storage.sqlite.Segment"

	segm := models.Segment{}
	var msec int64

	stmt, err := s.db.Prepare("SELECT id, media_id, period, start_ms, begin_cut, end_cut FROM schedule WHERE period = ?")
	if err != nil {
		return models.Segment{}, fmt.Errorf("%s: %w", op, err)
	}

	row := stmt.QueryRowContext(ctx, period)
	err = row.Scan(&segm.ID, &segm.MediaID, &segm.Period, &msec, &segm.BeginCut, &segm.StopCut)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return models.Segment{}, fmt.Errorf("%s: %w", op, storage.ErrSegmentNotFound)
		}

		return models.Segment{}, fmt.Errorf("%s: %w", op, err)
	}

	segm.Start = time.Unix(msec/1000, msec%1000)

	return segm, nil
}

// DeleteSegment deletes segmnt by its period
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

// checkSegmentIntersection check if new
// segment can be placed in the schedule
// without intersections with already placed ones
func (s *Storage) checkSegmentIntersection(ctx context.Context, start, end time.Time) error {
	const op = "storage.sqlite.checkSegmentIntersection"

	stmt, err := s.db.Prepare(`
		DECLARE @start AS REAL = ?;
		DECLARE @end   AS REAL = ?;
		SELECT COUNT(*) FROM schedule WHERE
			(
				@start < start_ms AND
				@end   > start_ms
			)
			OR
			(
				@start < start_ms + end_cut - begin_cut AND
				@end   > start_ms + end_cut - begin_cut
			)
	`)
	if err != nil {
		return fmt.Errorf("%s: %w", op, err)
	}
	defer stmt.Close()

	res, err := stmt.ExecContext(ctx, start.UnixMilli(), end.UnixMilli())
	if err != nil {
		return fmt.Errorf("%s: %w", op, err)
	}
	count, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("%s: %w", op, err)
	}
	if count != 0 {
		return fmt.Errorf("%s: %w", op, storage.ErrSegmentIntersect)
	}

	return nil
}

// getPeriodForNewSegment returns period
// that will have new segment
func (s *Storage) getPeriodForNewSegment(ctx context.Context, begin time.Time) (int64, error) {
	const op = "storage.sqlite.getPeriodForNewSegment"

	stmt, err := s.db.Prepare(`
		SELECT MAX(period) FROM schedule WHERE
			start_ms < ?
	`)
	if err != nil {
		return 0, fmt.Errorf("%s: %w", op, err)
	}
	defer stmt.Close()

	res := stmt.QueryRowContext(ctx, begin.UnixMilli())

	var maxPeriod sql.NullInt64
	err = res.Scan(&maxPeriod)

	if err != nil {
		return 0, fmt.Errorf("%s: %w", op, err)
	}

	if maxPeriod.Valid {
		return maxPeriod.Int64 + 1, nil
	} else {
		return 1, nil
	}
}

// shiftPeriods increases period by 1
// in segments that are placed
// chronologically after new segment
// to free space for new segment
func (s *Storage) shiftPeriods(ctx context.Context, insertPeriod int64) error {
	const op = "storage.sqlite.shiftPeriods"

	stmt, err := s.db.Prepare("UPDATE schedule SET period = period + 1 WHERE period > ?")
	if err != nil {
		return fmt.Errorf("%s: %w", op, err)
	}
	defer stmt.Close()

	_, err = stmt.ExecContext(ctx, insertPeriod)
	if err != nil {
		return fmt.Errorf("%s: %w", op, err)
	}

	return nil
}

// insertNewSegment inserts new segment
// into schedule
func (s *Storage) insertNewSegment(ctx context.Context, mediaID int64, insertPeriod int64, start time.Time, begin time.Duration, end time.Duration) (int64, error) {
	const op = "storage.sqlite.insertNewSegment"

	stmt, err := s.db.Prepare("INSERT INTO schedule(media_id, period, start_ms, begin_cut, end_cut) VALUES(?, ?, ?, ?, ?)")
	if err != nil {
		return 0, fmt.Errorf("%s: %w", op, err)
	}
	defer stmt.Close()

	res, err := stmt.ExecContext(ctx, mediaID, insertPeriod, start.UnixMilli(), begin.Milliseconds(), end.Milliseconds())
	if err != nil {
		var sqliteErr sqlite3.Error
		if errors.As(err, &sqliteErr) && sqliteErr.ExtendedCode == sqlite3.ErrConstraintUnique {
			return 0, fmt.Errorf("%s: %w", op, storage.ErrSegmentExists)
		}

		return 0, fmt.Errorf("%s: %w", op, err)
	}

	id, err := res.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("%s: %w", op, err)
	}

	return id, nil
}
