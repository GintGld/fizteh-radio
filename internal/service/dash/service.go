package service

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/GintGld/fizteh-radio/internal/lib/logger/sl"
	"github.com/GintGld/fizteh-radio/internal/models"
)

// TODO: add metadata storage and get correct information from it

type Dash struct {
	log        *slog.Logger
	updateFreq time.Duration
	horizon    time.Duration
	manifest   Manifest
	content    Content
	schedule   Schedule

	// notify to update
	notifyChan <-chan models.Segment
	// stop
	stopChan chan struct{}

	runMutex sync.Mutex
}

// New returns new dash manager
func New(
	log *slog.Logger,
	updateFreq time.Duration,
	horizon time.Duration,
	manifest Manifest,
	content Content,
	schedule Schedule,
	notifyChan <-chan models.Segment,
) *Dash {
	return &Dash{
		log:        log,
		updateFreq: updateFreq,
		horizon:    horizon,
		manifest:   manifest,
		content:    content,
		schedule:   schedule,
		notifyChan: notifyChan,
		stopChan:   make(chan struct{}),
	}
}

type Manifest interface {
	SetSchedule(ctx context.Context, schedule []models.Segment) error
	Dump() error
	CleanUp()
}

type Content interface {
	Init() error
	Generate(ctx context.Context, segment models.Segment) error
	ClearCache() error
	CleanUp()
}

type Schedule interface {
	ScheduleCut(ctx context.Context, start time.Time, stop time.Time) ([]models.Segment, error)
	Segment(ctx context.Context, id int64) (models.Segment, error)
}

// TODO: mutex and method for updating (horizon, updatefreq)

// TODO: move SetSchedule, Generate to goroutines

// Run starts dash streaming
func (d *Dash) Run(ctx context.Context) error {
	const op = "Dash.Run"

	log := d.log.With(
		slog.String("op", op),
	)

	// mutex to prevent multiple
	// run call.
	if !d.runMutex.TryLock() {
		return nil
	}
	defer d.runMutex.Unlock()

	log.Info("start dash")

	// Before loop starts, working directories will
	// be cleaned from previous files.
	d.content.CleanUp()
	d.manifest.CleanUp()

	if err := d.content.Init(); err != nil {
		log.Error("failed to init content maker", sl.Err(err))
		return fmt.Errorf("%s: %w", op, err)
	}

mainloop:
	for {
		// Get actual schedule
		now := time.Now()
		schedule, err := d.schedule.ScheduleCut(ctx, now, now.Add(d.horizon))
		if err != nil {
			log.Error("failed to load schedule", sl.Err(err))
			return err
		}

		log.Debug("got schedule")

		// Update manifest
		if err := d.manifest.SetSchedule(ctx, schedule); err != nil {
			log.Error("failed to update schedule")
			return err
		}

		log.Debug("schedule set")

		// Save new manifest
		if err := d.manifest.Dump(); err != nil {
			log.Error("failed to dump manifest")
		}

		log.Debug("dumped manifest")

		// Create dash segments
		for _, segment := range schedule {
			if err := d.content.Generate(ctx, segment); err != nil {
				log.Error("failed to generate content", slog.Int64("id", *segment.ID), sl.Err(err))
			}
		}

		log.Debug("generated segments")

		if err := d.content.ClearCache(); err != nil {
			log.Error("failed to clear cache", sl.Err(err))
		} else {
			log.Debug("cleared cache")
		}

		timer := time.After(d.updateFreq)

	select_case:
		select {
		case segm := <-d.notifyChan:
			log.Debug("got notify chan")
			start := *segm.Start
			stop := segm.Start.Add(*segm.StopCut - *segm.BeginCut)
			now := time.Now()
			hor := now.Add(d.horizon)
			if start.After(now) && start.Before(hor) || stop.After(now) && stop.Before(hor) {
				log.Debug(
					"segment is in horizon",
					slog.String("now", now.Format(models.TimeFormat)),
					slog.String("start", start.Format(models.TimeFormat)),
					slog.String("stop", stop.Format(models.TimeFormat)),
				)
			} else {
				log.Debug("segment is not in horizon")
				goto select_case
			}
		case <-d.stopChan:
			log.Debug("got stop chan")
			break mainloop
		case <-ctx.Done():
			log.Debug("got context stop")
			break mainloop
		case <-timer:
			log.Debug("timer tick")
		}
	}

	d.log.Info("stopped dash")

	return nil
}

// Stop stops dash
func (d *Dash) Stop() {
	d.stopChan <- struct{}{}
}
