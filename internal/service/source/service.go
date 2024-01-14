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
	"strings"
	"time"

	"github.com/GintGld/fizteh-radio/internal/lib/ffmpeg"
	"github.com/GintGld/fizteh-radio/internal/lib/logger/sl"
	ptr "github.com/GintGld/fizteh-radio/internal/lib/utils/pointers"
	"github.com/GintGld/fizteh-radio/internal/models"
)

type Source struct {
	log          *slog.Logger
	dir          string
	nestingDepth int

	idLength int
	maxId    int
}

func New(
	log *slog.Logger,
	dir string,
	nestingDepth int,
	idLength int,
) *Source {
	N := 1
	for i := 0; i < idLength; i++ {
		N *= 10
	}

	source := &Source{
		log:          log,
		dir:          dir,
		nestingDepth: nestingDepth,
		idLength:     idLength,
		maxId:        N,
	}

	source.mustInitFilesystem()

	return source
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

	sourceID, err := s.generateNewID()
	if err != nil {
		log.Error(
			"failed to generate new id",
			sl.Err(err),
		)
		return fmt.Errorf("%s: %w", op, err)
	}

	dir, err := s.getCorrespondingDir(sourceID)
	if err != nil {
		log.Error(
			"failed to get source dir",
			slog.Int64("id", sourceID),
			sl.Err(err),
		)
	}
	fileName := dir + "/" + strconv.FormatInt(sourceID, 10) + ".mp3"

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

	log.Info("uploaded source", slog.Int64("sourceID", sourceID))

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

	dir, err := s.getCorrespondingDir(*media.SourceID)
	if err != nil {
		log.Error(
			"failed to get source dir",
			slog.Int64("id", *media.SourceID),
			sl.Err(err),
		)
	}

	fileName := dir + "/" + strconv.Itoa(int(*media.SourceID)) + ".mp3"
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

	dir, err := s.getCorrespondingDir(sourceID)
	if err != nil {
		log.Error(
			"failed to get source dir",
			slog.Int64("id", sourceID),
			sl.Err(err),
		)
	}

	fileName := dir + "/" + strconv.Itoa(int(sourceID)) + ".mp3"

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

// initFileSystem inits file system.
// Creates necessary directories.
//
// Panics if occurs error.
func (s *Source) mustInitFilesystem() {
	const op = "Source.mustInitFileSystem"

	log := s.log.With(
		slog.String("op", op),
		slog.String("editorname", models.RootLogin),
	)

	// indexing directories
	N := 1
	for i := 0; i < s.nestingDepth; i++ {
		N *= 10
	}

	splitted := make([]string, s.nestingDepth)
	for i := 0; i < N; i++ {
		str := strconv.Itoa(i)

		for j := 0; j < s.nestingDepth-len(str); j++ {
			splitted[j] = "0"
		}
		for j := s.nestingDepth - len(str); j < s.nestingDepth; j++ {
			splitted[j] = string(str[j-s.nestingDepth+len(str)])
		}

		dir := s.dir + "/" + strings.Join(splitted, "/")

		if err := os.MkdirAll(dir, 0777); err != nil {
			log.Error(
				"failed to create dir",
				slog.String("dir", dir),
				sl.Err(err),
			)
			panic("failed to create dir")
		}
	}
}

// generateNewID generates new unique id
func (s *Source) generateNewID() (int64, error) {
	const op = "Source.generateNewID"

	log := s.log.With(
		slog.String("op", op),
		slog.String("editorname", models.RootLogin),
	)

	for {
		sourceID := int64(rand.Int63n(int64(s.maxId)))

		exists, err := s.checkExistingID(sourceID)
		if err != nil {
			log.Error(
				"failed to check id",
				slog.Int64("id", sourceID),
				sl.Err(err),
			)
			return 0, fmt.Errorf("%s: %w", op, err)
		}

		if !exists {
			return sourceID, nil
		}
	}
}

// getCorrespondingDir returns path,
// where source with given id should be placed.
func (s *Source) getCorrespondingDir(id int64) (string, error) {
	const op = "Source.getCorrespondingDir"

	log := s.log.With(
		slog.String("op", op),
		slog.String("editorname", models.RootLogin),
	)

	if id < 0 {
		log.Warn("invalid media source id", slog.Int64("id", id))
		return "", fmt.Errorf("%s: invalid media source id", op)
	}

	str := strconv.FormatInt(id, 10)

	if len(str) > s.idLength {
		log.Warn("invalid media source id", slog.Int64("id", id))
		return "", fmt.Errorf("%s: invalid media source id", op)
	}

	splitted := make([]string, s.nestingDepth)

	for j := 0; j < s.idLength-len(str); j++ {
		splitted[j] = "0"
	}
	for j := s.idLength - len(str); j < s.nestingDepth; j++ {
		splitted[j] = string(str[j-s.idLength+len(str)])
	}

	return s.dir + "/" + strings.Join(splitted, "/"), nil
}

func (s *Source) checkExistingID(id int64) (bool, error) {
	const op = "Source.checkExistingID"

	log := s.log.With(
		slog.String("op", op),
		slog.String("editorname", models.RootLogin),
	)

	dir, err := s.getCorrespondingDir(id)
	if err != nil {
		log.Error(
			"failed to get dir",
			slog.Int64("id", id),
			sl.Err(err),
		)
		return false, fmt.Errorf("%s: %w", op, err)
	}

	file := dir + "/" + strconv.FormatInt(id, 10) + ".mp3"

	if _, err := os.Stat(file); err == nil {
		return true, nil
	} else if errors.Is(err, os.ErrNotExist) {
		return false, nil
	} else {
		log.Error(
			"failed to probe file",
			slog.String("file", file),
			sl.Err(err),
		)
		return false, fmt.Errorf("%s: %w", op, err)
	}
}
