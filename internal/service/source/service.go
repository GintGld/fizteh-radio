package service

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"strconv"
	"time"

	"github.com/GintGld/fizteh-radio/internal/lib/ffmpeg"
	"github.com/GintGld/fizteh-radio/internal/lib/logger/sl"
	ptr "github.com/GintGld/fizteh-radio/internal/lib/utils/pointers"
	"github.com/GintGld/fizteh-radio/internal/models"
)

type Client interface {
	Upload(ctx context.Context, r io.Reader) (int, error)
	Download(ctx context.Context, id int, dst string) error
	Delete(ctx context.Context, id int) error
}

type Source struct {
	log    *slog.Logger
	client Client
}

func New(
	log *slog.Logger,
	client Client,
) *Source {
	return &Source{
		log:    log,
		client: client,
	}
}

// UploadSource moves source to given directory.
//
// After uploading media.SourceID and media.Duration
// will be fulfilled (must be undefined when calling function).
func (s *Source) UploadSource(ctx context.Context, path string, media *models.Media) error {
	const op = "Source.UploadSource"

	log := s.log.With(slog.String("op", op), slog.String("editorname", models.RootLogin))

	if media.SourceID != nil {
		log.Error("media source id already set")
		return fmt.Errorf("%s: media source id already set", op)
	}
	if media.Duration != nil {
		log.Error("media duration already set")
		return fmt.Errorf("%s: media duration already set", op)
	}

	// Open file to send.
	source, err := os.Open(path)
	if err != nil {
		log.Error("failed to open input file", slog.String("file", path), sl.Err(err))
		return err
	}
	defer source.Close()

	// Upload data.
	sourceID, err := s.client.Upload(ctx, source)
	if err != nil {
		log.Error("failed to send data", sl.Err(err))
		return fmt.Errorf("%s: %w", op, err)
	}
	media.SourceID = ptr.Ptr(int64(sourceID))

	// Get metadata.
	durationString, err := ffmpeg.GetMeta(&path, "duration")
	if err != nil {
		log.Error("failed to get source duration", slog.String("file", path), sl.Err(err))
		return fmt.Errorf("%s: %w", op, err)
	}
	durationSec, err := strconv.ParseFloat(durationString, 64)
	if err != nil {
		log.Error("got invalid duration from metadata", slog.String("value", durationString), sl.Err(err))
		return fmt.Errorf("%s: %w", op, err)
	}
	media.Duration = ptr.Ptr(time.Microsecond * time.Duration(durationSec*1000000))

	return nil
}

// LoadSource moves source file related to media
// to destDir.
func (s *Source) LoadSource(ctx context.Context, destDir string, media models.Media) (string, error) {
	const op = "Source.LoadSource"

	log := s.log.With(slog.String("op", op), slog.String("editorname", models.RootLogin))

	if media.SourceID == nil {
		log.Error("media source is not defined")
		return "", fmt.Errorf("%s: media source is not defined", op)
	}

	file := fmt.Sprintf("%s/%d.mp3", destDir, *media.SourceID)

	// Download file.
	if err := s.client.Download(ctx, int(*media.SourceID), file); err != nil {
		log.Error("failed to download file", slog.String("dst", destDir), slog.Int64("id", *media.SourceID), sl.Err(err))
		return "", fmt.Errorf("%s: %w", op, err)
	}

	return file, nil
}

// DeleteSource deletes source related to given media.
func (s *Source) DeleteSource(ctx context.Context, media models.Media) error {
	const op = "Source.DeleteSource"

	log := s.log.With(
		slog.String("op", op),
		slog.String("editorname", models.RootLogin),
	)

	if media.SourceID == nil {
		log.Error("media source is not defined")
		return fmt.Errorf("%s: media source is not defined", op)
	}
	sourceID := *media.SourceID

	if err := s.client.Delete(ctx, int(sourceID)); err != nil {
		log.Error("failed to delete file", slog.Int64("id", sourceID), sl.Err(err))
		return fmt.Errorf("%s: %w", op, err)
	}

	return nil
}
