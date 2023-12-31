package service

import (
	"context"
	"log/slog"
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

	notifyChan chan struct{}
	stopChan   chan struct{}
}

// New returns new dash manager
func New(
	log *slog.Logger,
	updateFreq time.Duration,
	horizon time.Duration,
	manifest Manifest,
	content Content,
	schedule Schedule,
) *Dash {
	return &Dash{
		log:        log,
		updateFreq: updateFreq,
		horizon:    horizon,
		manifest:   manifest,
		content:    content,
		schedule:   schedule,
		notifyChan: make(chan struct{}, 1),
		stopChan:   make(chan struct{}),
	}
}

type Manifest interface {
	SetSchedule(ctx context.Context, schedule []models.Segment) error
	Dump() error
	CleanUp()
}

type Content interface {
	Generate(ctx context.Context, segment *models.Segment) error
	CleanUp()
}

type Schedule interface {
	ScheduleCut(ctx context.Context, start time.Time, stop time.Time) ([]models.Segment, error)
	Segment(ctx context.Context, id int64) (models.Segment, error)
}

// TODO: mutex and method for updating (horizon, updatefreq)

// TODO: move SetSchedule, Generate to goroutines

// TODO: cleanup content during loop
// make method in manifest `segments in use`
// (don't forget about mutex)
// make method in content `delete all segment not in use`

// Run starts dash streaming
func (d *Dash) Run(ctx context.Context) error {
	const op = "Dash.Run"

	log := d.log.With(
		slog.String("op", op),
	)

	log.Info("start dash")

	// After loop stops, all generated files will be deleted
	defer d.content.CleanUp()
	defer d.manifest.CleanUp()

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
			if err := d.content.Generate(ctx, &segment); err != nil {
				log.Error("failed to generate content", slog.Int64("id", *segment.ID), sl.Err(err))
			}
		}

		log.Debug("generated segments")

		select {
		case <-time.After(d.updateFreq):
			log.Debug("timer tick")
		case <-d.notifyChan:
			log.Debug("got notify chan")
		case <-d.stopChan:
			log.Debug("got stop chan")
			break mainloop
		case <-ctx.Done():
			log.Debug("got context stop")
			break mainloop
		}
	}

	d.log.Info("stopped dash")

	return nil
}

// Notify notifies dash to
// unscheduled updating
func (d *Dash) Notify() {
	d.notifyChan <- struct{}{}
}

// Stop stops dash
func (d *Dash) Stop() {
	d.stopChan <- struct{}{}
}
