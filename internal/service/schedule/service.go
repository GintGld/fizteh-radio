package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/GintGld/fizteh-radio/internal/lib/logger/sl"
	ptr "github.com/GintGld/fizteh-radio/internal/lib/utils/pointers"
	"github.com/GintGld/fizteh-radio/internal/models"
	"github.com/GintGld/fizteh-radio/internal/service"
	"github.com/GintGld/fizteh-radio/internal/storage"
)

type Schedule struct {
	log          *slog.Logger
	schStorage   ScheduleStorage
	mediaStorage MediaStorage
}

type ScheduleStorage interface {
	ScheduleCut(ctx context.Context, start time.Time, stop time.Time) ([]models.Segment, error)
	SaveSegment(ctx context.Context, segment models.Segment) (int64, error)
	Segment(ctx context.Context, period int64) (models.Segment, error)
	DeleteSegment(ctx context.Context, period int64) error
}

type MediaStorage interface {
	Media(ctx context.Context, id int64) (models.Media, error)
}

func New(
	log *slog.Logger,
	schStorage ScheduleStorage,
	mediaStorage MediaStorage,
) *Schedule {
	return &Schedule{
		log:          log,
		schStorage:   schStorage,
		mediaStorage: mediaStorage,
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
		slog.Time("start", start),
		slog.Time("stop", stop),
		slog.Int("size", len(segments)),
	)

	return segments, nil
}

// NewSegment registers new segment in schedule
// if media for segment does not exists returns error
func (s *Schedule) NewSegment(ctx context.Context, segment models.Segment) (int64, error) {
	const op = "Schedule.NewSegment"

	log := s.log.With(
		slog.String("op", op),
		slog.String("editorname", models.RootLogin),
	)

	log.Info("validating media", slog.Int64("id", *segment.MediaID))

	if media, err := s.mediaStorage.Media(ctx, *segment.MediaID); err != nil {
		if errors.Is(err, storage.ErrMediaNotFound) {
			log.Warn("media not found", slog.Int64("id", *segment.MediaID))
			return 0, service.ErrMediaNotFound
		}
		log.Error("failed to get media", slog.Int64("id", *segment.MediaID), sl.Err(err))
		return 0, fmt.Errorf("%s: %w", op, err)
	} else {
		// TODO: it is temporary fix, remove it later
		segment.BeginCut = ptr.Ptr(time.Duration(0))
		segment.StopCut = ptr.Ptr(*media.Duration)
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
		slog.String("start", segment.Start.String()),
		slog.Float64("begin cut", segment.BeginCut.Seconds()),
		slog.Float64("stop cut", segment.StopCut.Seconds()),
	)

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
		slog.Time("start", *segment.Start),
		slog.Float64("beginCut", segment.BeginCut.Seconds()),
		slog.Float64("stopCut", segment.StopCut.Seconds()),
	)

	return segment, nil
}

// DeleteSegment deletes segment by id
func (s *Schedule) DeleteSegment(ctx context.Context, id int64) error {
	const op = "Schedule.DeleteSegment"

	log := s.log.With(
		slog.String("op", op),
		slog.String("editorname", models.RootLogin),
	)

	log.Info("deleting segment", slog.Int64("id", id))

	if err := s.schStorage.DeleteSegment(ctx, id); err != nil {
		if errors.Is(err, storage.ErrSegmentNotFound) {
			log.Warn("segment not found", slog.Int64("id", id))
			return service.ErrSegmentNotFound
		}
		log.Error("failed to delete segment", slog.Int64("id", id))
		return fmt.Errorf("%s: %w", op, err)
	}

	log.Info("deleted segment", slog.Int64("id", id))

	return nil
}
