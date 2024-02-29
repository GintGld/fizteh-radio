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

/*
// All time values are stored in microseconds.
// It is motivated by dash precision.
*/

// TODO: SaveSegment should take segmetns ...models.Segment

// ScheduelCut returns all segments intersecting given interval.
func (s *Storage) ScheduleCut(ctx context.Context, start time.Time, stop time.Time) ([]models.Segment, error) {
	const op = "storage.sqlite.ScheduleCut"

	// Select segments intersecting diaposon [start, stop]
	stmt, err := s.db.Prepare(`
		SELECT id, media_id, start_mus, begin_cut, stop_cut 
		FROM schedule
		WHERE (
			start_mus BETWEEN $1 AND $2
			OR
			start_mus + (stop_cut - begin_cut) BETWEEN $1 AND $2
		)
	`)
	if err != nil {
		return []models.Segment{}, fmt.Errorf("%s: %w", op, err)
	}
	defer stmt.Close()

	rows, err := stmt.QueryContext(ctx, start.UnixMicro(), stop.UnixMicro())
	if err != nil {
		return []models.Segment{}, fmt.Errorf("%s: %w", op, err)
	}

	segments := make([]models.Segment, 0)
	var (
		segment models.Segment
		id, mediaID, startMs,
		beginMuS, stopMuS int64
	)
	for rows.Next() {
		if err = rows.Scan(&id, &mediaID, &startMs, &beginMuS, &stopMuS); err != nil {
			return segments, fmt.Errorf("%s: %w", op, err)
		}
		segment.ID = ptr.Ptr(id)
		segment.MediaID = ptr.Ptr(mediaID)
		segment.Start = ptr.Ptr(time.Unix(startMs/1000000, startMs%1000000*1000))
		segment.BeginCut = ptr.Ptr(time.Duration(beginMuS) * time.Microsecond)
		segment.StopCut = ptr.Ptr(time.Duration(stopMuS) * time.Microsecond)

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

	stmt, err := s.db.Prepare("INSERT INTO schedule(media_id, start_mus, begin_cut, stop_cut) VALUES(?, ?, ?, ?)")
	if err != nil {
		return 0, fmt.Errorf("%s: %w", op, err)
	}
	defer stmt.Close()

	res, err := stmt.ExecContext(
		ctx,
		*segment.MediaID,
		segment.Start.UnixMicro(),
		segment.BeginCut.Microseconds(),
		segment.StopCut.Microseconds(),
	)
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

	stmt, err := s.db.Prepare("SELECT media_id, start_mus, begin_cut, stop_cut FROM schedule WHERE id = ?")
	if err != nil {
		return models.Segment{}, fmt.Errorf("%s: %w", op, err)
	}

	var (
		segment                          models.Segment
		mediaID, msec, beginMuS, stopMuS int64
	)

	row := stmt.QueryRowContext(ctx, id)
	err = row.Scan(&mediaID, &msec, &beginMuS, &stopMuS)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return models.Segment{}, fmt.Errorf("%s: %w", op, storage.ErrSegmentNotFound)
		}

		return models.Segment{}, fmt.Errorf("%s: %w", op, err)
	}

	segment.ID = &id
	segment.MediaID = &mediaID
	segment.Start = ptr.Ptr(time.Unix(msec/1000000, msec%1000000*1000))
	segment.BeginCut = ptr.Ptr(time.Duration(beginMuS * 1000))
	segment.StopCut = ptr.Ptr(time.Duration(stopMuS * 1000))

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

// ProtectSegment set protect label for segment,
// which mean it won't be deleted by ClearSchedule.
func (s *Storage) ProtectSegment(ctx context.Context, id int64) error {
	const op = "storage.sqlite.ProtectSegment"

	stmt, err := s.db.Prepare("INSERT INTO schedule_protect(segment_id) VALUES(?)")
	if err != nil {
		return fmt.Errorf("%s: %w", op, err)
	}
	defer stmt.Close()

	if _, err := stmt.ExecContext(ctx, id); err != nil {
		return fmt.Errorf("%s: %w", op, err)
	}

	return nil
}

// IsSegmentProtected returns true, if
// segment is protected from ClearSchedule.
func (s *Storage) IsSegmentProtected(ctx context.Context, id int64) (bool, error) {
	const op = "storage.sqlite.IsSegmentProtected"

	stmt, err := s.db.Prepare("SELECT EXISTS (SELECT id FROM schedule_protect WHERE segment_id=?)")
	if err != nil {
		return false, fmt.Errorf("%s: %w", op, err)
	}
	defer stmt.Close()

	row := stmt.QueryRowContext(ctx, id)

	var res int8

	if err := row.Scan(&res); err != nil {
		return false, fmt.Errorf("%s: %w", op, err)
	}

	return res == 1, nil
}

// ClearSchedule clears schedule from given timestamp.
// Doesn't delete protected segments.
func (s *Storage) ClearSchedule(ctx context.Context, from time.Time) error {
	const op = "storage.sqlite.ClearSchedule"

	stmt, err := s.db.Prepare(`
		DELETE FROM schedule
		WHERE start_mus + (stop_cut - begin_cut) >= ?
		AND
		NOT EXISTS (SELECT * FROM schedule_protect WHERE schedule_protect.segment_id = schedule.id)
	`)
	if err != nil {
		return fmt.Errorf("%s: %w", op, err)
	}
	defer stmt.Close()

	if _, err := stmt.ExecContext(ctx, from.UnixMicro()); err != nil {
		return fmt.Errorf("%s: %w", op, err)
	}

	return nil
}
