package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
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
	ScheduleCut(ctx context.Context, start time.Time, stop time.Time) ([]models.Segment, error)
	SaveSegment(ctx context.Context, segment models.Segment) (int64, error)
	Segment(ctx context.Context, period int64) (models.Segment, error)
	DeleteSegment(ctx context.Context, period int64) error
	ProtectSegment(ctx context.Context, id int64) error
	IsSegmentProtected(ctx context.Context, id int64) (bool, error)
	ClearSchedule(ctx context.Context, stamp time.Time) error
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

	log.Info("get schedule cut", slog.Time("start", start), slog.Time("stop", stop))

	segments, err := s.schStorage.ScheduleCut(ctx, start, stop)
	if err != nil {
		log.Error("failed to get schedule cut", sl.Err(err))
		return []models.Segment{}, fmt.Errorf("%s: %w", op, err)
	}

	log.Info(
		"got schedule cut",
		slog.String("start", start.Format(models.TimeFormat)),
		slog.String("stop", stop.Format(models.TimeFormat)),
		slog.Int("size", len(segments)),
	)

	for i, segment := range segments {
		if isProt, err := s.schStorage.IsSegmentProtected(ctx, *segment.ID); err != nil {
			log.Error("fialed to check segment protection", slog.Int64("id", *segment.ID), sl.Err(err))
			return []models.Segment{}, fmt.Errorf("%s: %w", op, err)
		} else {
			segments[i].Protected = isProt
		}
	}

	return segments, nil
}

// NewSegment registers new segment in schedule
// if media for segment does not exists returns error.
func (s *Schedule) NewSegment(ctx context.Context, segment models.Segment) (int64, error) {
	const op = "Schedule.NewSegment"

	log := s.log.With(
		slog.String("op", op),
		slog.String("editorname", models.RootLogin),
	)

	log.Info("validating media", slog.Int64("id", *segment.MediaID))

	media, err := s.mediaStorage.Media(ctx, *segment.MediaID)

	if err != nil {
		if errors.Is(err, storage.ErrMediaNotFound) {
			log.Warn("media not found", slog.Int64("id", *segment.MediaID))
			return 0, service.ErrMediaNotFound
		}
		log.Error("failed to get media", slog.Int64("id", *segment.MediaID), sl.Err(err))
		return 0, fmt.Errorf("%s: %w", op, err)
	}

	// Check cut correctness
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

	log.Info("media is valid", slog.Int64("id", *segment.MediaID))

	log.Info("registering new segment")

	id, err := s.schStorage.SaveSegment(ctx, segment)
	if err != nil {
		log.Error("failed to save segment", sl.Err(err))
		return 0, fmt.Errorf("%s: %w", op, err)
	}

	log.Info(
		"registered segment",
		slog.Int64("id", id),
		slog.Int64("mediaID", *segment.MediaID),
		slog.String("start", segment.Start.Format(models.TimeFormat)),
		slog.Float64("begin cut", segment.BeginCut.Seconds()),
		slog.Float64("stop cut", segment.StopCut.Seconds()),
	)

	if segment.Protected {
		if err := s.schStorage.ProtectSegment(ctx, id); err != nil {
			log.Error("failed to set segment protection", sl.Err(err))
			return 0, fmt.Errorf("%s: %w", op, err)
		}
		log.Info("protected segment", slog.Int64("id", id))

		chans.Notify(s.protectedSegmentsChan)
	}

	chans.Send(s.allSegmentsChan, segment)

	return id, nil
}

// Segment returns by its id
func (s *Schedule) Segment(ctx context.Context, id int64) (models.Segment, error) {
	const op = "Schedule.Segment"

	log := s.log.With(
		slog.String("op", op),
		slog.String("editorname", models.RootLogin),
	)

	log.Info("get segment", slog.Int64("id", id))

	segment, err := s.schStorage.Segment(ctx, id)
	if err != nil {
		if errors.Is(err, storage.ErrSegmentNotFound) {
			log.Warn("segment not found", slog.Int64("id", id))
			return models.Segment{}, service.ErrSegmentNotFound
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
		log.Error("failed to check segment protection", slog.Int64("id", id), sl.Err(err))
		return models.Segment{}, fmt.Errorf("%s: %w", op, err)
	}

	segment.Protected = isProt

	return segment, nil
}

// DeleteSegment deletes segment by id.
func (s *Schedule) DeleteSegment(ctx context.Context, id int64) error {
	const op = "Schedule.DeleteSegment"

	log := s.log.With(
		slog.String("op", op),
		slog.String("editorname", models.RootLogin),
	)

	log.Info("deleting segment", slog.Int64("id", id))

	isProt, err := s.schStorage.IsSegmentProtected(ctx, id)
	if err != nil {
		if errors.Is(err, storage.ErrSegmentNotFound) {
			log.Warn("segment not found", slog.Int64("id", id))
			return service.ErrSegmentNotFound
		}
		log.Error("failed to check is segment protected")
		return fmt.Errorf("%s: %w", op, err)
	}

	if err := s.schStorage.DeleteSegment(ctx, id); err != nil {
		if errors.Is(err, storage.ErrSegmentNotFound) {
			log.Warn("segment not found", slog.Int64("id", id))
			return service.ErrSegmentNotFound
		}
		log.Error("failed to delete segment", slog.Int64("id", id))
		return fmt.Errorf("%s: %w", op, err)
	}

	log.Info("deleted segment", slog.Int64("id", id))

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
		log.Error("failed to clear schedule", slog.Time("from", from))
		return fmt.Errorf("%s: %w", op, err)
	}

	log.Info("cleared schedule", slog.String("from", from.Format(models.TimeFormat)))

	return nil
}
