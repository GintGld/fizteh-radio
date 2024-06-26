package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/GintGld/fizteh-radio/internal/lib/logger/sl"
	"github.com/GintGld/fizteh-radio/internal/models"
	"github.com/GintGld/fizteh-radio/internal/service"
)

// TODO: add metadata storage and get correct information from it

type Dash struct {
	log        *slog.Logger
	ctxTimeout time.Duration
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
	ctxTimeout time.Duration,
	updateFreq time.Duration,
	horizon time.Duration,
	manifest Manifest,
	content Content,
	schedule Schedule,
	notifyChan <-chan models.Segment,
) *Dash {
	return &Dash{
		log:        log,
		ctxTimeout: ctxTimeout,
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

// RunInfinitely runs dash,
// if it returns an errror, restarts.
func (d *Dash) RunInfinitely(ctx context.Context) {
	const op = "Dash.RunInfinetely"

	log := d.log.With(
		slog.String("op", op),
	)

	log.Info("start infinite run")

inf_loop:
	for {
		if err := d.Run(ctx); err != nil {
			log.Error("run failed with an error, restart", sl.Err(err))
		} else {
			log.Info("run exited normally, stop")
			break inf_loop
		}

		select {
		case <-ctx.Done():
			break inf_loop
		default:
		}
	}

	log.Info("stop infinite run")
}

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
		ctxSchCut, cancelSchCut := context.WithTimeout(ctx, d.ctxTimeout)
		defer cancelSchCut()
		now := time.Now()
		schedule, err := d.schedule.ScheduleCut(ctxSchCut, now, now.Add(d.horizon))
		if err != nil {
			if errors.Is(err, service.ErrTimeout) {
				log.Error("schedule cut timeout exceeded, wait next iteration")
				goto select_case_with_timer
			}
			log.Error("failed to load schedule", sl.Err(err))
			return err
		}

		// Update manifest.
		// Do not set timeout since
		// ctx not used in manifest.
		if err := d.manifest.SetSchedule(ctx, schedule); err != nil {
			log.Error("failed to update schedule")
			return err
		}

		// Save new manifest
		if err := d.manifest.Dump(); err != nil {
			log.Error("failed to dump manifest")
		}

		// Create dash chunks for non-live segments.
		for _, segment := range schedule {
			if segment.LiveId == 0 {
				ctxContent, cancelContent := context.WithTimeout(ctx, d.ctxTimeout)
				defer cancelContent()
				if err := d.content.Generate(ctxContent, segment); err != nil {
					if errors.Is(err, service.ErrTimeout) {
						log.Error("content.Generate timout exceeded, wait next iteration")
						goto select_case_with_timer
					}
					log.Error("failed to generate content", slog.Int64("id", *segment.ID), sl.Err(err))
				}
			}
		}

		if err := d.content.ClearCache(); err != nil {
			log.Error("failed to clear cache", sl.Err(err))
		}

	select_case_with_timer:
		timer := time.After(d.updateFreq)

	select_case:
		select {
		case segm := <-d.notifyChan:
			start := *segm.Start
			stop := segm.Start.Add(*segm.StopCut - *segm.BeginCut)
			now := time.Now()
			hor := now.Add(d.horizon)
			if start.After(hor) || stop.Before(now) {
				log.Debug("segment is not in horizon")
				goto select_case
			}
		case <-d.stopChan:
			break mainloop
		case <-ctx.Done():
			break mainloop
		case <-timer:
		}
	}

	d.log.Info("stopped dash")

	return nil
}

// Stop stops dash
func (d *Dash) Stop() {
	d.stopChan <- struct{}{}
}
