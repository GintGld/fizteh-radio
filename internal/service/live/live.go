package live

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/GintGld/fizteh-radio/internal/lib/ffmpeg"
	"github.com/GintGld/fizteh-radio/internal/lib/logger/sl"
	chans "github.com/GintGld/fizteh-radio/internal/lib/utils/channels"
	ptr "github.com/GintGld/fizteh-radio/internal/lib/utils/pointers"
	"github.com/GintGld/fizteh-radio/internal/lib/utils/writer"
	"github.com/GintGld/fizteh-radio/internal/models"
	"github.com/GintGld/fizteh-radio/internal/service"
)

const (
	waitBeforeDelete = 30 * time.Second
)

type Live struct {
	log          *slog.Logger
	timeout      time.Duration
	sch          Schedule
	delay        time.Duration
	stepDuration time.Duration
	sourceType   string
	source       string
	filters      map[string]string
	dir          string
	chunkLength  time.Duration

	cmd         *exec.Cmd
	errorWriter *writer.ByteWriter

	live     models.Live
	mutex    sync.Mutex
	stopChan chan struct{}
}

type Schedule interface {
	NewLive(ctx context.Context, live models.Live) (int64, error)
	SetLiveStop(ctx context.Context, live models.Live) error
	ScheduleCut(ctx context.Context, start time.Time, stop time.Time) ([]models.Segment, error)
	NewSegment(ctx context.Context, segment models.Segment) (int64, error)
	UpdateSegmentTiming(ctx context.Context, segment models.Segment) error
	DeleteSegment(ctx context.Context, id int64) error
	ClearSchedule(ctx context.Context, from time.Time) error
}

func New(
	log *slog.Logger,
	timeout time.Duration,
	sch Schedule,
	delay time.Duration,
	stepDuration time.Duration,
	sourceType string,
	source string,
	filters map[string]string,
	dir string,
	chunkLength time.Duration,
) *Live {
	return &Live{
		log:          log,
		timeout:      timeout,
		sch:          sch,
		delay:        delay,
		stepDuration: stepDuration,
		sourceType:   sourceType,
		source:       source,
		filters:      filters,
		dir:          dir,
		chunkLength:  chunkLength,

		mutex:    sync.Mutex{},
		stopChan: make(chan struct{}),
	}
}

// Start start live.
func (l *Live) Run(ctx context.Context, live models.Live) error {
	const op = "Live.StartLive"

	// Avoid multiple calls.
	if !l.mutex.TryLock() {
		return nil
	}
	defer l.mutex.Unlock()

	log := l.log.With(
		slog.String("op", op),
	)

	log.Info("starting live")

	// Determine live offset for correct mpd
	l.live = live
	l.live.Delay = l.delay
	l.live.Offset = time.Until(l.live.Start)
	if l.live.Delay > l.live.Offset {
		log.Warn(
			"delay is greater than offset, set offset as delay",
			slog.Float64("delay", l.live.Delay.Seconds()),
			slog.Float64("offset", l.live.Offset.Seconds()),
		)
		l.live.Start = time.Now().Add(l.live.Delay)
		l.live.Offset = l.live.Delay
	}

	// Register new live
	ctxLive, cancelLive := context.WithTimeout(ctx, l.timeout)
	defer cancelLive()
	if id, err := l.sch.NewLive(ctxLive, l.live); err != nil {
		if errors.Is(err, service.ErrTimeout) {
			log.Error("sch.NewLive timeout exceeded")
			return service.ErrTimeout
		}
		log.Error("failed to register live", sl.Err(err))
		return fmt.Errorf("%s: %w", op, err)
	} else {
		l.live.ID = id
	}

	log.Info("start live")

	// Reserve segment for live.
	reservedSegm := models.Segment{
		MediaID:   ptr.Ptr[int64](0),
		Start:     ptr.Ptr(l.live.Start),
		BeginCut:  ptr.Ptr[time.Duration](0),
		StopCut:   ptr.Ptr(2 * l.stepDuration),
		Protected: true,
		LiveId:    l.live.ID,
	}
	// Clear space for live.
	ctxClearSpace, cancelClearSpace := context.WithTimeout(ctx, l.timeout)
	defer cancelClearSpace()
	if err := l.clearSpace(ctxClearSpace, *reservedSegm.Start, reservedSegm.End()); err != nil {
		if errors.Is(err, service.ErrTimeout) {
			log.Error("clearSpace timeout exceeded")
			return service.ErrTimeout
		}
		log.Error("failed to clear space", sl.Err(err))
		return fmt.Errorf("%s: %w", op, err)
	}
	// Register segment.
	ctxNewSeg, cancelNewSeg := context.WithTimeout(ctx, l.timeout)
	defer cancelNewSeg()
	id, err := l.sch.NewSegment(ctxNewSeg, reservedSegm)
	if err != nil {
		if errors.Is(err, service.ErrSegmentIntersection) {
			log.Warn("intersecting protected segment, clearing failed")
			return fmt.Errorf("%s: %w", op, err)
		}
		if errors.Is(err, service.ErrTimeout) {
			log.Error("sch.NewSegment timeout exceeded")
			return service.ErrTimeout
		}
		log.Error("failed to create segment", sl.Err(err))
		return fmt.Errorf("%s: %w", op, err)

	}
	reservedSegm.ID = ptr.Ptr(id)

	// Conext for subroutines.
	ctxSub, cancel := context.WithCancel(ctx)
	errChan := make(chan error)
	// Start cmd
	go func() {
		if err := l.runCmd(ctxSub, id, errChan); err != nil {
			log.Error("live cmd returned error", sl.Err(err))
		}
	}()
	// Start cleanup.
	time.AfterFunc(
		time.Until(l.live.Start.Add(waitBeforeDelete)),
		func() { l.cleanup(ctxSub, id, 1) },
	)
	// Stop cmd at the end of this function.
	defer cancel()

	// In main loop
	// increase reserved segment
	// stopcut by fixed values.
main_loop:
	for {
		*reservedSegm.StopCut += l.stepDuration
		// Clear space for live.
		ctxClearSpace, cancelClearSpace := context.WithTimeout(ctx, l.timeout)
		defer cancelClearSpace()
		if err := l.clearSpace(ctxClearSpace, *reservedSegm.Start, reservedSegm.End()); err != nil {
			if errors.Is(err, service.ErrTimeout) {
				log.Error("clearSpace timeout exceeded")
				return service.ErrTimeout
			}
			log.Error("failed to clear space", sl.Err(err))
			return fmt.Errorf("%s: %w", op, err)
		}
		ctxUpdateTiming, cancelUpdateTiming := context.WithTimeout(ctx, l.timeout)
		defer cancelUpdateTiming()
		if err := l.sch.UpdateSegmentTiming(ctxUpdateTiming, reservedSegm); err != nil {
			if errors.Is(err, service.ErrTimeout) {
				log.Error("sch.UpdateSegmentTiming timeout exceeded")
				return service.ErrTimeout
			}
			log.Error("failed to change segment timing")
			return fmt.Errorf("%s: %w", op, err)
		}

		select {
		case <-time.After(l.stepDuration):
		case err := <-errChan:
			log.Error("cmd returned error, stop live.", sl.Err(err))
			break main_loop
		case <-l.stopChan:
			break main_loop
		case <-ctx.Done():
			break main_loop
		}
	}

	log.Info("stopping live")

	// Set live end
	l.live.Stop = time.Now()

	log.Debug("set live stop", slog.Time("stop", live.Stop))

	ctxLiveStop, cancelLiveStop := context.WithTimeout(ctx, l.timeout)
	defer cancelLiveStop()
	if err := l.sch.SetLiveStop(ctxLiveStop, l.live); err != nil {
		if errors.Is(err, service.ErrTimeout) {
			log.Error("sch.SetLiveStop timeout exceeded")
			return service.ErrTimeout
		}
		log.Error("failed to set live stop", slog.Int64("id", l.live.ID), sl.Err(err))
	}
	// Update live StopCut.
	*reservedSegm.StopCut = time.Since(l.live.Start)
	ctxUpdateTiming, cancelUpdateTiming := context.WithTimeout(ctx, l.timeout)
	defer cancelUpdateTiming()
	if err := l.sch.UpdateSegmentTiming(ctxUpdateTiming, reservedSegm); err != nil {
		if errors.Is(err, service.ErrTimeout) {
			log.Error("sch.UpdateSegmentTiming timeout exceeded")
			return service.ErrTimeout
		}
		log.Error("failed to change segment timing")
		return fmt.Errorf("%s: %w", op, err)
	}

	log.Info("stopped live")

	return nil
}

// runCmd runs cmd for ffmpeg recording source.
func (l *Live) runCmd(ctx context.Context, id int64, errChan chan<- error) (errRes error) {
	const op = "Live.runCmd"

	log := l.log.With(
		slog.String("op", op),
		slog.Int64("id", id),
	)

	// Send error if not nil.
	defer func() {
		if errRes != nil {
			errChan <- errRes
		}
	}()

	// Create dir for generated files
	dir := fmt.Sprintf("%s/%s", l.dir, ffmpeg.DirLive(id))
	if err := os.MkdirAll(dir, 0777); err != nil {
		log.Error("failed to create dir", sl.Err(err))
		errRes = fmt.Errorf("%s: %w", op, err)
		return
	}
	log.Debug("created dir", slog.String("", dir))

	const (
		bitrate      = "96k"
		channels     = "2"
		samplingRate = "44100"
	)
	durationString := strconv.FormatFloat(l.chunkLength.Seconds(), 'g', -1, 64)

	// Construct cmd.
	// Basic args.
	cmdArgs := []string{"-hide_banner", "-y", "-loglevel", "error"}
	// Source type if exists (e.g. "pulse", "alsa")
	if l.sourceType != "" {
		cmdArgs = append(cmdArgs, "-f", l.sourceType)
	}
	// Source (like "hw:1,0" for alsa or ip address)
	cmdArgs = append(cmdArgs, "-i", l.source)
	// Additional filters.
	if len(l.filters) != 0 {
		log.Debug("filter", slog.Any("", l.filters))

		// Format "<key>=<val>,<key>=<val>"
		s := make([]string, 0, len(l.filters))
		for k, v := range l.filters {
			s = append(s, fmt.Sprintf("%s=%s", k, v))
		}
		cmdArgs = append(cmdArgs, "-filter_complex", strings.Join(s, ","))
	}
	// Dash chunk settings
	cmdArgs = append(cmdArgs,
		"-c:a", "aac",
		"-b:a", bitrate,
		"-ac", channels,
		"-ar", samplingRate,
		"-dash_segment_type", "mp4",
		"-use_template", "1",
		"-use_timeline", "0",
		"-init_seg_name", ffmpeg.InitFileLive(id),
		"-media_seg_name", ffmpeg.ChunkFileLive(id),
		"-seg_duration", durationString,
		"-f", "dash",
		fmt.Sprintf("%s/%s", l.dir, "tmp.mpd"),
	)

	l.cmd = exec.CommandContext(ctx, "ffmpeg", cmdArgs...)

	log.Debug("setup live cmd", slog.String("cmd", l.cmd.String()))
	log.Info("start live cmd")

	l.errorWriter = writer.New()
	l.cmd.Stderr = l.errorWriter

	if err := l.cmd.Run(); err != nil {
		// Since cmd is being closed by context,
		// the correct shutdown returns error code -1
		// and "signal: killed" message.
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) && exitErr.ExitCode() == -1 && exitErr.String() == "signal: killed" {
			log.Debug("successfully killed process")
		} else {
			log.Error(
				"failed to run live cmd",
				slog.String("stderr", l.errorWriter.String()),
				sl.Err(err),
			)
			errRes = fmt.Errorf("%s: %w", op, err)
			return
		}
	}

	log.Info("stopped live cmd")
	log.Debug("stderr", slog.String("", l.errorWriter.String()))

	errRes = nil
	return
}

// clearSpace deletes protectected
// segments in given interval.
func (l *Live) clearSpace(ctx context.Context, start, stop time.Time) error {
	const op = "Live.clearSpace"

	log := l.log.With(
		slog.String("op", op),
	)

	res, err := l.sch.ScheduleCut(ctx, start, stop)
	if err != nil {
		if errors.Is(err, service.ErrTimeout) {
			log.Error("sch.ScheduleCut timeout exceeded")
			return service.ErrTimeout
		}
		log.Error("failed to get schedule cut", sl.Err(err))
		return fmt.Errorf("%s: %w", op, err)
	}

	for _, s := range res {
		if s.Protected && s.LiveId == 0 {
			if err := l.sch.DeleteSegment(ctx, *s.ID); err != nil {
				if errors.Is(err, service.ErrTimeout) {
					log.Error("sch.ScheduleCut timeout exceeded")
					return service.ErrTimeout
				}
				log.Error("failed to delete segment", slog.Int64("id", *s.ID), sl.Err(err))
				return fmt.Errorf("%s: %w", op, err)
			}
		}
	}

	return nil
}

func (l *Live) IsPlaying() bool {
	if l.mutex.TryLock() {
		l.mutex.Unlock()
		return false
	}
	return true
}

func (l *Live) Info() models.Live {
	if l.IsPlaying() {
		return l.live
	}
	return models.Live{}
}

func (l *Live) Stop() {
	if l.IsPlaying() {
		chans.Notify(l.stopChan)
	}
}

// cleanup deletes chunks one by one
// when ctx is cancelled deletes whole directory.
func (l *Live) cleanup(ctx context.Context, id int64, chunkId int) {
	const op = "Live.cleanup"

	log := l.log.With(
		slog.String("op", op),
		slog.Int64("id", id),
	)

	file := l.dir + "/" + ffmpeg.ChunkFileLiveCurrent(id, chunkId)
	if err := os.Remove(file); err != nil {
		log.Error("failed to delete file", slog.String("file", file), sl.Err(err))
	}

	select {
	case <-time.After(l.chunkLength):
		l.cleanup(ctx, id, chunkId+1)
	case <-ctx.Done():
		delTime := l.live.Stop.Add(waitBeforeDelete)
		log.Debug("ctx done, remove all chunks. wait until live ends", slog.Time("until", delTime))
		time.Sleep(time.Until(delTime))
		if err := os.RemoveAll(l.dir + "/" + ffmpeg.DirLive(id)); err != nil {
			log.Error("failed to clear dir", slog.Int64("id", id), sl.Err(err))
		}
		log.Debug("removed chunks")
	}
}
