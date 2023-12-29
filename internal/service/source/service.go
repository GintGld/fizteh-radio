package service

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"math/rand"
	"os"
	"strconv"
	"time"

	"github.com/GintGld/fizteh-radio/internal/lib/ffmpeg"
	"github.com/GintGld/fizteh-radio/internal/lib/logger/sl"
	ptr "github.com/GintGld/fizteh-radio/internal/lib/utils/pointers"
	"github.com/GintGld/fizteh-radio/internal/models"
)

type Source struct {
	log *slog.Logger
	dir string
}

func New(
	log *slog.Logger,
	dir string,
) *Source {
	return &Source{
		log: log,
		dir: dir,
	}
}

// UploadSource moves source to given directory.
//
// After uploading media.SourceID and media.Duration
// will be fulfilled (must be undefined when calling function).
func (s *Source) UploadSource(ctx context.Context, path string, media *models.Media) error {
	const op = "Source.UploadSource"

	log := s.log.With(
		slog.String("op", op),
		slog.String("editorname", models.RootLogin),
	)

	log.Info("uploading source")

	if media.SourceID != nil {
		log.Error("media source id already set")
		return fmt.Errorf("%s: media source id already set", op)
	}
	if media.Duration != nil {
		log.Error("media duration already set")
		return fmt.Errorf("%s: media duration already set", op)
	}

	source, err := os.Open(path)
	if err != nil {
		log.Error("failed to open input file", slog.String("file", path), sl.Err(err))
		return err
	}
	defer source.Close()

	sourceID := rand.Int()
	fileName := s.dir + "/" + strconv.Itoa(sourceID) + ".mp3"

	destination, err := os.Create(fileName)
	if err != nil {
		log.Error("failed to create file", slog.String("file", fileName), sl.Err(err))
		return err
	}
	defer destination.Close()

	if _, err = io.Copy(destination, source); err != nil {
		log.Error("failed to copy file", sl.Err(err))
		return err
	}

	log.Info("uploaded source", slog.Int("sourceID", sourceID))

	media.SourceID = ptr.Ptr(int64(sourceID))

	durationString, err := ffmpeg.GetMeta(&fileName, "duration")
	if err != nil {
		log.Error(
			"failed to get source duration",
			slog.String("file", fileName),
			sl.Err(err),
		)
		return fmt.Errorf("%s: %w", op, err)
	}
	durationSec, err := strconv.ParseFloat(durationString, 64)
	if err != nil {
		log.Error(
			"got invalid duration from metadata",
			slog.String("value", durationString),
			sl.Err(err),
		)
		return fmt.Errorf("%s: %w", op, err)
	}
	media.Duration = ptr.Ptr(time.Second * time.Duration(durationSec))

	return nil
}

// LoadSource moves source file related to media
// to destDir.
func (s *Source) LoadSource(ctx context.Context, destDir string, media models.Media) (string, error) {
	const op = "Source.LoadSource"

	log := s.log.With(
		slog.String("op", op),
		slog.String("editorname", models.RootLogin),
	)

	if media.SourceID == nil {
		log.Error("media source is not defined")
		return "", fmt.Errorf("%s: media source is not defined", op)
	}

	log.Info("loading source", slog.Int64("sourceID", *media.SourceID))

	fileName := s.dir + "/" + strconv.Itoa(int(*media.SourceID)) + ".mp3"
	destName := destDir + "/" + strconv.Itoa(int(*media.SourceID)) + ".mp3"

	source, err := os.Open(fileName)
	if err != nil {
		log.Error("failed to open file", slog.String("file", fileName), sl.Err(err))
		return "", err
	}
	defer source.Close()

	destination, err := os.Create(destName)
	if err != nil {
		log.Error("failed to create file", slog.String("file", fileName), sl.Err(err))
		return "", err
	}
	defer destination.Close()

	if _, err = io.Copy(destination, source); err != nil {
		log.Error("failed to copy file", sl.Err(err))
		return "", err
	}

	log.Info("loaded source", slog.Int64("sourceID", *media.SourceID))

	return destName, nil
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

	fileName := s.dir + "/" + strconv.Itoa(int(sourceID)) + ".mp3"

	log.Info("deleting source", slog.Int64("sourceID", sourceID))

	if _, err := os.Stat(fileName); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			log.Warn("source does not exist", slog.Int64("sourceID", sourceID))
			return fmt.Errorf("%s: source does not exist", op)
		}
		log.Error("failed to find source file", slog.Int64("sourceID", sourceID), sl.Err(err))
	}

	if err := os.Remove(fileName); err != nil {
		log.Error("failed to delete source file", slog.Int64("sourceID", sourceID), sl.Err(err))
	}

	log.Info("deleted source", slog.Int64("sourceID", sourceID))

	return nil
}
