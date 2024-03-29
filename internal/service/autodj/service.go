package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"math/rand"
	"sync"
	"time"

	"github.com/GintGld/fizteh-radio/internal/lib/logger/sl"
	chans "github.com/GintGld/fizteh-radio/internal/lib/utils/channels"
	ptr "github.com/GintGld/fizteh-radio/internal/lib/utils/pointers"
	"github.com/GintGld/fizteh-radio/internal/models"
	"github.com/GintGld/fizteh-radio/internal/service"
)

// TODO: save config and restore it
// TODO: move segmentsBuff to config

const (
	// Number of segments to have in buffer.
	segmentsBuff = 5
	// Time delay for fixedNow() function
	// all absolute time values for this package
	// are shifted ny this values to give time
	// for dash package to create segments.
	timeDelay = 2 * time.Second
)

var (
	infinity = time.Date(2100, 0, 0, 0, 0, 0, 0, time.Local)
)

// TODO: add comment
func fixedNow() time.Time {
	return time.Now().Add(timeDelay)
}

type AutoDJ struct {
	// Dependencies
	log   *slog.Logger
	media MediaSearcher
	sch   Schedule
	conf  models.AutoDJConfig

	// External notifying channels
	scheduleChan <-chan struct{}
	mediaChan    <-chan struct{}

	// Internal channels
	confChan  chan struct{}
	stopChan  chan struct{}
	confMutex sync.Mutex
	runMutex  sync.Mutex

	// Cache
	timeHorizon       time.Time
	library           []models.Media
	stub              models.Media
	protectedSegments []models.Segment
	shuffledIds       []int64
	currentId         int64
	timerId           int
	stubWasUsed       bool
}

func New(
	log *slog.Logger,
	media MediaSearcher,
	sch Schedule,
	scheduleChan <-chan struct{},
	mediaChan <-chan struct{},
) *AutoDJ {
	return &AutoDJ{
		log:   log,
		media: media,
		sch:   sch,
		conf: models.AutoDJConfig{
			Tags: make(models.TagList, 0),
			Stub: models.AutoDJStub{
				Threshold: time.Duration(0),
				MediaID:   0,
			},
		},
		scheduleChan: scheduleChan,
		mediaChan:    mediaChan,
		confChan:     make(chan struct{}),
		stopChan:     make(chan struct{}),
	}
}

type MediaSearcher interface {
	SearchMedia(ctx context.Context, filter models.MediaFilter) ([]models.Media, error)
	Media(ctx context.Context, id int64) (models.Media, error)
}

type Schedule interface {
	ScheduleCut(ctx context.Context, start time.Time, stop time.Time) ([]models.Segment, error)
	NewSegment(ctx context.Context, segment models.Segment) (int64, error)
	ClearSchedule(ctx context.Context, from time.Time) error
}

// SetCriteria updates AutoDJ settings.
func (a *AutoDJ) SetConfig(conf models.AutoDJConfig) {
	a.confMutex.Lock()
	a.conf = conf
	chans.Notify(a.confChan)
	a.confMutex.Unlock()
}

// Config returns actual AutoDJ settings.
func (a *AutoDJ) Config() models.AutoDJConfig {
	a.confMutex.Lock()
	conf := a.conf
	a.confMutex.Unlock()
	return conf
}

// Run starts filling schedule.
//
// Fills empty time with media, satisfying
// given cirteria.
// Empty time is time not reserved
// by protected segments.
func (a *AutoDJ) Run(ctx context.Context) error {
	const op = "AutoDJ.Run"

	log := a.log.With(
		slog.String("op", op),
		slog.String("editorname", models.RootLogin),
	)

	// mutex to prevent multiple
	// run call.
	if !a.runMutex.TryLock() {
		return nil
	}
	defer a.runMutex.Unlock()

	log.Info("start autodj")

dj_start:
	// Get library with given parameters.
	if err := a.updateLibrary(ctx); err != nil {
		log.Error("failed to update library", sl.Err(err))
		return fmt.Errorf("%s: %w", op, err)
	}

	// Setup shuffled indices.
	a.updateIndices()

	// Update protected segments info.
	if err := a.updateProtected(ctx); err != nil {
		log.Error("failed to update schedule", sl.Err(err))
		return fmt.Errorf("%s: %w", op, err)
	}

	a.timerId = 0

	// If some segment is playing,
	// postpone dj start till the end
	// of this segment.
	// Repeat this untill, dj will not
	// "step on empty space"
	var (
		s   models.Segment
		err error
	)
	// distinguish protected and not
	for s, err = a.nowPlaying(ctx); err == nil; s, err = a.nowPlaying(ctx) {
		untill := time.Until(s.Start.Add(*s.StopCut-*s.BeginCut)) - timeDelay
		log.Debug("something is playing now, wait till it ends", slog.Time("now", time.Now()), slog.Float64("untill", untill.Seconds()))
		select {
		case <-time.After(untill):
			log.Debug("segment ended, try to start autodj")
		case <-a.stopChan:
			log.Debug("got stop chan")
			return nil
		case <-ctx.Done():
			log.Debug("got stop chan")
			return nil
		}
	}
	if !errors.Is(err, service.ErrSegmentNotFound) {
		log.Debug("failed to get current playing segment", sl.Err(err))
		return fmt.Errorf("%s: %w", op, err)
	}

	a.timeHorizon = fixedNow()
	log.Debug("start time", slog.Time("", a.timeHorizon))

	// Add some segments at the beginning.
	for i := 0; i < segmentsBuff; i++ {
		if err := a.addSegment(ctx); err != nil {
			log.Error("failed to add new segment", sl.Err(err))
		}
		log.Debug("time hor", slog.Time("", a.timeHorizon))
	}

	log.Debug("added first segments")

main_loop:
	for {
		// Add one by one.
		if err := a.addSegment(ctx); err != nil {
			log.Error("failed to add new segment", sl.Err(err))
		}

		select {
		case <-a.getTimer():
			log.Debug("timer tick")
		case <-a.confChan:
			log.Debug("got conf chan")
			goto dj_start
		case <-a.scheduleChan:
			log.Debug("got schedule chan")
			if err := a.updateProtected(ctx); err != nil {
				log.Error("failed to update schedule", sl.Err(err))
				return fmt.Errorf("%s: %w", op, err)
			}
		case <-a.mediaChan:
			log.Debug("got media chan")
			if err := a.updateLibrary(ctx); err != nil {
				log.Error("failed to update schedule", sl.Err(err))
				return fmt.Errorf("%s: %w", op, err)
			}
			a.updateIndices()
		case <-a.stopChan:
			log.Debug("got stop chan")
			break main_loop
		case <-ctx.Done():
			log.Debug("got stop chan")
			break main_loop
		}
	}

	log.Info("finish autodj")

	return nil
}

// addSegment adds new segment to the schedule.
// New segment starts at the end of the previous one.
//
// If there's an intersection with protected segment,
// AutoDJ prevents it by cutting the segment.
func (a *AutoDJ) addSegment(ctx context.Context) error {
	const op = "AutoDJ.addSegment"

	log := a.log.With(
		slog.String("op", op),
	)

	conf := a.Config()

	// Get next media to put in schedule.
	var media models.Media
	if int(a.currentId) >= len(a.shuffledIds) {
		a.currentId = 0
	}
	media = a.library[a.currentId]
	a.currentId++

	// Create new segment
	newSegm := models.Segment{
		MediaID:   ptr.Ptr(*media.ID),
		Start:     ptr.Ptr(a.timeHorizon),
		BeginCut:  ptr.Ptr[time.Duration](0),
		StopCut:   ptr.Ptr(*media.Duration),
		Protected: false,
	}

	// Get id for the nearest protected segment
	// to prevent segment intersection.
	protedctedId := a.nearestProtectedSegment()

	if protedctedId != -1 &&
		a.timeHorizon.Add(*media.Duration).After(*a.protectedSegments[protedctedId].Start) {
		protectedStart := *a.protectedSegments[protedctedId].Start

		// Handle small time with stub
		if protectedStart.Sub(a.timeHorizon) <= conf.Stub.Threshold {
			newSegm = models.Segment{
				MediaID:   ptr.Ptr(*a.stub.ID),
				Start:     ptr.Ptr(a.timeHorizon),
				BeginCut:  ptr.Ptr[time.Duration](0),
				StopCut:   ptr.Ptr(*a.stub.Duration),
				Protected: false,
			}
			a.stubWasUsed = true
		} else {
			// Handle possible intersection
			// by cutting the end of new segment.
			cut := protectedStart.Sub(a.timeHorizon)
			newSegm.StopCut = &cut
		}
	}

	// Add new segment.
	if _, err := a.sch.NewSegment(ctx, newSegm); err != nil {
		log.Error("failed to add new segment")
		return fmt.Errorf("%s: %w", op, err)
	}

	// Move time horizon.
	a.timeHorizon = a.timeHorizon.Add(*newSegm.StopCut)

	return nil
}

// getTimer returns timer to wait before
// add new segment.
func (a *AutoDJ) getTimer() (timer <-chan time.Time) {
	if a.timerId >= len(a.library) {
		a.timerId = 0
	}
	if a.stubWasUsed {
		timer = time.After(*a.stub.Duration)
		a.stubWasUsed = false
	} else {
		timer = time.After(*a.library[a.timerId].Duration)
		a.timerId++
	}

	return
}

// updateLibrary updates library via current config.
func (a *AutoDJ) updateLibrary(ctx context.Context) error {
	const op = "AutoDJ.updateLibrary"

	log := a.log.With(
		slog.String("op", op),
	)

	log.Debug("library update")

	var err error

	conf := a.Config()
	tagNames := make([]string, len(conf.Tags))
	for i, t := range conf.Tags {
		tagNames[i] = t.Name
	}

	a.library, err = a.media.SearchMedia(ctx, models.MediaFilter{Tags: tagNames})
	if err != nil {
		log.Error("failed to update library", sl.Err(err))
		return fmt.Errorf("%s: %w", op, err)
	}

	// Update tail media
	if conf.Stub.Threshold > 0 {
		a.stub, err = a.media.Media(ctx, conf.Stub.MediaID)
		if err != nil {
			if errors.Is(err, service.ErrMediaNotFound) {
				log.Error("invalid tail media", slog.Int64("id", conf.Stub.MediaID))
				return service.ErrMediaNotFound
			}
			log.Error("failed to get tail media, disable tail", slog.Int64("id", conf.Stub.MediaID))
			return fmt.Errorf("%s: %w", op, err)
		}
	}

	return nil
}

// updateIndices updates shuffled indices.
func (a *AutoDJ) updateIndices() {
	var lastId int64 = -1
	if len(a.shuffledIds) > 0 {
		lastId = a.shuffledIds[len(a.shuffledIds)-1]
	}

	sl := rand.Perm(len(a.library))
	for sl[len(sl)-1] == int(lastId) {
		sl = rand.Perm(len(a.library))
	}

	a.shuffledIds = make([]int64, 0, len(sl))

	for _, id := range sl {
		a.shuffledIds = append(a.shuffledIds, int64(id))
	}

	a.currentId = 0
}

// updateProtected updates schedule with protected segments.
// Reconfigures caches to make schedule update smooth.
func (a *AutoDJ) updateProtected(ctx context.Context) error {
	const op = "AutoDJ.updateProtected"

	log := a.log.With(
		slog.String("op", op),
	)

	// Get segments.
	now := fixedNow()
	sch, err := a.sch.ScheduleCut(ctx, now, infinity)
	if err != nil {
		log.Error("failed to get schedule cut", sl.Err(err))
		return fmt.Errorf("%s: %w", op, err)
	}

	// Create slices for protected (external) segments
	// and unprotected (internal) segments from now.
	a.protectedSegments = make([]models.Segment, 0)
	djSch := make([]models.Segment, 0)
	for _, s := range sch {
		if s.Protected {
			a.protectedSegments = append(a.protectedSegments, s)
		} else {
			djSch = append(djSch, s)
		}
	}

	log.Debug("splitted schedule", slog.Any("prot.", a.protectedSegments), slog.Any("not prot.", djSch))

	// Handle protected segments.
	// It is neccesesary to consider
	// only first protected segment.
	if len(a.protectedSegments) > 0 {
		protSegmentStart := *a.protectedSegments[0].Start
		protSegmentStop := protSegmentStart.Add(*a.protectedSegments[0].StopCut - *a.protectedSegments[0].BeginCut)

		// Protected segment is now playing case.
		// This case usually appears, if autodj dj
		// starts when protected segment is
		// already playing.
		if protSegmentStart.Before(now) {
			log.Debug("now is playing prot.", slog.Time("now", now))

			// Clear schedule.
			if err := a.sch.ClearSchedule(ctx, now); err != nil {
				log.Error("failed to clear schedule", sl.Err(err))
				return fmt.Errorf("%s: %w", op, err)
			}

			// Move time horizon to the end of
			// protected segment.
			a.timeHorizon = protSegmentStop

			return nil
		}

		// Find dj's first segment that intersects
		// first protected segment.
		for i, s := range djSch {
			log.Debug("looking for prot.", slog.Time("now", now))

			djSegmentStop := s.Start.Add(*s.StopCut - *s.BeginCut)

			if djSegmentStop.After(protSegmentStart) {
				log.Debug("found protected",
					slog.Time("dj stop", djSegmentStop),
					slog.Time("protected start", protSegmentStart),
				)

				// Clear schedule from "bad" dj's segments.
				// Subtract second to delete current segment also.
				if err := a.sch.ClearSchedule(ctx, djSegmentStop.Add(-time.Second)); err != nil {
					log.Error("failed to clear schedule", sl.Err(err))
					return fmt.Errorf("%s: %w", op, err)
				}

				slog.Debug("clear schedule", slog.Time("from", djSegmentStop.Add(-time.Second)))

				// Move time horizon to the start of
				// the first deleted segment.
				a.timeHorizon = *s.Start

				// Move media id backwards to keep
				// media order.
				a.currentId -= int64(len(djSch) - i - 1)
				if a.currentId < 0 {
					a.currentId += int64(len(a.shuffledIds))
				}

				log.Debug("new a.currentId", slog.Int64("", a.currentId))
			}
		}

		log.Debug("new horizon", slog.Time("", a.timeHorizon))
	}

	return nil
}

// nowPlaying returns segment playing now.
// If there is no such, return
// service.ErrSegmentNotFound error.
func (a *AutoDJ) nowPlaying(ctx context.Context) (models.Segment, error) {
	const op = "AutoDJ.nowPlaying"

	log := a.log.With(
		slog.String("op", op),
	)

	now := fixedNow()
	sch, err := a.sch.ScheduleCut(ctx, now, infinity)
	if err != nil {
		log.Error("failed to get schedule cut", sl.Err(err))
		return models.Segment{}, fmt.Errorf("%s: %w", op, err)
	}

	if len(sch) == 0 {
		return models.Segment{}, service.ErrSegmentNotFound
	}

	if sch[0].Start.Before(now) {
		if err := a.sch.ClearSchedule(ctx, sch[0].Start.Add(*sch[0].StopCut-*sch[0].BeginCut+timeDelay)); err != nil {
			log.Error("failed to clear schedule", sl.Err(err))
		}
		return sch[0], nil
	}

	return models.Segment{}, service.ErrSegmentNotFound
}

// nearestProtectedSegment return nearest protected segment.
func (a *AutoDJ) nearestProtectedSegment() int64 {
	const op = "AutoDJ.nearestProtectedSegment"

	log := a.log.With(
		slog.String("op", op),
	)

	for i, s := range a.protectedSegments {
		if s.Start.Before(a.timeHorizon) &&
			s.Start.Add(*s.StopCut-*s.BeginCut).After(a.timeHorizon) {
			log.Warn("time horizon intersects protected segment", slog.Int("id", i))
		}
		if s.Start.After(a.timeHorizon) {
			return int64(i)
		}
	}
	return -1
}

// IsPlaying returns autodj status.
func (a *AutoDJ) IsPlaying() bool {
	if a.runMutex.TryLock() {
		a.runMutex.Unlock()
		return false
	}
	return true
}

func (a *AutoDJ) Stop() {
	chans.Notify(a.stopChan)
}
