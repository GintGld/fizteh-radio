package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"slices"
	"time"

	"github.com/GintGld/fizteh-radio/internal/lib/logger/sl"
	chans "github.com/GintGld/fizteh-radio/internal/lib/utils/channels"
	"github.com/GintGld/fizteh-radio/internal/models"
	"github.com/GintGld/fizteh-radio/internal/service"
	"github.com/GintGld/fizteh-radio/internal/storage"
)

type Schedule struct {
	log          *slog.Logger
	schStorage   ScheduleStorage
	mediaStorage MediaStorage

	allSegmentsChan       chan<- models.Segment
	protectedSegmentsChan chan<- struct{}
}

type ScheduleStorage interface {
	// Global schedule
	ScheduleCut(ctx context.Context, start time.Time, stop time.Time) ([]models.Segment, error)
	ClearSchedule(ctx context.Context, stamp time.Time) error
	// Single segment
	SaveSegment(ctx context.Context, segment models.Segment) (int64, error)
	UpdateSegmenTiming(ctx context.Context, segment models.Segment) error
	Segment(ctx context.Context, period int64) (models.Segment, error)
	DeleteSegment(ctx context.Context, period int64) error
	// Segment protection
	ProtectSegment(ctx context.Context, id int64) error
	IsSegmentProtected(ctx context.Context, id int64) (bool, error)
	// Segment live
	GetLive(ctx context.Context, start time.Time) ([]models.Live, error)
	NewLive(ctx context.Context, live models.Live) (int64, error)
	SetLiveStop(ctx context.Context, live models.Live) error
	LiveId(ctx context.Context, id int64) (int64, error)
	AttachLive(ctx context.Context, segmId int64, liveId int64) error
}

type MediaStorage interface {
	Media(ctx context.Context, id int64) (models.Media, error)
}

func New(
	log *slog.Logger,
	schStorage ScheduleStorage,
	mediaStorage MediaStorage,
	allSegmentsChan chan<- models.Segment,
	protectedSegmentsChan chan<- struct{},
) *Schedule {
	return &Schedule{
		log:                   log,
		schStorage:            schStorage,
		mediaStorage:          mediaStorage,
		allSegmentsChan:       allSegmentsChan,
		protectedSegmentsChan: protectedSegmentsChan,
	}
}

// ScheduleCut returns segments intersecting given interval
func (s *Schedule) ScheduleCut(ctx context.Context, start time.Time, stop time.Time) ([]models.Segment, error) {
	const op = "Schedule.ScheduleCut"

	log := s.log.With(
		slog.String("op", op),
		slog.String("editorname", models.RootLogin),
	)

	segments, err := s.schStorage.ScheduleCut(ctx, start, stop)
	if err != nil {
		if errors.Is(err, storage.ErrContextCancelled) {
			log.Error("schStorage.ScheduleCut timeout exceeded")
			return []models.Segment{}, service.ErrTimeout
		}
		log.Error("failed to get schedule cut", sl.Err(err))
		return []models.Segment{}, fmt.Errorf("%s: %w", op, err)
	}

	for i, segment := range segments {
		if isProt, err := s.schStorage.IsSegmentProtected(ctx, *segment.ID); err != nil {
			if errors.Is(err, storage.ErrContextCancelled) {
				log.Error("schStorage.IsSegmentProtected timeout exceeded")
				return []models.Segment{}, service.ErrTimeout
			}
			log.Error("fialed to check segment protection", slog.Int64("id", *segment.ID), sl.Err(err))
			return []models.Segment{}, fmt.Errorf("%s: %w", op, err)
		} else {
			segments[i].Protected = isProt
		}
		if liveId, err := s.schStorage.LiveId(ctx, *segment.ID); err != nil {
			if errors.Is(err, storage.ErrContextCancelled) {
				log.Error("schStorage.LiveId timeout exceeded")
				return []models.Segment{}, service.ErrTimeout
			}
			log.Error("failed to get live id", slog.Int64("id", *segment.ID), sl.Err(err))
			return []models.Segment{}, fmt.Errorf("%s: %w", op, err)
		} else {
			segments[i].LiveId = liveId
		}
	}

	return segments, nil
}

// Lives returns all registered live streams
// stopping after given time point.
func (s *Schedule) Lives(ctx context.Context, start time.Time) ([]models.Live, error) {
	const op = "Schedule.Lives"

	log := s.log.With(
		slog.String("op", op),
		slog.String("editorname", models.RootLogin),
	)

	res, err := s.schStorage.GetLive(ctx, start)
	if err != nil {
		if errors.Is(err, storage.ErrContextCancelled) {
			log.Error("schStorage.GetLive timeout exceeded")
			return []models.Live{}, service.ErrTimeout
		}
		log.Error("failed to get lives", sl.Err(err))
		return []models.Live{}, fmt.Errorf("%s: %w", op, err)
	}

	return res, nil
}

// NewLive register new live stream.
func (s *Schedule) NewLive(ctx context.Context, live models.Live) (int64, error) {
	const op = "Schedule.NewLive"

	log := s.log.With(
		slog.String("op", op),
		slog.String("editorname", models.RootLogin),
	)

	id, err := s.schStorage.NewLive(ctx, live)
	if err != nil {
		if errors.Is(err, storage.ErrContextCancelled) {
			log.Error("schStorage.NewLive timeout exceeded")
			return 0, service.ErrTimeout
		}
		log.Error("failed to register live", sl.Err(err))
		return 0, fmt.Errorf("%s: %w", op, err)
	}

	return id, nil
}

func (s *Schedule) SetLiveStop(ctx context.Context, live models.Live) error {
	const op = "Schedule.SetLiveStop"

	log := s.log.With(
		slog.String("op", op),
		slog.String("editorname", models.RootLogin),
	)

	if err := s.schStorage.SetLiveStop(ctx, live); err != nil {
		if errors.Is(err, storage.ErrContextCancelled) {
			log.Error("schStorage.SetLiveStop timeout exceeded")
			return service.ErrTimeout
		}
		log.Error("failed to set live stop", slog.Int64("id", live.ID), sl.Err(err))
		return fmt.Errorf("%s: %w", op, err)
	}

	return nil
}

// NewSegment registers new segment in schedule
// if media for segment does not exists returns error.
func (s *Schedule) NewSegment(ctx context.Context, segment models.Segment) (int64, error) {
	const op = "Schedule.NewSegment"

	log := s.log.With(
		slog.String("op", op),
		slog.String("editorname", models.RootLogin),
	)

	// Check cut correctness
	// for non-live segments.
	if *segment.MediaID != 0 {
		if segment.LiveId != 0 {
			log.Warn("live segment can't have media id", slog.Int64("id", *segment.MediaID))
		}

		media, err := s.mediaStorage.Media(ctx, *segment.MediaID)
		if err != nil {
			if errors.Is(err, storage.ErrMediaNotFound) {
				log.Warn("media not found", slog.Int64("id", *segment.MediaID))
				return 0, service.ErrMediaNotFound
			}
			log.Error("failed to get media", slog.Int64("id", *segment.MediaID), sl.Err(err))
			return 0, fmt.Errorf("%s: %w", op, err)
		}

		if *media.Duration < *segment.StopCut ||
			*media.Duration < *segment.BeginCut ||
			*segment.BeginCut < 0 ||
			*segment.StopCut < 0 {
			log.Warn(
				"invalid cut (out of bounds)",
				slog.Int64("beginCut", segment.BeginCut.Microseconds()),
				slog.Int64("stopCut", segment.StopCut.Microseconds()),
			)
			return 0, service.ErrCutOutOfBounds
		}
		if *segment.BeginCut > *segment.StopCut {
			log.Warn(
				"invalid cut (start after stop)",
				slog.Int64("beginCut", segment.BeginCut.Microseconds()),
				slog.Int64("stopCut", segment.StopCut.Microseconds()),
			)
			return 0, service.ErrBeginAfterStop
		}
	}

	res, err := s.ScheduleCut(ctx, *segment.Start, segment.End())
	if err != nil {
		if errors.Is(err, service.ErrTimeout) {
			log.Error("schStorage.SetLiveStop timeout exceeded")
			return 0, service.ErrTimeout
		}
		log.Error("failed to get schedule cut")
		return 0, fmt.Errorf("%s: %w", op, err)
	}

	// If new segment is not protected
	// any intersection causes error.
	if !segment.Protected {
		if len(res) > 0 {
			log.Warn("new not prot. segm has intersection(s)", slog.Any("res", res), slog.Time("start", *segment.Start), slog.Time("end", segment.End()))
			return 0, service.ErrSegmentIntersection
		}
		chans.Send(s.allSegmentsChan, segment)

		id, err := s.schStorage.SaveSegment(ctx, segment)
		if err != nil {
			if errors.Is(err, storage.ErrContextCancelled) {
				log.Error("schStorage.SaveSegment timeout exceeded")
				return 0, service.ErrTimeout
			}
			log.Error("failed to save segment", sl.Err(err))
			return 0, fmt.Errorf("%s: %w", op, err)
		}

		log.Debug("not prot.", slog.Int64("id", id))

		return id, nil
	}

	// If new segment is protected and
	// intersects another protected segment
	// returns error, since can't resolve intersection.
	if j := slices.IndexFunc(res, func(s models.Segment) bool { return s.Protected }); j != -1 {
		log.Warn(
			"detected intersection of protected segments",
			slog.String("found (start-stop)", fmt.Sprintf("%s - %s", res[j].Start.String(), res[j].End().String())),
			slog.String("new (start-stop)", fmt.Sprintf("%s - %s", segment.Start.String(), segment.End().String())),
		)
		// FIXME handle this errors every where.
		return 0, service.ErrSegmentIntersection
	}

	// All intersected segments are
	// not protected. Delete them all.
	for _, segm := range res {
		if segm.End() == *segment.Start || *segm.Start == segm.End() {
			continue
		}
		if err := s.DeleteSegment(ctx, *segm.ID); err != nil {
			if errors.Is(err, service.ErrSegmentNotFound) {
				log.Warn("did not found segment to delete", slog.Int64("segmId", *segm.ID))
				continue
			}
			if errors.Is(err, storage.ErrContextCancelled) {
				log.Error("schStorage.DeleteSegment timeout exceeded")
				return 0, service.ErrTimeout
			}
			log.Error("failed to delete segment", slog.Int64("segmId", *segm.ID), sl.Err(err))
			return 0, fmt.Errorf("%s: %w", op, err)
		}
	}

	log.Debug("saving segment")

	// Create segment.
	id, err := s.schStorage.SaveSegment(ctx, segment)
	if err != nil {
		if errors.Is(err, storage.ErrContextCancelled) {
			log.Error("schStorage.SaveSegment timeout exceeded")
			return 0, service.ErrTimeout
		}
		log.Error("failed to save segment", sl.Err(err))
		return 0, fmt.Errorf("%s: %w", op, err)
	}

	log.Debug("saved, protecting", slog.Int64("id", id))

	// Set segment protection.
	if err := s.schStorage.ProtectSegment(ctx, id); err != nil {
		if errors.Is(err, storage.ErrSegmentAlreadyProtected) {
			log.Warn("segment already protected", slog.Int64("id", id))
		} else {
			if errors.Is(err, storage.ErrContextCancelled) {
				log.Error("schStorage.ProtectSegment timeout exceeded")
				return 0, service.ErrTimeout
			}
			log.Error("failed to set segment protection", sl.Err(err))
			return 0, fmt.Errorf("%s: %w", op, err)
		}
	}

	log.Debug("protected")

	if segment.LiveId != 0 {
		log.Debug("segment is live, attaching")
		if err := s.schStorage.AttachLive(ctx, id, segment.LiveId); err != nil {
			if errors.Is(err, storage.ErrContextCancelled) {
				log.Error("schStorage.AttachLive timeout exceeded")
				return 0, service.ErrTimeout
			}
			log.Error("failed to attach segment to live", slog.Int64("id", id), slog.Int64("liveId", segment.LiveId), sl.Err(err))
			return 0, fmt.Errorf("%s: %w", op, err)
		}
	}

	chans.Notify(s.protectedSegmentsChan)
	chans.Send(s.allSegmentsChan, segment)

	return id, nil
}

// UpdateSegmentTiming updates
// all fields referred to time.
func (s *Schedule) UpdateSegmentTiming(ctx context.Context, segment models.Segment) error {
	const op = "Schedule.UpdateSegmentTiming"

	log := s.log.With(
		slog.String("op", op),
		slog.String("editorname", models.RootLogin),
	)

	if err := s.schStorage.UpdateSegmenTiming(ctx, segment); err != nil {
		if errors.Is(err, storage.ErrContextCancelled) {
			log.Error("schStorage.UpdateSegmenTiming timeout exceeded")
			return service.ErrTimeout
		}
		log.Error("failed to update segment timing", slog.Int64("id", *segment.ID), sl.Err(err))
		return fmt.Errorf("%s: %w", op, err)
	}

	res, err := s.ScheduleCut(ctx, *segment.Start, segment.End())
	if err != nil {
		if errors.Is(err, service.ErrTimeout) {
			log.Error("schStorage.ScheduleCut timeout exceeded")
			return service.ErrTimeout
		}
		log.Error("failed to get schedule cut", sl.Err(err))
		return fmt.Errorf("%s: %w", op, err)
	}
	if !segment.Protected && len(res) > 0 {
		log.Warn("new not prot. segm has intersection(s)", slog.Any("res", res), slog.Time("start", *segment.Start), slog.Time("end", segment.End()))
	}
	if j := slices.IndexFunc(res, func(s models.Segment) bool { return s.Protected && *s.ID != *segment.ID }); j != -1 {
		log.Warn(
			"detected intersection of protected segments",
			slog.String("found (start-stop)", fmt.Sprintf("%s - %s", res[j].Start.String(), res[j].End().String())),
			slog.String("new (start-stop)", fmt.Sprintf("%s - %s", segment.Start.String(), segment.End().String())),
		)
		// FIXME handle this errors every where.
		return service.ErrSegmentIntersection
	}

	if segment.Protected {
		chans.Notify(s.protectedSegmentsChan)
	}
	chans.Send(s.allSegmentsChan, segment)

	return nil
}

// Segment returns by its id
func (s *Schedule) Segment(ctx context.Context, id int64) (models.Segment, error) {
	const op = "Schedule.Segment"

	log := s.log.With(
		slog.String("op", op),
		slog.String("editorname", models.RootLogin),
	)

	segment, err := s.schStorage.Segment(ctx, id)
	if err != nil {
		if errors.Is(err, storage.ErrSegmentNotFound) {
			log.Warn("segment not found", slog.Int64("id", id))
			return models.Segment{}, service.ErrSegmentNotFound
		}
		if errors.Is(err, storage.ErrContextCancelled) {
			log.Error("schStorage.Segment timeout exceeded")
			return models.Segment{}, service.ErrTimeout
		}
		log.Error("failed to get segment", slog.Int64("id", id))
		return models.Segment{}, fmt.Errorf("%s: %w", op, err)
	}

	log.Info(
		"got segment",
		slog.Int64("id", id),
		slog.Int64("mediaID", *segment.MediaID),
		slog.String("start", segment.Start.Format(models.TimeFormat)),
		slog.Float64("beginCut", segment.BeginCut.Seconds()),
		slog.Float64("stopCut", segment.StopCut.Seconds()),
	)

	isProt, err := s.schStorage.IsSegmentProtected(ctx, id)
	if err != nil {
		if errors.Is(err, storage.ErrContextCancelled) {
			log.Error("schStorage.IsSegmentProtected timeout exceeded")
			return models.Segment{}, service.ErrTimeout
		}
		log.Error("failed to check segment protection", slog.Int64("id", id), sl.Err(err))
		return models.Segment{}, fmt.Errorf("%s: %w", op, err)
	}

	liveId, err := s.schStorage.LiveId(ctx, id)
	if err != nil {
		if errors.Is(err, storage.ErrContextCancelled) {
			log.Error("schStorage.LiveId timeout exceeded")
			return models.Segment{}, service.ErrTimeout
		}
		log.Error("failed to check segment id", slog.Int64("id", id), sl.Err(err))
	}

	segment.Protected = isProt
	segment.LiveId = liveId

	return segment, nil
}

// DeleteSegment deletes segment by id.
func (s *Schedule) DeleteSegment(ctx context.Context, id int64) error {
	const op = "Schedule.DeleteSegment"

	log := s.log.With(
		slog.String("op", op),
		slog.String("editorname", models.RootLogin),
	)

	isProt, err := s.schStorage.IsSegmentProtected(ctx, id)
	if err != nil {
		if errors.Is(err, storage.ErrSegmentNotFound) {
			log.Warn("segment not found", slog.Int64("id", id))
			return service.ErrSegmentNotFound
		}
		if errors.Is(err, storage.ErrContextCancelled) {
			log.Error("schStorage.IsSegmentProtected timeout exceeded")
			return service.ErrTimeout
		}
		log.Error("failed to check is segment protected")
		return fmt.Errorf("%s: %w", op, err)
	}

	if err := s.schStorage.DeleteSegment(ctx, id); err != nil {
		if errors.Is(err, storage.ErrSegmentNotFound) {
			log.Warn("segment not found", slog.Int64("id", id))
			return service.ErrSegmentNotFound
		}
		if errors.Is(err, storage.ErrContextCancelled) {
			log.Error("schStorage.DeleteSegment timeout exceeded")
			return service.ErrTimeout
		}
		log.Error("failed to delete segment", slog.Int64("id", id))
		return fmt.Errorf("%s: %w", op, err)
	}

	if isProt {
		chans.Notify(s.protectedSegmentsChan)
	}

	return nil
}

// ClearSchedule clears schedule from given timestamp.
func (s *Schedule) ClearSchedule(ctx context.Context, from time.Time) error {
	const op = "Schedule.ClearSchedule"

	log := s.log.With(
		slog.String("op", op),
		slog.String("editorname", models.RootLogin),
	)

	log.Info("clearing segments", slog.Time("from", from))

	if err := s.schStorage.ClearSchedule(ctx, from); err != nil {
		if errors.Is(err, storage.ErrContextCancelled) {
			log.Error("schStorage.ClearSchedule timeout exceeded")
			return service.ErrTimeout
		}
		log.Error("failed to clear schedule", slog.Time("from", from))
		return fmt.Errorf("%s: %w", op, err)
	}

	log.Info("cleared schedule", slog.String("from", from.Format(models.TimeFormat)))

	return nil
}
