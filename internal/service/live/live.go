package live

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"strconv"
	"sync"
	"syscall"
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
	waitBeforeDelete = 15 * time.Second
	cleanupFreq      = 15 * time.Second
)

type Live struct {
	log          *slog.Logger
	sch          Schedule
	delay        time.Duration
	stepDuration time.Duration
	scriptPath   string
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
	sch Schedule,
	delay time.Duration,
	stepDuration time.Duration,
	scriptPath string,
	dir string,
	chunkLength time.Duration,
) *Live {
	return &Live{
		log:          log,
		sch:          sch,
		delay:        delay,
		stepDuration: stepDuration,
		scriptPath:   scriptPath,
		dir:          dir,
		chunkLength:  chunkLength,

		mutex:    sync.Mutex{},
		stopChan: make(chan struct{}),
	}
}

// Start start live.
func (l *Live) Start(ctx context.Context, live models.Live) error {
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
	if id, err := l.sch.NewLive(ctx, l.live); err != nil {
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
		StopCut:   ptr.Ptr(l.stepDuration),
		Protected: true,
		LiveId:    l.live.ID,
	}
	// Clear space for live.
	if err := l.clearSpace(ctx, *reservedSegm.Start, reservedSegm.End()); err != nil {
		log.Error("failed to clear space", sl.Err(err))
		return fmt.Errorf("%s: %w", op, err)
	}
	// Register segment.
	id, err := l.sch.NewSegment(ctx, reservedSegm)
	if err != nil {
		if errors.Is(err, service.ErrSegmentIntersection) {
			log.Warn("intersecting protected segment, clearing failed")
			return fmt.Errorf("%s: %w", op, err)
		} else {
			log.Error("failed to create segment", sl.Err(err))
			if err := l.stopCmd(); err != nil {
				log.Error("failed to stop cmd", sl.Err(err))
			}
			return fmt.Errorf("%s: %w", op, err)
		}
	}
	reservedSegm.ID = ptr.Ptr(id)

	// Start cmd
	go func() {
		if err := l.runCmd(id); err != nil {
			log.Error("live cmd returned error", sl.Err(err))
		}
	}()
	// Stop cmd at the end of this function.
	defer func() {
		if err := l.stopCmd(); err != nil {
			log.Error("error in stopping live cmd", sl.Err(err))
		}
	}()

	// Setup cleanup.
	ctxCleanup, cleanupCancel := context.WithCancel(ctx)
	defer cleanupCancel()
	go func() {
		l.cleanup(ctxCleanup, id, 1)
	}()

	// Increase reserved segment stopcut
	// by fixed values.
main_loop:
	for {
		*reservedSegm.StopCut += l.stepDuration
		// Clear space for live.
		if err := l.clearSpace(ctx, *reservedSegm.Start, reservedSegm.End()); err != nil {
			log.Error("failed to clear space", sl.Err(err))
			return fmt.Errorf("%s: %w", op, err)
		}
		if err := l.sch.UpdateSegmentTiming(ctx, reservedSegm); err != nil {
			log.Error("failed to change segment timing")
			return fmt.Errorf("%s: %w", op, err)
		}

		select {
		case <-time.After(l.stepDuration):
		case <-l.stopChan:
			break main_loop
		case <-ctx.Done():
			break main_loop
		}
	}

	log.Info("stopping live")

	// Set live end
	live.Stop = time.Now()
	if err := l.sch.SetLiveStop(ctx, l.live); err != nil {
		log.Error("failed to set live stop", slog.Int64("id", l.live.ID), sl.Err(err))
	}
	// Update live StopCut.
	*reservedSegm.StopCut = time.Since(l.live.Start)
	if err := l.sch.UpdateSegmentTiming(ctx, reservedSegm); err != nil {
		log.Error("failed to change segment timing")
		return fmt.Errorf("%s: %w", op, err)
	}

	log.Info("stopped live")

	return nil
}

// runCmd runs cmd for ffmpeg catching source.
func (l *Live) runCmd(id int64) error {
	const op = "Live.runCmd"

	log := l.log.With(
		slog.String("op", op),
		slog.Int64("id", id),
	)

	dir := fmt.Sprintf("%s/%s", l.dir, ffmpeg.Dir(id))
	if err := os.MkdirAll(dir, 0777); err != nil {
		log.Error("failed to create dir")
	}

	l.cmd = exec.Command("bash", l.scriptPath)

	const (
		bitrate      = "96k"
		channels     = 2
		samplingRate = 44100
	)

	durationString := strconv.FormatFloat(l.chunkLength.Seconds(), 'g', -1, 64)

	l.cmd.Env = append(l.cmd.Environ(),
		fmt.Sprintf("BITRATE=%s", bitrate),
		fmt.Sprintf("CHANNELS=%d", channels),
		fmt.Sprintf("SAMPLING_RATE=%d", samplingRate),
		fmt.Sprintf("INIT_NAME=%s", ffmpeg.InitFile(id)),
		fmt.Sprintf("SEGMENT_NAME=%s", ffmpeg.ChunkFile(id)),
		fmt.Sprintf("SEGMENT_DURATION=%s", durationString),
		fmt.Sprintf("OUTPUT=%s", fmt.Sprintf("%s/%s", l.dir, "tmp.mpd")),
	)

	log.Debug("setup live cmd", slog.String("cmd", l.cmd.String()))
	log.Info("start live cmd")

	l.errorWriter = writer.New()
	l.cmd.Stderr = l.errorWriter

	if err := l.cmd.Run(); err != nil {
		log.Error(
			"failed to run live cmd",
			slog.String("stderr", l.errorWriter.String()),
			sl.Err(err),
		)
		return fmt.Errorf("%s: %w", op, err)
	}

	log.Info("stopped live cmd")

	return nil
}

// stopCmd stops cmd.
func (l *Live) stopCmd() error {
	const op = "Live.stopCmd"

	log := l.log.With(
		slog.String("op", op),
	)

	if l.cmd == nil {
		log.Warn("process is nil")
		return nil
	}

	if l.cmd.Process == nil {
		log.Warn("process is nil")
		return nil
	}

	if err := l.cmd.Process.Signal(syscall.SIGTERM); err != nil {
		log.Error("failed to send SIGTERM, trying to seng SIGKILL", sl.Err(err))
		if err := l.cmd.Process.Signal(syscall.SIGKILL); err != nil {
			log.Error("failed to send SIGKILL", sl.Err(err))
			return fmt.Errorf("%s: %w", op, err)
		}
	}

	if err := l.cmd.Wait(); err != nil {
		log.Error("failed to wait process stop", sl.Err(err))
		return fmt.Errorf("%s: %w", op, err)
	}

	if l.cmd.ProcessState == nil {
		log.Warn("process state is nil")
		return nil
	}

	if !l.cmd.ProcessState.Success() {
		log.Error(
			"cmd ended with error",
			slog.Int("code", l.cmd.ProcessState.ExitCode()),
			slog.String("proc. State", l.cmd.ProcessState.String()),
		)
	}

	return nil
}

// clearSpace deletes protectected
// segments in given interval.
func (l *Live) clearSpace(ctx context.Context, start, stop time.Time) error {
	const op = "Live.clearSpacectx, *reservedSegm.Start, *reservedSegm.End()"

	log := l.log.With(
		slog.String("op", op),
	)

	res, err := l.sch.ScheduleCut(ctx, start, stop)
	if err != nil {
		log.Error("failed to get schedule cut", sl.Err(err))
		return fmt.Errorf("%s: %w", op, err)
	}

	for _, s := range res {
		if s.Protected && s.LiveId == 0 {
			if err := l.sch.DeleteSegment(ctx, *s.ID); err != nil {
				log.Error("failed to delere segment", slog.Int64("id", *s.ID), sl.Err(err))
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
	return l.live
}

func (l *Live) Stop() {
	if l.IsPlaying() {
		chans.Notify(l.stopChan)
	}
}

// cleanup deletes segment's directory
// with all chunk,produces by live.
func (l *Live) cleanup(ctx context.Context, id int64, chunkId int) {
	const op = "Live.cleanup"

	log := l.log.With(
		slog.String("op", op),
		slog.Int64("id", id),
	)

	// Time pasted from the first recorded segment
	timeSpent := time.Since(l.live.Start) - l.live.Offset
	maxId := int(timeSpent / l.chunkLength)

	log.Debug("delete chunks", slog.Int("from", chunkId), slog.Int("to", maxId-1))

	for i := chunkId; i < maxId; i++ {
		if err := os.Remove(l.dir + "/" + ffmpeg.ChunkFileCurrent(id, i)); err != nil {
			log.Error("failed to delete file", slog.String("file", l.dir+"/"+ffmpeg.ChunkFileCurrent(id, i)), sl.Err(err))
		}
	}

	if maxId < chunkId {
		maxId = chunkId
	}

	select {
	case <-time.After(cleanupFreq):
		l.cleanup(ctx, id, maxId)
	case <-ctx.Done():
		delTime := l.live.Stop.Add(waitBeforeDelete)
		log.Debug("ctx done, remove all chunks. wait until live ends", slog.Time("until", delTime))
		time.Sleep(time.Until(delTime))
		if err := os.RemoveAll(l.dir + "/" + ffmpeg.Dir(id)); err != nil {
			log.Error("failed to clear dir", slog.Int64("id", id), sl.Err(err))
		}
		log.Debug("removed chunks")
	}
}
