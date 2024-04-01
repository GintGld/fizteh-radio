package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"math/rand"
	"os"
	"slices"
	"sync"
	"time"

	"github.com/GintGld/fizteh-radio/internal/lib/logger/sl"
	chans "github.com/GintGld/fizteh-radio/internal/lib/utils/channels"
	ptr "github.com/GintGld/fizteh-radio/internal/lib/utils/pointers"
	"github.com/GintGld/fizteh-radio/internal/models"
	"github.com/GintGld/fizteh-radio/internal/service"
	"github.com/gofiber/fiber/v2/log"
)

// TODO: save config and restore it
// TODO: move segmentsBuff to config

const (
	// Number of segments to have in buffer.
	segmentsBuff = 5
	// "Quant" of time. It is supposed that
	// all media will be shorter that it.
	timeDelta = time.Second
)

var (
	infinity = time.Date(2100, 0, 0, 0, 0, 0, 0, time.Local)
)

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
	// stubWasUsed       bool
	cacheFile string
}

func New(
	log *slog.Logger,
	media MediaSearcher,
	sch Schedule,
	cacheFile string,
	scheduleChan <-chan struct{},
	mediaChan <-chan struct{},
) *AutoDJ {
	a := &AutoDJ{
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
		cacheFile:    cacheFile,
		confChan:     make(chan struct{}),
		stopChan:     make(chan struct{}),
	}

	a.recoverConfig()

	return a
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

// SetConfig updates AutoDJ settings.
func (a *AutoDJ) SetConfig(conf models.AutoDJConfig) {
	a.confMutex.Lock()
	defer a.confMutex.Unlock()
	a.conf = conf
	if a.IsPlaying() {
		chans.Notify(a.confChan)
	}
	a.saveConfig()
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
		if errors.Is(err, service.ErrMediaNotFound) {
			log.Error("library is empty, stop autodj")
			return service.ErrMediaNotFound
		}
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

	var (
		s   models.Segment
		err error
	)
	if s, err = a.nowPlaying(ctx); err == nil {
		log.Debug("something is playing now, start after that")
		a.timeHorizon = s.End()
		if err := a.sch.ClearSchedule(ctx, a.timeHorizon.Add(timeDelta)); err != nil {
			log.Error("failed to clear schedule", sl.Err(err))
			return fmt.Errorf("%s: %w", op, err)
		}
	} else {
		if errors.Is(err, service.ErrSegmentNotFound) {
			a.timeHorizon = time.Now()
			log.Debug("nothing playing", slog.Time("horizon", a.timeHorizon))
		} else {
			log.Error("failed to call a.nowPlaying", sl.Err(err))
			return fmt.Errorf("%s: %w", op, err)
		}
	}

	log.Debug("start time", slog.Time("", a.timeHorizon))

main_loop:
	for {
		// Count how many dj segment
		// are now in schedule
		sch, err := a.sch.ScheduleCut(ctx, time.Now(), infinity)
		if err != nil {
			log.Error("failed to get schedule cut", sl.Err(err))
			return fmt.Errorf("%s: %w", op, err)
		}
		counter := 0
		for _, s := range sch {
			if !s.Protected {
				counter++
			}
		}

		// Add segments to keep fixed number of them
		// in schedule.
		// If error occures, don't stop.
		for i := counter; i <= segmentsBuff; i++ {
			if err := a.addSegment(ctx); err != nil {
				if errors.Is(err, service.ErrSegmentIntersection) {
					log.Error("failed to add new segment (intersection)")
				} else {
					log.Error("failed to add new segment", sl.Err(err))
				}
			}
		}

		timer, err := a.getTimer(ctx)
		if err != nil {
			log.Error("failed to get timer", sl.Err(err))
			return fmt.Errorf("%s: %w", op, err)
		}

		select {
		case <-timer:
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
				if errors.Is(err, service.ErrMediaNotFound) {
					log.Error("library is empty, stop autodj")
					return service.ErrMediaNotFound
				}
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

	// conf := a.Config()

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
	protectedId := a.nearestProtectedSegment()

	if protectedId != -1 &&
		a.timeHorizon.Add(*media.Duration).After(*a.protectedSegments[protectedId].Start) {
		protectedStart := *a.protectedSegments[protectedId].Start

		// TODO enable stubs
		// Handle small time with stub
		// if protectedStart.Sub(a.timeHorizon) <= conf.Stub.Threshold {
		// 	newSegm = models.Segment{
		// 		MediaID:   ptr.Ptr(*a.stub.ID),
		// 		Start:     ptr.Ptr(a.timeHorizon),
		// 		BeginCut:  ptr.Ptr[time.Duration](0),
		// 		StopCut:   ptr.Ptr(*a.stub.Duration),
		// 		Protected: false,
		// 	}
		// 	a.stubWasUsed = true
		// } else {
		// 	// Handle possible intersection
		// 	// by cutting the end of new segment.
		// 	cut := protectedStart.Sub(a.timeHorizon)
		// 	newSegm.StopCut = &cut
		// }
		cut := protectedStart.Sub(a.timeHorizon)
		newSegm.StopCut = &cut

		// Shift autodj horizon to the end of
		// protected media series.
		i := protectedId + 1
		for i < len(a.protectedSegments) &&
			a.protectedSegments[i].Start.Sub(*a.protectedSegments[i-1].Start) < timeDelta {
			i++
		}
		s := a.protectedSegments[i-1]
		a.timeHorizon = s.Start.Add(*s.StopCut - *s.BeginCut)

		// Delete protected segments that
		// already got around.
		if i < len(a.protectedSegments) {
			a.protectedSegments = a.protectedSegments[i:]
		} else {
			a.protectedSegments = nil
		}

		// FIXME: crutch, fix it may be
		// If supposed segment has zero duration,
		// just skip its adding to schedule.
		// This situiation usually appears
		// during stacking protected segments,
		// dj tries to put segment after first protected one
		// and the next step cuts it to zero.
		if cut == 0 {
			return nil
		}
	} else {
		// Move time horizon.
		a.timeHorizon = a.timeHorizon.Add(*newSegm.StopCut)
	}

	// Add new segment.
	if _, err := a.sch.NewSegment(ctx, newSegm); err != nil {
		if errors.Is(err, service.ErrSegmentIntersection) {
			log.Error(
				"failed to add segment (intersection)",
				slog.String("segm start - stop", fmt.Sprintf("%s - %s", newSegm.Start.String(), newSegm.End().String())),
			)
			return service.ErrSegmentIntersection
		}
		log.Error("failed to add new segment", sl.Err(err))
		return fmt.Errorf("%s: %w", op, err)
	}

	return nil
}

// getTimer returns timer to wait before
// add new segment.
func (a *AutoDJ) getTimer(ctx context.Context) (<-chan time.Time, error) {
	if a.timerId >= len(a.library) {
		a.timerId = 0
	}

	sch, err := a.sch.ScheduleCut(ctx, time.Now(), infinity)
	if err != nil {
		log.Error("failed to get schedule cut")
	}

	a.timerId++

	// Take first dj period in future
	// wait until it start playing
	j := slices.IndexFunc(sch, func(s models.Segment) bool {
		return !s.Protected && s.Start.After(time.Now())
	})

	// No dj periods
	if j == -1 {
		return time.After(0), nil
	}

	return time.After(time.Until(*sch[j].Start)), nil
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

	if len(a.library) == 0 {
		log.Error("library is empty, stop autodj")
		return service.ErrMediaNotFound
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

	defer func() {
		log.Debug("hor after updateProtected", slog.Time("", a.timeHorizon))
	}()

	// Get segments.
	now := time.Now()
	sch, err := a.sch.ScheduleCut(ctx, now, infinity)
	if err != nil {
		log.Error("failed to get schedule cut", sl.Err(err))
		return fmt.Errorf("%s: %w", op, err)
	}

	log.Debug("got sch", slog.Any("", sch))

	// Create slices for protected (external) segments
	a.protectedSegments = make([]models.Segment, 0)
	for _, s := range sch {
		if s.Protected {
			a.protectedSegments = append(a.protectedSegments, s)
		}
	}

	// If nothig plays now,
	// set time horizon to now
	if _, err := a.nowPlaying(ctx); err != nil {
		if errors.Is(err, service.ErrSegmentNotFound) {
			log.Warn("nothing playing now, set horizon to now")
			a.timeHorizon = now
			if err := a.sch.ClearSchedule(ctx, now); err != nil {
				log.Error("failed to clear schedule", sl.Err(err))
				return fmt.Errorf("%s: %w", op, err)
			}
			return nil
		}
		log.Error("failed to get current playing segment", sl.Err(err))
		return fmt.Errorf("%s: %w", op, err)
	}

	if len(sch) == 0 {
		return nil
	}

	// special case if there is only one
	// segment in schedule.
	// If it's plying now, move horizon to its end,
	// delete othervise.
	if len(sch) == 1 {
		if sch[0].Start.Before(now) {
			a.timeHorizon = sch[0].End()
			return nil
		} else {
			if err := a.sch.ClearSchedule(ctx, now); err != nil {
				log.Error("failed to clear schedule", sl.Err(err))
				return fmt.Errorf("%s: %w", op, err)
			}
			a.timeHorizon = time.Now()
			return nil
		}
	}

	// Iterate over segments in schedule
	// search for gap, produces by deleting
	// unprotected segments in Media.NewSegment function
	// Find last segment before gap, move horizon
	// to its end, clear schedule after this time point
	for i := 0; i < len(sch)-1; i++ {
		s1 := sch[i]
		s2 := sch[i+1]
		if s1.End() != *s2.Start {
			log.Debug("found cutted empty interval", slog.Time("begin", s1.End()), slog.Time("stop", *s2.Start))
			a.timeHorizon = s1.End()
			if s1.Protected {
				if err := a.sch.ClearSchedule(ctx, now); err != nil {
					log.Error("failed to clear schedule", sl.Err(err))
					return fmt.Errorf("%s: %w", op, err)
				}
				return nil
			} else {
				if err := a.sch.ClearSchedule(ctx, s1.End().Add(timeDelta)); err != nil {
					log.Error("failed to clear schedule", sl.Err(err))
					return fmt.Errorf("%s: %w", op, err)
				}

				// Recover id of last segment before gap
				// to continue playing the same segment.
				lastId := slices.IndexFunc(a.library, func(m models.Media) bool {
					return *m.ID == *s1.MediaID
				})

				if lastId == -1 {
					log.Error("failed to recover last valid media used by dj, reset to 0")
				}

				a.currentId = int64(lastId) + 1
				return nil
			}
		}
	}

	// There's no gap because
	// new protected segment is last.
	a.timeHorizon = sch[len(sch)-1].End()

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

	now := time.Now()
	sch, err := a.sch.ScheduleCut(ctx, now, infinity)
	if err != nil {
		log.Error("failed to get schedule cut", sl.Err(err))
		return models.Segment{}, fmt.Errorf("%s: %w", op, err)
	}

	if len(sch) == 0 {
		return models.Segment{}, service.ErrSegmentNotFound
	}

	if sch[0].Start.Before(now) {
		if err := a.sch.ClearSchedule(ctx, sch[0].Start.Add(*sch[0].StopCut-*sch[0].BeginCut)); err != nil {
			log.Error("failed to clear schedule", sl.Err(err))
		}
		return sch[0], nil
	}

	return models.Segment{}, service.ErrSegmentNotFound
}

// nearestProtectedSegment return nearest protected segment.
func (a *AutoDJ) nearestProtectedSegment() int {
	const op = "AutoDJ.nearestProtectedSegment"

	log := a.log.With(
		slog.String("op", op),
	)

	for i, s := range a.protectedSegments {
		stop := s.Start.Add(*s.StopCut - *s.BeginCut)
		if stop.After(a.timeHorizon) {
			log.Debug("time horizon intersects protected segment", slog.Int("id", i))
			return i
		}
	}
	return -1
}

// recoverConfig tries to read config file.
func (a *AutoDJ) recoverConfig() {
	const op = "AutoDj.recoverConfig"

	log := a.log.With(
		slog.String("op", op),
		slog.String("file", a.cacheFile),
	)

	file, err := os.Open(a.cacheFile)
	if err != nil {
		log.Warn("failed to open file", sl.Err(err))
		return
	}
	defer file.Close()

	body, err := io.ReadAll(file)
	if err != nil {
		log.Error("failed to read file", sl.Err(err))
		return
	}

	if err := json.Unmarshal(body, &a.conf); err != nil {
		log.Error("failed to parse config", sl.Err(err))
		a.conf = models.AutoDJConfig{}
		return
	}
}

// saveConfig saves config to a file.
// does not use mutex, since called only
// from SetConfing, which already mutex config.
func (a *AutoDJ) saveConfig() {
	const op = "AutoDJ.saveConfig"

	log := a.log.With(
		slog.String("op", op),
		slog.String("file", a.cacheFile),
	)

	file, err := os.OpenFile(a.cacheFile, os.O_WRONLY|os.O_CREATE, 0644)
	if err != nil {
		log.Error("failed to open file", sl.Err(err))
		return
	}
	defer file.Close()

	body, err := json.Marshal(a.conf)
	if err != nil {
		log.Error("failed to marshal config", sl.Err(err))
		return
	}

	if _, err := file.Write(body); err != nil {
		log.Error("failed to write to file", sl.Err(err))
	}
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
