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

// ScheduelCut returns all segments intersecting given interval.
func (s *Storage) ScheduleCut(ctx context.Context, start time.Time, stop time.Time) ([]models.Segment, error) {
	const op = "storage.sqlite.ScheduleCut"

	// Select segments intersecting diaposon [start, stop]
	stmt, err := s.db.Prepare(`
		SELECT id, media_id, start_mus, begin_cut, stop_cut 
		FROM schedule
		WHERE (
			start_mus + (stop_cut - begin_cut) > ?
			AND
			start_mus < ?
		)
		ORDER BY start_mus
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
	defer stmt.Close()

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

// UpdateSegmentTiming updates
// all fields referred to time.
func (s *Storage) UpdateSegmenTiming(ctx context.Context, segment models.Segment) error {
	const op = "Storage.UpdateSegmentiming"

	stmt, err := s.db.Prepare("UPDATE schedule SET start_mus=?, begin_cut=?, stop_cut=? WHERE id=?")
	if err != nil {
		return fmt.Errorf("%s: %w", op, err)
	}
	defer stmt.Close()

	if _, err := stmt.ExecContext(ctx,
		segment.Start.UnixMicro(),
		segment.BeginCut.Microseconds(),
		segment.StopCut.Microseconds(),
		*segment.ID,
	); err != nil {
		return fmt.Errorf("%s: %w", op, err)
	}

	return nil
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
		var sqliteErr sqlite3.Error
		if errors.As(err, &sqliteErr) && sqliteErr.ExtendedCode == sqlite3.ErrConstraintUnique {
			return storage.ErrSegmentAlreadyProtected
		}
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

// NewLive registers new segment.
func (s *Storage) NewLive(ctx context.Context, live models.Live) (int64, error) {
	const op = "storage.NewLive"

	stmt, err := s.db.Prepare("INSERT INTO live_stream(name, start, stop, delay, offset) VALUES(?, ?, ?, ?, ?)")
	if err != nil {
		return 0, fmt.Errorf("%s: %w", op, err)
	}
	defer stmt.Close()

	res, err := stmt.ExecContext(ctx,
		live.Name,
		live.Start.UnixMicro(),
		live.Stop.UnixMicro(),
		live.Delay.Microseconds(),
		live.Offset.Microseconds(),
	)
	if err != nil {
		return 0, fmt.Errorf("%s: %w", op, err)
	}

	id, err := res.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("%s: %w", op, err)
	}

	return id, nil
}

// Set time when live stopped.
func (s *Storage) SetLiveStop(ctx context.Context, live models.Live) error {
	const op = "Storage.SetLiveStop"

	stmt, err := s.db.Prepare("UPDATE live_stream SET stop=? WHERE id=?")
	if err != nil {
		return fmt.Errorf("%s: %w", op, err)
	}
	defer stmt.Close()

	if _, err := stmt.ExecContext(ctx, live.Stop.UnixMicro(), live.ID); err != nil {
		return fmt.Errorf("%s: %w", op, err)
	}

	return nil
}

// GetLive returns all registered live streams
// stopping after given time point.
func (s *Storage) GetLive(ctx context.Context, start time.Time) ([]models.Live, error) {
	const op = "Storage.GetLive"

	stmt, err := s.db.Prepare("SELECT id, name, start, stop, delay, offset FROM live_stream WHERE stop >= ?")
	if err != nil {
		return []models.Live{}, fmt.Errorf("%s: %w", op, err)
	}
	defer stmt.Close()

	rows, err := stmt.QueryContext(ctx, start.UnixMicro())
	if err != nil {
		return []models.Live{}, fmt.Errorf("%s: %w", op, err)
	}

	lives := make([]models.Live, 0)
	live := models.Live{}
	var (
		startMs, stopMs, delayMs, offsetMs int64
	)

	for rows.Next() {
		if err := rows.Scan(&live.ID, &live.Name, &startMs, &stopMs, &delayMs, &offsetMs); err != nil {
			return []models.Live{}, fmt.Errorf("%s: %w", op, err)
		}
		live.Start = time.Unix(startMs/1000000, startMs%1000000*1000)
		live.Stop = time.Unix(stopMs/1000000, stopMs%1000000*1000)
		live.Delay = time.Duration(delayMs * 1000)
		live.Offset = time.Duration(offsetMs * 1000)
		lives = append(lives, live)
	}

	return lives, nil
}

// LiveId returns corresponding live id
// for segment. If not exists. returns 0.
func (s *Storage) LiveId(ctx context.Context, id int64) (int64, error) {
	const op = "storage.sqlite.LiveId"

	stmt, err := s.db.Prepare("SELECT live_id FROM schedule_live WHERE segment_id=?")
	if err != nil {
		return 0, fmt.Errorf("%s: %w", op, err)
	}
	defer stmt.Close()

	row := stmt.QueryRowContext(ctx, id)

	var res int64

	if err := row.Scan(&res); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return 0, nil
		}
		return 0, fmt.Errorf("%s: %w", op, err)
	}

	return res, nil
}

// AttachLive registers segment as live chapter.
func (s *Storage) AttachLive(ctx context.Context, segmId int64, liveId int64) error {
	const op = "Storage.AttachLive"

	stmt, err := s.db.PrepareContext(ctx, `
		INSERT INTO schedule_live(segment_id, live_id)
		VALUES(?, ?)
	`)
	if err != nil {
		return fmt.Errorf("%s: %w", op, err)
	}
	defer stmt.Close()

	if _, err := stmt.Exec(segmId, liveId); err != nil {
		var sqliteErr sqlite3.Error
		if errors.As(err, &sqliteErr) && sqliteErr.ExtendedCode == sqlite3.ErrConstraintUnique {
			return storage.ErrSegmentAlreadyAttachedToLive
		}
		return fmt.Errorf("%s: %w", op, err)
	}

	return nil
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
