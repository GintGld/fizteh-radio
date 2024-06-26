package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"slices"

	"github.com/GintGld/fizteh-radio/internal/lib/logger/sl"
	chans "github.com/GintGld/fizteh-radio/internal/lib/utils/channels"
	"github.com/GintGld/fizteh-radio/internal/models"
	"github.com/GintGld/fizteh-radio/internal/service"
	"github.com/GintGld/fizteh-radio/internal/storage"
)

type Media struct {
	log             *slog.Logger
	mediaStorage    MediaStorage
	tagTypes        models.TagTypes
	maxAnswerLength int

	updateChan chan<- struct{}
}

type MediaStorage interface {
	// Media
	AllMedia(ctx context.Context, limit, offset int) ([]models.Media, error)
	SaveMedia(ctx context.Context, newMedia models.Media) (int64, error)
	UpdateMediaBasicInfo(ctx context.Context, media models.Media) error
	Media(ctx context.Context, id int64) (models.Media, error)
	MediaTags(ctx context.Context, id int64) (models.TagList, error)
	DeleteMedia(ctx context.Context, id int64) error

	// Tag managment
	TagTypes(ctx context.Context) (models.TagTypes, error)
	AllTags(ctx context.Context) (models.TagList, error)
	SaveTag(ctx context.Context, tag models.Tag) (int64, error)
	UpdateTag(ctx context.Context, tag models.Tag) error
	Tag(ctx context.Context, id int64) (models.Tag, error)
	DeleteTag(ctx context.Context, id int64) error
	TagMedia(ctx context.Context, mediaId int64, tags ...models.Tag) error
	MultiTagMedia(ctx context.Context, tag models.Tag, mediaIds ...int64) error
	UntagMedia(ctx context.Context, mediaId int64, tags ...models.Tag) error

	// Tag meta information
	SetTagMeta(ctx context.Context, tag models.Tag, key, val string) error
	TagMeta(ctx context.Context, tag models.Tag) (map[string]string, error)
	DelTagMeta(ctx context.Context, tag models.Tag, key string) error
}

func New(
	log *slog.Logger,
	mediaStorage MediaStorage,
	maxAnswerLength int,
	updateChan chan<- struct{},
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
		log:             log,
		mediaStorage:    mediaStorage,
		tagTypes:        tagTypes,
		maxAnswerLength: maxAnswerLength,
		updateChan:      updateChan,
	}
}

// TODO: in logging save editor name (put in context)

func (l *Media) SearchMedia(ctx context.Context, filter models.MediaFilter) ([]models.Media, error) {
	const op = "Media.SearchMedia"

	log := l.log.With(
		slog.String("op", op),
		slog.String("editorname", models.RootLogin),
	)

	log.Info(
		"start search",
		slog.String("name", filter.Name),
		slog.String("author", filter.Author),
		slog.Any("tags", filter.Tags),
	)

	bestRes := make([]mediaRank, 0)

main_loop:
	for offset := 0; ; offset += l.maxAnswerLength {
		// Get new media chunk
		newSlice, err := l.mediaStorage.AllMedia(ctx, l.maxAnswerLength, offset)
		if err != nil {
			if errors.Is(err, storage.ErrContextCancelled) {
				log.Error("mediaStorage.AllMedia timeout exceeded")
				return []models.Media{}, service.ErrTimeout
			}
			log.Error(
				"failed to get media",
				slog.Int("limit", offset),
				slog.Int("offset", l.maxAnswerLength),
				sl.Err(err),
			)
			return []models.Media{}, fmt.Errorf("%s: %w", op, err)
		}
		// End point
		if len(newSlice) == 0 {
			break main_loop
		}
		// Add media tags
		for i, media := range newSlice {
			newSlice[i].Tags, err = l.mediaStorage.MediaTags(ctx, *media.ID)
			if err != nil {
				log.Error("failed to get media tag list", slog.Int64("id", *media.ID), sl.Err(err))
				return []models.Media{}, fmt.Errorf("%s: %w", op, err)
			}
		}
		// Apply filter
		merge := filterRank(newSlice, filter)
		// Merge new slice with previous one
		bestRes = mergeLibs(bestRes, merge)

		if len(bestRes) > filter.MaxRespLen && filter.MaxRespLen > 0 {
			bestRes = bestRes[:filter.MaxRespLen:filter.MaxRespLen]
		}

		// Shortcut for tag filter only
		if filter.Name == "" &&
			filter.Author == "" &&
			len(bestRes) == filter.MaxRespLen {
			break main_loop
		}
	}

	// Create slice for the answer (drop rank info).
	res := make([]models.Media, len(bestRes))
	for i, mediaRank := range bestRes {
		res[i] = mediaRank.media
	}

	log.Info("finish search")

	return res, nil
}

// mergeLibs merges 2 slices in ascending order
func mergeLibs(l1 []mediaRank, l2 []mediaRank) []mediaRank {
	res := make([]mediaRank, len(l1)+len(l2))

	var (
		i1 = 0
		i2 = 0
	)

	for i := 0; i < len(res); i++ {
		i := i

		if i1 == len(l1) {
			res[i] = l2[i2]
			i2++
			continue
		}
		if i2 == len(l2) {
			res[i] = l1[i1]
			i1++
			continue
		}

		if rankCmp(l1[i1], l2[i2]) < 0 {
			res[i] = l1[i1]
			i1++
		} else {
			res[i] = l2[i2]
			i2++
		}
	}

	return res
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
		if errors.Is(err, storage.ErrContextCancelled) {
			log.Error("mediaStorage.SaveMedia timeout exceeded")
			return 0, service.ErrTimeout
		}
		log.Error("failed to save media", sl.Err(err))
		return 0, fmt.Errorf("%s: %w", op, err)
	}

	tags, _ := l.mediaStorage.AllTags(ctx)

	// Check if tags exist.
	for _, tag := range media.Tags {
		if !slices.ContainsFunc(tags, func(t models.Tag) bool {
			return models.EqualTags(t, tag)
		}) {
			log.Warn("tag not found", slog.String("name", tag.Name))
			return 0, service.ErrTagNotFound
		}
	}

	if err := l.mediaStorage.TagMedia(ctx, id, media.Tags...); err != nil {
		if errors.Is(err, storage.ErrContextCancelled) {
			log.Error("mediaStorage.TagMedia timeout exceeded")
			return 0, service.ErrTimeout
		}
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

	chans.Notify(l.updateChan)

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
		if errors.Is(err, storage.ErrContextCancelled) {
			log.Error("mediaStorage.Media timeout exceeded")
			return service.ErrTimeout
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
		if errors.Is(err, storage.ErrContextCancelled) {
			log.Error("mediaStorage.UpdateMediaBasicInfo timeout exceeded")
			return service.ErrTimeout
		}
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
		if !slices.ContainsFunc(tags, func(t models.Tag) bool {
			return models.EqualTags(t, newTag)
		}) {
			log.Warn("tag not found", slog.Int64("id", newTag.ID))
			return service.ErrTagNotFound
		}
		if !slices.ContainsFunc(oldMedia.Tags, func(t models.Tag) bool {
			return models.EqualTags(t, newTag)
		}) {
			tagsToAdd = append(tagsToAdd, newTag)
		}
	}
	for _, oldTag := range oldMedia.Tags {
		if !slices.ContainsFunc(media.Tags, func(t models.Tag) bool {
			return models.EqualTags(t, oldTag)
		}) {
			tagsToDel = append(tagsToDel, oldTag)
		}
	}

	if err := l.mediaStorage.TagMedia(ctx, *media.ID, tagsToAdd...); err != nil {
		if errors.Is(err, storage.ErrContextCancelled) {
			log.Error("mediaStorage.TagMedia timeout exceeded")
			return service.ErrTimeout
		}
		log.Error("failed to tag media")
		return fmt.Errorf("%s: %w", op, err)
	}
	if err := l.mediaStorage.UntagMedia(ctx, *media.ID, tagsToDel...); err != nil {
		if errors.Is(err, storage.ErrContextCancelled) {
			log.Error("mediaStorage.UntagMedia timeout exceeded")
			return service.ErrTimeout
		}
		log.Error("failed to untag media")
		return fmt.Errorf("%s: %w", op, err)
	}

	chans.Notify(l.updateChan)

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

	if !slices.ContainsFunc(tags, func(t models.Tag) bool {
		return models.EqualTags(t, tag)
	}) {
		log.Warn("tag not found", slog.Int64("tag id", tag.ID))
		return service.ErrTagNotFound
	}

	if err := l.mediaStorage.MultiTagMedia(ctx, tag, mediaIds...); err != nil {
		if errors.Is(err, storage.ErrContextCancelled) {
			log.Error("mediaStorage.MultiTagMedia timeout exceeded")
			return service.ErrTimeout
		}
		log.Error("failed to tag media list", slog.Int64("tag id", tag.ID), sl.Err(err))
		return fmt.Errorf("%s: %w", op, err)
	}

	chans.Notify(l.updateChan)

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

	media, err := l.mediaStorage.Media(ctx, id)
	if err != nil {
		if errors.Is(err, storage.ErrMediaNotFound) {
			log.Warn("media not found", slog.Int64("id", id))
			return models.Media{}, service.ErrMediaNotFound
		}
		if errors.Is(err, storage.ErrContextCancelled) {
			log.Error("mediaStorage.Media timeout exceeded")
			return models.Media{}, service.ErrTimeout
		}
		log.Error("failed to get media", slog.Int64("id", id), sl.Err(err))
		return models.Media{}, err
	}

	return media, nil
}

// DeleteMedia deletes media.
//
// If media with given id does not exist, returns error.
func (l *Media) DeleteMedia(ctx context.Context, id int64) error {
	const op = "Media.DeleteMedia"

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
		if errors.Is(err, storage.ErrContextCancelled) {
			log.Error("mediaStorage.DeleteMedia timeout exceeded")
			return service.ErrTimeout
		}
		log.Error("failed to delete media", slog.Int64("id", id))
		return fmt.Errorf("%s: %w", op, err)
	}

	chans.Notify(l.updateChan)

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
		if errors.Is(err, storage.ErrContextCancelled) {
			log.Error("mediaStorage.TagTypes")
			return models.TagTypes{}, service.ErrTimeout
		}
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
		if errors.Is(err, storage.ErrContextCancelled) {
			log.Error("mediaStorage.AllTags timeout exceeded")
			return models.TagList{}, service.ErrTimeout
		}
		log.Error("failed to get tag list", sl.Err(err))
		return models.TagList{}, fmt.Errorf("%s: %w", op, err)
	}

	for i := range tagList {
		tagList[i].Meta, err = l.mediaStorage.TagMeta(ctx, tagList[i])
		if err != nil {
			if errors.Is(err, storage.ErrContextCancelled) {
				log.Error("mediaStorage.TagMeta timeout exceeded")
				return models.TagList{}, service.ErrTimeout
			}
			log.Error("failed to get tag meta", slog.Int64("id", tagList[i].ID), sl.Err(err))
			return models.TagList{}, fmt.Errorf("%s: %w", op, err)
		}
	}

	log.Info("got all tags")

	return tagList, nil
}

// TODO add meta to these methods
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
		if errors.Is(err, storage.ErrContextCancelled) {
			log.Error("mediaStorage.SaveTag timeout exceeded")
			return 0, service.ErrTimeout
		}
		log.Error(
			"failed to register tag",
			slog.String("name", tag.Name),
			sl.Err(err),
		)
		return 0, fmt.Errorf("%s: %w", op, err)
	}

	tag.ID = id

	for k, v := range tag.Meta {
		if err := l.mediaStorage.SetTagMeta(ctx, tag, k, v); err != nil {
			if errors.Is(err, storage.ErrContextCancelled) {
				log.Error("mediaStorage.SetTagMeta timeout exceeded")
				return 0, service.ErrTimeout
			}
			log.Error("failed to add tag meta", sl.Err(err))
			return 0, fmt.Errorf("%s: %w", op, err)
		}
	}

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
		if errors.Is(err, storage.ErrContextCancelled) {
			log.Error("mediaStorage.Tag timeout exceeded")
			return models.Tag{}, service.ErrTimeout
		}
		log.Error("failed to get tag", slog.Int64("id", id), sl.Err(err))
		return models.Tag{}, fmt.Errorf("%s: %w", op, err)
	}

	tag.Meta, err = l.mediaStorage.TagMeta(ctx, tag)
	if err != nil {
		if errors.Is(err, storage.ErrContextCancelled) {
			log.Error("mediaStorage.TagMeta timeout exceeded")
			return models.Tag{}, service.ErrTimeout
		}
		log.Error("failed to get tag meta", slog.Int64("id", id), sl.Err(err))
	}

	for _, ttype := range l.tagTypes {
		if ttype.ID == tag.Type.ID {
			tag.Type.Name = ttype.Name
			return tag, nil
		}
	}

	return models.Tag{}, service.ErrTagTypeNotFound
}

func (l *Media) UpdateTag(ctx context.Context, tag models.Tag) error {
	const op = "Media.UpdateTag"

	log := l.log.With(
		slog.String("op", op),
		slog.String("editorname", models.RootLogin),
		slog.Int64("id", tag.ID),
	)

	oldTag, err := l.Tag(ctx, tag.ID)
	if err != nil {
		if errors.Is(err, service.ErrTagNotFound) {
			log.Warn("tag not found")
			return service.ErrTagNotFound
		}
		if errors.Is(err, service.ErrTimeout) {
			log.Error("tag timeout exceeded")
			return service.ErrTimeout
		}
		log.Error("failed to get currect tag info", sl.Err(err))
		return fmt.Errorf("%s: %w", op, err)
	}

	if oldTag.Type != tag.Type {
		log.Warn("ivalid tag type", slog.String("expected", oldTag.Type.Name), slog.String("got", tag.Type.Name))
		return service.ErrTagTypeInvalid
	}

	if oldTag.Name != tag.Name {
		if err := l.mediaStorage.UpdateTag(ctx, tag); err != nil {
			if errors.Is(err, storage.ErrContextCancelled) {
				log.Error("mediaStorage.UpdateTag timeout exceeded")
				return service.ErrTimeout
			}
			log.Error("failed to update tag name", sl.Err(err))
			return fmt.Errorf("%s: %w", op, err)
		}
	}

	for oldKey := range oldTag.Meta {
		if _, ok := tag.Meta[oldKey]; !ok {
			if err := l.mediaStorage.DelTagMeta(ctx, tag, oldKey); err != nil {
				if errors.Is(err, storage.ErrContextCancelled) {
					log.Error("mediaStorage.DelTagMeta timeout exceeded")
					return service.ErrTimeout
				}
				log.Error("failed to delete tag meta", sl.Err(err))
				return fmt.Errorf("%s: %w", op, err)
			}
		}
	}

	for k, v := range tag.Meta {
		if err := l.mediaStorage.SetTagMeta(ctx, tag, v, k); err != nil {
			if errors.Is(err, storage.ErrContextCancelled) {
				log.Error("mediaStorage.SetTagMeta timeout exceeded")
				return service.ErrTimeout
			}
			log.Error("failed to set tag meta", sl.Err(err))
			return fmt.Errorf("%s: %w", op, err)
		}
	}

	panic("not implemetned")
}

// DeleteTag deletes tag by its id.
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
		if errors.Is(err, storage.ErrContextCancelled) {
			log.Error("mediaStorage.DeleteTag timeout exceeded")
			return service.ErrTimeout
		}
		log.Error("failed to delete media", slog.Int64("id", id))
		return fmt.Errorf("%s: %w", op, err)
	}

	return nil
}
