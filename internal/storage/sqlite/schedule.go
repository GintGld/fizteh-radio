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

// TODO: rename all variables named *Ms to smth else
// because they are nanoseconds

// ScheduelCut returns all segments intersecting given interval.
func (s *Storage) ScheduleCut(ctx context.Context, start time.Time, stop time.Time) ([]models.Segment, error) {
	const op = "storage.sqlite.ScheduleCut"

	// Select segments intersecting diaposon [start, stop]
	stmt, err := s.db.Prepare(`
		SELECT id, media_id, start_ms, begin_cut, stop_cut 
		FROM schedule
		WHERE (
			(
				$1 <= start_ms AND
				$2 >= start_ms
			) OR (
				$1 <= start_ms + (stop_cut - begin_cut)/1000000 AND
				$2 >= start_ms + (stop_cut - begin_cut)/1000000
			)
		)
	`)
	if err != nil {
		return []models.Segment{}, fmt.Errorf("%s: %w", op, err)
	}
	defer stmt.Close()

	rows, err := stmt.QueryContext(ctx, start.UnixMilli(), stop.UnixMilli())
	if err != nil {
		return []models.Segment{}, fmt.Errorf("%s: %w", op, err)
	}

	segments := make([]models.Segment, 0)
	var (
		segment models.Segment
		id, mediaID, startMs,
		beginMs, stopMs int64
	)
	for rows.Next() {
		if err = rows.Scan(&id, &mediaID, &startMs, &beginMs, &stopMs); err != nil {
			return segments, fmt.Errorf("%s: %w", op, err)
		}
		segment.ID = ptr.Ptr(id)
		segment.MediaID = ptr.Ptr(mediaID)
		segment.Start = ptr.Ptr(time.Unix(startMs/1000, startMs%1000))
		segment.BeginCut = ptr.Ptr(time.Duration(beginMs))
		segment.StopCut = ptr.Ptr(time.Duration(stopMs))

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

	stmt, err := s.db.Prepare("INSERT INTO schedule(media_id, start_ms, begin_cut, stop_cut) VALUES(?, ?, ?, ?)")
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
	segment.Start = ptr.Ptr(time.Unix(msec/1000, msec%1000*1000000))
	segment.BeginCut = ptr.Ptr(time.Duration(beginMs))
	segment.StopCut = ptr.Ptr(time.Duration(stopMs))

	return segment, nil
}

// DeleteSegment deletes segmnt by its period.
func (s *Storage) DeleteSegment(ctx context.Context, id int64) error {
	const op = "storage.sqlite.DeleteSegment"

	stmt, err := s.db.Prepare("DELETE FROM schedule WHERE id = ?")
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
		return storage.ErrSegmentNotFound
	}

	return nil
}
