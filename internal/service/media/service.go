package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"slices"

	"github.com/GintGld/fizteh-radio/internal/lib/logger/sl"
	"github.com/GintGld/fizteh-radio/internal/models"
	"github.com/GintGld/fizteh-radio/internal/service"
	"github.com/GintGld/fizteh-radio/internal/storage"
)

type Media struct {
	log          *slog.Logger
	mediaStorage MediaStorage
	tagTypes     models.TagTypes
}

type MediaStorage interface {
	AllMedia(ctx context.Context) ([]models.Media, error)
	SaveMedia(ctx context.Context, newMedia models.Media) (int64, error)
	UpdateMediaBasicInfo(ctx context.Context, media models.Media) error
	Media(ctx context.Context, id int64) (models.Media, error)
	DeleteMedia(ctx context.Context, id int64) error
	TagTypes(ctx context.Context) (models.TagTypes, error)
	AllTags(ctx context.Context) (models.TagList, error)
	SaveTag(ctx context.Context, tag models.Tag) (int64, error)
	Tag(ctx context.Context, id int64) (models.Tag, error)
	DeleteTag(ctx context.Context, id int64) error
	TagMedia(ctx context.Context, mediaId int64, tags ...models.Tag) error
	MultiTagMedia(ctx context.Context, tag models.Tag, mediaIds ...int64) error
	UntagMedia(ctx context.Context, mediaId int64, tags ...models.Tag) error
}

func New(
	log *slog.Logger,
	mediaStorage MediaStorage,
) *Media {
	const op = "Media.New"

	localLog := log.With(
		slog.String("op", op),
		slog.String("editorname", models.RootLogin),
	)

	tagTypes, err := mediaStorage.TagTypes(context.Background())
	if err != nil {
		localLog.Error("failed to get tag types", sl.Err(err))
		return nil
	}

	return &Media{
		log:          log,
		mediaStorage: mediaStorage,
		tagTypes:     tagTypes,
	}
}

// TODO: in logging save editor name (put in context)
// TODO: searching filters
// TODO: autodj

func (l *Media) AllMedia(ctx context.Context) ([]models.Media, error) {
	const op = "Media.AllMedia"

	log := l.log.With(
		slog.String("op", op),
		slog.String("editorname", models.RootLogin),
	)

	log.Info("getting all media")

	media, err := l.mediaStorage.AllMedia(ctx)
	if err != nil {
		if errors.Is(err, storage.ErrMediaNotFound) {
			log.Warn("media not found")
			return []models.Media{}, service.ErrMediaNotFound
		}
		log.Error("failed to get media", sl.Err(err))
		return []models.Media{}, err
	}

	log.Info("found media")

	return media, nil
}

// NewMedia registers new editor in the system and returns media ID.
func (l *Media) NewMedia(ctx context.Context, media models.Media) (int64, error) {
	const op = "Media.NewMedia"

	log := l.log.With(
		slog.String("op", op),
		slog.String("editorname", models.RootLogin),
	)

	log.Info("registering new media")

	id, err := l.mediaStorage.SaveMedia(ctx, media)
	if err != nil {
		log.Error("failed to save media", sl.Err(err))
		return 0, fmt.Errorf("%s: %w", op, err)
	}

	tags, _ := l.mediaStorage.AllTags(ctx)

	for _, tag := range media.Tags {
		if !slices.Contains(tags, tag) {
			log.Warn("tag not found", slog.Int64("id", tag.ID))
			return 0, service.ErrTagNotFound
		}
	}

	if err := l.mediaStorage.TagMedia(ctx, id, media.Tags...); err != nil {
		log.Error("failed to tag media", sl.Err(err))
		return 0, fmt.Errorf("%s: %w", op, err)
	}

	log.Info(
		"registered media",
		slog.Int64("id", id),
		slog.String("name", *media.Name),
		slog.String("author", *media.Author),
		slog.Int64("sourceID", *media.SourceID),
	)

	return id, nil
}

// UpdateMedia saves new media information.
// If there's no media with given id, returns error.
func (l *Media) UpdateMedia(ctx context.Context, media models.Media) error {
	const op = "Media.UpdateMedia"

	log := l.log.With(
		slog.String("op", op),
		slog.String("editorname", models.RootLogin),
	)

	log.Info("updating media", slog.Int64("id", *media.ID))

	oldMedia, err := l.mediaStorage.Media(ctx, *media.ID)
	if err != nil {
		if errors.Is(err, storage.ErrMediaNotFound) {
			log.Warn("media not found", slog.Int64("id", *media.ID))
			return service.ErrMediaNotFound
		}
		log.Error(
			"failed to get old media",
			slog.Int64("id", *media.ID),
			sl.Err(err),
		)
		return fmt.Errorf("%s: %w", op, err)
	}

	log.Info("found old media", slog.Int64("id", *media.ID))

	if err := l.mediaStorage.UpdateMediaBasicInfo(ctx, media); err != nil {
		log.Error(
			"failed to update basic media info",
			slog.Int64("id", *media.ID),
			sl.Err(err),
		)
		return fmt.Errorf("%s: %w", op, err)
	}

	log.Info("updated basic info")

	log.Debug("new info",
		slog.String("name", *media.Name),
		slog.String("author", *media.Author))

	tags, _ := l.mediaStorage.AllTags(ctx)

	tagsToAdd := make(models.TagList, 0)
	tagsToDel := make(models.TagList, 0)

	for _, newTag := range media.Tags {
		if !slices.Contains(tags, newTag) {
			log.Warn("tag not found", slog.Int64("id", newTag.ID))
		}
		if !slices.Contains(oldMedia.Tags, newTag) {
			tagsToAdd = append(tagsToAdd, newTag)
		}
	}
	for _, oldTag := range oldMedia.Tags {
		if !slices.Contains(media.Tags, oldTag) {
			tagsToDel = append(tagsToDel, oldTag)
		}
	}

	l.mediaStorage.TagMedia(ctx, *media.ID, tagsToAdd...)
	l.mediaStorage.UntagMedia(ctx, *media.ID, tagsToDel...)

	return nil
}

// MultiTagMedia adds tag to media list.
func (l *Media) MultiTagMedia(ctx context.Context, tag models.Tag, mediaIds ...int64) error {
	const op = "Media.MultiTagMedia"

	log := l.log.With(
		slog.String("op", op),
		slog.String("editorname", models.RootLogin),
	)

	log.Info("adding tag to several media", slog.Int64("tag id", tag.ID))

	tags, _ := l.mediaStorage.AllTags(ctx)

	if !slices.Contains(tags, tag) {
		log.Warn("tag not found", slog.Int64("tag id", tag.ID))
		return service.ErrTagNotFound
	}

	if err := l.mediaStorage.MultiTagMedia(ctx, tag, mediaIds...); err != nil {
		log.Error("failed to tag media list", slog.Int64("tag id", tag.ID), sl.Err(err))
		return fmt.Errorf("%s: %w", op, err)
	}

	return nil
}

// Media returns media model by given id.
//
// If media with given id does not exist, returns error.
func (l *Media) Media(ctx context.Context, id int64) (models.Media, error) {
	const op = "Media.Media"

	log := l.log.With(
		slog.String("op", op),
		slog.String("editorname", models.RootLogin),
	)

	log.Info("getting media")

	media, err := l.mediaStorage.Media(ctx, id)
	if err != nil {
		if errors.Is(err, storage.ErrMediaNotFound) {
			log.Warn("media not found", slog.Int64("id", id))
			return models.Media{}, service.ErrMediaNotFound
		}
		log.Error("failed to get media", slog.Int64("id", id), sl.Err(err))
		return models.Media{}, err
	}

	log.Info("found media", slog.Int64("id", id))

	return media, nil
}

// DeleteMedia deletes media.
//
// If media with given id does not exist, returns error.
func (l *Media) DeleteMedia(ctx context.Context, id int64) error {
	const op = "Media.DeleteEditor"

	log := l.log.With(
		slog.String("op", op),
		slog.String("editorname", models.RootLogin),
	)

	log.Info("deleting media", slog.Int64("id", id))

	if err := l.mediaStorage.DeleteMedia(ctx, id); err != nil {
		if errors.Is(err, storage.ErrMediaNotFound) {
			log.Warn("media not found", slog.Int64("id", id))
			return fmt.Errorf("%s: %w", op, service.ErrMediaNotFound)
		}
		log.Error("failed to delete media", slog.Int64("id", id))
		return fmt.Errorf("%s: %w", op, err)
	}

	return nil
}

// TagTypes returns available tag types.
func (l *Media) TagTypes(ctx context.Context) (models.TagTypes, error) {
	const op = "Media.TagTypes"

	log := l.log.With(
		slog.String("op", op),
		slog.String("editorname", models.RootLogin),
	)

	tagTypes, err := l.mediaStorage.TagTypes(ctx)
	if err != nil {
		log.Error("failed to get tag types", sl.Err(err))
		return models.TagTypes{}, err
	}

	log.Info("got tag types")

	return tagTypes, nil
}

// AllTags return all registered tags.
func (l *Media) AllTags(ctx context.Context) (models.TagList, error) {
	const op = "Media.AllTags"

	log := l.log.With(
		slog.String("op", op),
		slog.String("editorname", models.RootLogin),
	)

	tagList, err := l.mediaStorage.AllTags(ctx)
	if err != nil {
		log.Error("failed to get tag list", sl.Err(err))
	}

	log.Info("got all tags")

	return tagList, nil
}

// SaveTag registers new tag.
func (l *Media) SaveTag(ctx context.Context, tag models.Tag) (int64, error) {
	const op = "Media.SaveTag"

	log := l.log.With(
		slog.String("op", op),
		slog.String("editorname", models.RootLogin),
	)

	log.Info("saving tag", slog.String("name", tag.Name))

	// Validating tag type.
	validType := false
	for _, tagType := range l.tagTypes {
		if tagType.ID == tag.Type.ID {
			validType = true
		}
	}
	if !validType {
		log.Warn("tag type not found", slog.Int64("id", tag.Type.ID))
		return 0, service.ErrTagTypeNotFound
	}

	log.Info("tag type valid", slog.Int64("id", tag.Type.ID))

	id, err := l.mediaStorage.SaveTag(ctx, tag)
	if err != nil {
		if errors.Is(err, storage.ErrTagExists) {
			log.Warn("tag exists", slog.String("name", tag.Name))
			return 0, service.ErrTagExists
		}
		log.Error(
			"failed to register tag",
			slog.String("name", tag.Name),
			sl.Err(err),
		)
		return 0, fmt.Errorf("%s: %w", op, err)
	}

	log.Info("got all tags")

	return id, nil
}

// Tag return Tag by its id.
func (l *Media) Tag(ctx context.Context, id int64) (models.Tag, error) {
	const op = "Media.Tag"

	log := l.log.With(
		slog.String("op", op),
		slog.String("editorname", models.RootLogin),
	)

	log.Info("getting tag", slog.Int64("id", id))

	tag, err := l.mediaStorage.Tag(ctx, id)
	if err != nil {
		if errors.Is(err, storage.ErrTagNotFound) {
			log.Warn("tag not found", slog.Int64("id", id))
			return models.Tag{}, service.ErrTagNotFound
		}
		log.Error("failed to get tag", slog.Int64("id", id), sl.Err(err))
		return models.Tag{}, fmt.Errorf("%s: %w", op, err)
	}

	for _, ttype := range l.tagTypes {
		if ttype.ID == tag.Type.ID {
			tag.Type.Name = ttype.Name
			return tag, nil
		}
	}

	return models.Tag{}, service.ErrTagTypeNotFound
}

func (l *Media) DeleteTag(ctx context.Context, id int64) error {
	const op = "Media.DeleteTag"

	log := l.log.With(
		slog.String("op", op),
		slog.String("editorname", models.RootLogin),
	)

	log.Info("deleting tag", slog.Int64("id", id))

	if err := l.mediaStorage.DeleteTag(ctx, id); err != nil {
		if errors.Is(err, storage.ErrTagNotFound) {
			log.Warn("tag not found", slog.Int64("id", id))
			return fmt.Errorf("%s: %w", op, service.ErrTagNotFound)
		}
		log.Error("failed to delete media", slog.Int64("id", id))
		return fmt.Errorf("%s: %w", op, err)
	}

	return nil
}
