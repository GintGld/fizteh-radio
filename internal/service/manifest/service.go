package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"strconv"
	"time"

	"github.com/zencoder/go-dash/v3/mpd"

	"github.com/GintGld/fizteh-radio/internal/lib/ffmpeg"
	"github.com/GintGld/fizteh-radio/internal/lib/logger/sl"
	ptr "github.com/GintGld/fizteh-radio/internal/lib/utils/pointers"
	"github.com/GintGld/fizteh-radio/internal/models"
)

const (
	scale = 1000
)

type Manifest struct {
	log          *slog.Logger
	live         Live
	path         string
	baseUrl      string
	startTime    time.Time
	chunkLength  time.Duration
	bufferDepth  time.Duration
	updatePeriod time.Duration

	man              *mpd.MPD
	lastPlayedPeriod int
}

type Live interface {
	Info() models.Live
}

// New returns new Manifest
func New(
	log *slog.Logger,
	live Live,
	path string,
	baseUrl string,
	startTime time.Time,
	chunkLength time.Duration,
	bufferTime time.Duration,
	bufferDepth time.Duration,
	updatePeriod time.Duration,
) *Manifest {
	// Cast special duration type
	bufferTimeMPD := mpd.Duration(bufferTime)
	updatePeriodMPD := mpd.Duration(updatePeriod)

	// Struct implements mpd structure
	man := mpd.NewDynamicMPD(
		mpd.DASH_PROFILE_LIVE,
		startTime.UTC().Format("2006-01-02T15:04:05"), // see https://ffmpeg.org/doxygen/trunk/dashdec_8c_source.html get_utc_date_time_insec
		bufferTimeMPD.String(),
		mpd.AttrMinimumUpdatePeriod(updatePeriodMPD.String()),
	)

	// Set buffer depth
	bufferDepthMPD := mpd.Duration(bufferDepth)
	man.TimeShiftBufferDepth = ptr.Ptr(bufferDepthMPD.String())

	// client synchronization
	man.UTCTiming.SchemeIDURI = ptr.Ptr("urn:mpeg:dash:utc:http-iso:2014")
	man.UTCTiming.Value = ptr.Ptr("https://time.akamai.com/?isoms")

	// Reset period array since it has uneccesary element
	man.Periods = nil

	return &Manifest{
		log:              log,
		live:             live,
		path:             path,
		baseUrl:          baseUrl,
		startTime:        startTime,
		chunkLength:      chunkLength,
		bufferDepth:      bufferDepth,
		updatePeriod:     updatePeriod,
		man:              man,
		lastPlayedPeriod: 0,
	}
}

// TODO: get meta information about segment (if needed)
// TODO: remove baseurl, since it does not work correctly (or fix it)

// SetSchedule updates schedule in Manifest
// if segments has intersection returns
// custom (temporary) error
func (m *Manifest) SetSchedule(_ context.Context, schedule []models.Segment) error {
	const op = "Manifest.SetSchedule"

	log := m.log.With(
		slog.String("op", op),
	)

	// update period indexing
	m.updateLastPlayedPeriod()

	// reset periods
	m.man.Periods = make([]*mpd.Period, len(schedule))

main_loop:
	for i, segment := range schedule {
		// Handle segment intersection.
		// That's no guarantee that client
		// won't play remained chunks.
		// Music stream may be raggy,
		// but nor server nor client won't crash.
		stopCut := *segment.StopCut // TODO: remove this variable after migrating from pointers
		if i < len(schedule)-1 {
			next := schedule[i+1]
			if segment.Start.Add(*segment.StopCut - *segment.BeginCut).After(*next.Start) {
				log.Warn(
					"segment intersection detected",
					slog.Time("curr end", segment.Start.Add(*segment.StopCut-*segment.BeginCut)),
					slog.Time("next start", *next.Start),
					slog.Float64("beginCut", segment.BeginCut.Seconds()),
					slog.Float64("stop", segment.StopCut.Seconds()),
				)
				stopCut = next.Start.Sub(*segment.Start)
				if stopCut <= 0 {
					log.Warn("stopcut became negative, don't put this segment into manifest")
					continue main_loop
				}
			}
		}

		var presentationShift uint64 = 0
		initFile := ffmpeg.InitFile(*segment.ID)
		chunkFile := ffmpeg.ChunkFile(*segment.ID)

		if segment.LiveId != 0 {
			live := m.live.Info()
			presentationShift = uint64(live.Offset - live.Delay)
			initFile = ffmpeg.InitFileLive(*segment.ID)
			chunkFile = ffmpeg.ChunkFileLive(*segment.ID)
		}

		m.man.Periods[i] = &mpd.Period{
			ID:       strconv.Itoa(i + 1 + m.lastPlayedPeriod),
			Duration: mpd.Duration(stopCut - *segment.BeginCut),
			Start:    ptr.Ptr(mpd.Duration(segment.Start.Sub(m.startTime))),
			// BaseURL:  []string{m.baseUrl},
			AdaptationSets: []*mpd.AdaptationSet{{
				ID:               ptr.Ptr("0"),
				ContentType:      ptr.Ptr("audio"),
				SegmentAlignment: ptr.Ptr(true),
				Representations: []*mpd.Representation{{
					ID:                ptr.Ptr("0"),
					AudioSamplingRate: ptr.Ptr[int64](44100),
					Bandwidth:         ptr.Ptr[int64](96000),
					Codecs:            ptr.Ptr("mp4a.40.2"),
					SegmentTemplate: &mpd.SegmentTemplate{
						StartNumber:            ptr.Ptr[int64](1),
						PresentationTimeOffset: ptr.Ptr(presentationShift),
						Initialization:         ptr.Ptr(initFile),
						Media:                  ptr.Ptr(chunkFile),
						Duration:               ptr.Ptr(m.chunkLength.Milliseconds()),
						Timescale:              ptr.Ptr[int64](scale),
					},
					CommonAttributesAndElements: mpd.CommonAttributesAndElements{
						MimeType: ptr.Ptr(mpd.DASH_MIME_TYPE_AUDIO_MP4),
					},
					AudioChannelConfiguration: &mpd.AudioChannelConfiguration{
						SchemeIDURI: ptr.Ptr("urn:mpeg:dash:23003:3:audio_channel_configuration:2011"),
						Value:       ptr.Ptr("2"),
					},
				}},
				CommonAttributesAndElements: mpd.CommonAttributesAndElements{
					StartWithSAP: ptr.Ptr[int64](1),
				},
			}},
		}
	}

	return nil
}

// updateLastPlayedPeriod updates Manifest.lastPlayedPeriod.
//
// Implements correct period indexing.
func (m *Manifest) updateLastPlayedPeriod() {
	const op = "Manifest.updateLastPlayedPeriod"

	log := m.log.With(
		slog.String("op", op),
	)

	if len(m.man.Periods) == 0 {
		return
	}

	now := time.Now()

	// no periods were played until now
	if now.Before(m.startTime.Add(time.Duration(*m.man.Periods[0].Start))) {
		return
	}

	for i, period := range m.man.Periods {
		if period.Start == nil {
			log.Warn("period start is nil", slog.Int("periodId", i), slog.String("period", fmt.Sprintf("%+v", period)))
		}
		periodStart := m.startTime.Add(time.Duration(*period.Start))
		periodEnd := periodStart.Add(time.Duration(period.Duration))

		// there is a period playing now
		if now.After(periodStart) && now.Before(periodEnd) {
			m.lastPlayedPeriod += i
			return
		}

		// there is no period playing now,
		// but there is at least one period in the future
		if now.Before(periodStart) {
			m.lastPlayedPeriod += i
			log.Warn(
				"found first period after now",
				slog.Int("lastPlayedPeriod", m.lastPlayedPeriod),
				slog.Int("idx", i),
			)
			return
		}
	}

	// all periods were played
	m.lastPlayedPeriod += len(m.man.Periods)
	log.Warn(
		"all periods were played",
		slog.Int("lastPlayedPeriod", m.lastPlayedPeriod),
	)
}

// Dump writes manifest to given path
func (m *Manifest) Dump() error {
	const op = "Manifest.Dump"

	log := m.log.With(
		slog.String("op", op),
	)

	if err := m.man.WriteToFile(m.path); err != nil {
		log.Error("filed to write manifest", sl.Err(err))
		return err
	}

	return nil
}

// CleanUp deletes manifest
func (m *Manifest) CleanUp() {
	const op = "Manifest.CleanUp"

	log := m.log.With(
		slog.String("op", op),
	)

	if err := os.Remove(m.path); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			log.Warn("mpd not exists")
		} else {
			log.Error("failed to delete mpd", sl.Err(err))
		}
	}
}
