package tests

import (
	"encoding/json"
	"net/url"
	"strconv"
	"testing"
	"time"

	"github.com/brianvoe/gofakeit/v6"
	"github.com/gavv/httpexpect/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/GintGld/fizteh-radio/internal/lib/ffmpeg"
	ptr "github.com/GintGld/fizteh-radio/internal/lib/utils/pointers"
	"github.com/GintGld/fizteh-radio/internal/models"
	"github.com/GintGld/fizteh-radio/tests/suite"
)

const (
	tagCount = 3
)

var (
	tagTypes = models.TagTypes{
		models.TagType{ID: 1, Name: "format"},
		models.TagType{ID: 2, Name: "genre"},
		models.TagType{ID: 3, Name: "playlist"},
		models.TagType{ID: 4, Name: "mood"},
		models.TagType{ID: 5, Name: "language"},
	}
)

func TestCreateNewMedia(t *testing.T) {
	token, err := suite.RootLogin()
	require.NoError(t, err)

	u := url.URL{
		Scheme: "http",
		Host:   cfg.Address,
	}
	e := httpexpect.Default(t, u.String())

	// post some tag
	e.POST("/library/tag").
		WithHeader("Authorization", "Bearer "+token).
		WithJSON(struct {
			Tag models.Tag `json:"tag"`
		}{
			Tag: randomTag(),
		}).
		Expect().
		Status(200)

	// get available tags
	tags := make(models.TagList, 0)
	e.GET("/library/tag").
		WithHeader("Authorization", "Bearer "+token).
		Expect().
		Status(200).
		JSON().
		Path("$.tags").
		Decode(&tags)

	// create random media with random tag list
	media := randomMedia()
	media.Tags = randomTagList(tags, tagCount)
	mediaStr, err := json.Marshal(media)
	require.NoError(t, err)

	res := e.POST("/library/media").
		WithHeader("Authorization", "Bearer "+token).
		WithMultipart().
		WithFile("source", sourceFile).
		WithFormField("media", string(mediaStr)).
		Expect()

	res.Status(200).
		JSON().
		Object().
		Keys().
		ContainsOnly("id")
}

func TestCreateMediaWithNotExistingTag(t *testing.T) {
	token, err := suite.RootLogin()
	require.NoError(t, err)

	u := url.URL{
		Scheme: "http",
		Host:   cfg.Address,
	}
	e := httpexpect.Default(t, u.String())

	rndTag := randomTag()
	rndTag.ID = int64(gofakeit.Uint32())

	// create random media with random tag list
	media := randomMedia()
	media.Tags = models.TagList{rndTag}
	mediaStr, err := json.Marshal(media)
	require.NoError(t, err)

	e.POST("/library/media").
		WithHeader("Authorization", "Bearer "+token).
		WithMultipart().
		WithFile("source", sourceFile).
		WithFormField("media", string(mediaStr)).
		Expect().
		Status(400).
		JSON().
		Path("$.error").
		String().
		IsEqual("tag not found")
}

func TestGetMedia(t *testing.T) {
	durationStr, err := ffmpeg.GetMeta(&sourceFile, "duration")
	require.NoError(t, err)

	seconds, err := strconv.ParseFloat(durationStr, 64)
	require.NoError(t, err)

	duration := time.Duration(seconds * 1000000000)

	token, err := suite.RootLogin()
	require.NoError(t, err)

	u := url.URL{
		Scheme: "http",
		Host:   cfg.Address,
	}
	e := httpexpect.Default(t, u.String())

	// get available tags
	tags := make(models.TagList, 0)
	e.GET("/library/tag").
		WithHeader("Authorization", "Bearer "+token).
		Expect().
		Status(200).
		JSON().
		Path("$.tags").
		Decode(&tags)

	// Create new media
	media := randomMedia()
	media.Tags = randomTagList(tags, tagCount)
	mediaStr, err := json.Marshal(media)
	require.NoError(t, err)

	// post media
	id := e.POST("/library/media").
		WithHeader("Authorization", "Bearer "+token).
		WithMultipart().
		WithFile("source", sourceFile).
		WithFormField("media", string(mediaStr)).
		Expect().
		Status(200).
		JSON().
		Path("$.id").
		Number().
		Raw()

	// Get media
	json := e.GET("/library/media/{id}", int64(id)).
		WithHeader("Authorization", "Bearer "+token).
		Expect().
		Status(200).
		JSON()

	// Check response structure
	json.Object().Keys().ContainsOnly("media")
	json.Path("$.media").Object().Keys().ContainsOnly("id", "name", "author", "duration", "tags")
	for _, value := range json.Path("$.media.tags").Array().Iter() {
		value.Object().Keys().ContainsOnly("id", "name", "type")
		value.Path("$.type").Object().Keys().ContainsOnly("id", "name")
	}

	// Check response values
	json.Path("$.media.name").String().IsEqual(*media.Name)
	json.Path("$.media.author").String().IsEqual(*media.Author)
	json.Path("$.media.duration").Number().IsEqual(duration)
	var mediaRes models.Media
	json.Path("$.media").Decode(&mediaRes)
	require.Equal(t, media.Tags, mediaRes.Tags)
}

func TestGetMediaWithoutTags(t *testing.T) {
	durationStr, err := ffmpeg.GetMeta(&sourceFile, "duration")
	require.NoError(t, err)

	seconds, err := strconv.ParseFloat(durationStr, 64)
	require.NoError(t, err)

	duration := time.Duration(seconds * 1000000000)

	token, err := suite.RootLogin()
	require.NoError(t, err)

	u := url.URL{
		Scheme: "http",
		Host:   cfg.Address,
	}
	e := httpexpect.Default(t, u.String())

	// Create new media
	media := randomMedia()
	media.Tags = make(models.TagList, 0)
	mediaStr, err := json.Marshal(media)
	require.NoError(t, err)

	// post media
	id := e.POST("/library/media").
		WithHeader("Authorization", "Bearer "+token).
		WithMultipart().
		WithFile("source", sourceFile).
		WithFormField("media", string(mediaStr)).
		Expect().
		Status(200).
		JSON().
		Path("$.id").
		Number().
		Raw()

	// Get media
	json := e.GET("/library/media/{id}", int64(id)).
		WithHeader("Authorization", "Bearer "+token).
		Expect().
		Status(200).
		JSON()

	// Check response structure
	json.Object().Keys().ContainsOnly("media")
	json.Path("$.media").Object().Keys().ContainsOnly("id", "name", "author", "duration", "tags")
	for _, value := range json.Path("$.media.tags").Array().Iter() {
		value.Object().Keys().ContainsOnly("id", "name", "type")
		value.Path("$.type").Object().Keys().ContainsOnly("id", "name")
	}

	// Check response values
	json.Path("$.media.name").String().IsEqual(*media.Name)
	json.Path("$.media.author").String().IsEqual(*media.Author)
	json.Path("$.media.duration").Number().IsEqual(duration)
	var mediaRes models.Media
	json.Path("$.media").Decode(&mediaRes)
	require.Equal(t, media.Tags, mediaRes.Tags)
}

func TestGetNotExistingMedia(t *testing.T) {
	token, err := suite.RootLogin()
	require.NoError(t, err)

	u := url.URL{
		Scheme: "http",
		Host:   cfg.Address,
	}
	e := httpexpect.Default(t, u.String())

	// Get not existing editor
	json := e.GET("/library/media/{id}", gofakeit.Uint32()).
		WithHeader("Authorization", "Bearer "+token).
		Expect().
		Status(400).
		JSON()

	// Check response
	json.Object().Keys().ContainsOnly("error")
	json.Path("$.error").String().IsEqualFold("media not found")
}

func TestUpdateMediaBasicInfo(t *testing.T) {
	token, err := suite.RootLogin()
	require.NoError(t, err)

	u := url.URL{
		Scheme: "http",
		Host:   cfg.Address,
	}
	e := httpexpect.Default(t, u.String())

	// get available tags
	tags := make(models.TagList, 0)
	e.GET("/library/tag").
		WithHeader("Authorization", "Bearer "+token).
		Expect().
		Status(200).
		JSON().
		Path("$.tags").
		Decode(&tags)

	// create random media with random tag list
	media := randomMedia()
	media.Tags = randomTagList(tags, tagCount)
	mediaStr, err := json.Marshal(media)
	require.NoError(t, err)

	// post "old" media
	id := e.POST("/library/media").
		WithHeader("Authorization", "Bearer "+token).
		WithMultipart().
		WithFile("source", sourceFile).
		WithFormField("media", string(mediaStr)).
		Expect().
		Status(200).
		JSON().
		Path("$.id").
		Number().
		Raw()

	newMedia := randomMedia()
	newMedia.ID = ptr.Ptr(int64(id))
	newMedia.Tags = media.Tags

	// put "new" media
	e.PUT("/library/media").
		WithHeader("Authorization", "Bearer "+token).
		WithJSON(struct {
			Media models.Media `json:"media"`
		}{
			Media: newMedia,
		}).
		Expect().
		Status(200)

	// get "new" media
	json := e.GET("/library/media/{id}", id).
		WithHeader("Authorization", "Bearer "+token).
		Expect().
		Status(200).
		JSON().
		Path("$.media")

	var mediaRes models.Media
	json.Decode(&mediaRes)

	// Original media does not have this information.
	mediaRes.Duration = nil

	require.Equal(t, newMedia, mediaRes)
}

func TestUpdateMediaTags(t *testing.T) {
	token, err := suite.RootLogin()
	require.NoError(t, err)

	u := url.URL{
		Scheme: "http",
		Host:   cfg.Address,
	}
	e := httpexpect.Default(t, u.String())

	// get available tags
	tags := make(models.TagList, 0)
	e.GET("/library/tag").
		WithHeader("Authorization", "Bearer "+token).
		Expect().
		Status(200).
		JSON().
		Path("$.tags").
		Decode(&tags)

	// create random media with random tag list
	media := randomMedia()
	media.Tags = randomTagList(tags, tagCount)
	mediaStr, err := json.Marshal(media)
	require.NoError(t, err)

	// post "old" media
	id := e.POST("/library/media").
		WithHeader("Authorization", "Bearer "+token).
		WithMultipart().
		WithFile("source", sourceFile).
		WithFormField("media", string(mediaStr)).
		Expect().
		Status(200).
		JSON().
		Path("$.id").
		Number().
		Raw()

	newMedia := media
	newMedia.ID = ptr.Ptr(int64(id))
	newMedia.Tags = randomTagList(tags, tagCount)

	// put "new" media
	e.PUT("/library/media").
		WithHeader("Authorization", "Bearer "+token).
		WithJSON(struct {
			Media models.Media `json:"media"`
		}{
			Media: newMedia,
		}).
		Expect().
		Status(200)

	// get "new" media
	json := e.GET("/library/media/{id}", id).
		WithHeader("Authorization", "Bearer "+token).
		Expect().
		Status(200).
		JSON().
		Path("$.media")

	var mediaRes models.Media
	json.Decode(&mediaRes)

	// Original media does not have this information.
	mediaRes.Duration = nil

	require.Equal(t, newMedia, mediaRes)
}

func TestUpdateMediaCreateTags(t *testing.T) {
	token, err := suite.RootLogin()
	require.NoError(t, err)

	u := url.URL{
		Scheme: "http",
		Host:   cfg.Address,
	}
	e := httpexpect.Default(t, u.String())

	// get available tags
	tags := make(models.TagList, 0)
	e.GET("/library/tag").
		WithHeader("Authorization", "Bearer "+token).
		Expect().
		Status(200).
		JSON().
		Path("$.tags").
		Decode(&tags)

	// create random media with random tag list
	media := randomMedia()
	media.Tags = make(models.TagList, 0)
	mediaStr, err := json.Marshal(media)
	require.NoError(t, err)

	// post "old" media
	id := e.POST("/library/media").
		WithHeader("Authorization", "Bearer "+token).
		WithMultipart().
		WithFile("source", sourceFile).
		WithFormField("media", string(mediaStr)).
		Expect().
		Status(200).
		JSON().
		Path("$.id").
		Number().
		Raw()

	newMedia := media
	newMedia.ID = ptr.Ptr(int64(id))
	newMedia.Tags = randomTagList(tags, tagCount)

	// put "new" media
	e.PUT("/library/media").
		WithHeader("Authorization", "Bearer "+token).
		WithJSON(struct {
			Media models.Media `json:"media"`
		}{
			Media: newMedia,
		}).
		Expect().
		Status(200)

	// get "new" media
	json := e.GET("/library/media/{id}", id).
		WithHeader("Authorization", "Bearer "+token).
		Expect().
		Status(200).
		JSON().
		Path("$.media")

	var mediaRes models.Media
	json.Decode(&mediaRes)

	// Original media does not have this information.
	mediaRes.Duration = nil

	require.Equal(t, newMedia, mediaRes)
}

func TestUpdateMediaDeleteTags(t *testing.T) {
	token, err := suite.RootLogin()
	require.NoError(t, err)

	u := url.URL{
		Scheme: "http",
		Host:   cfg.Address,
	}
	e := httpexpect.Default(t, u.String())

	// get available tags
	tags := make(models.TagList, 0)
	e.GET("/library/tag").
		WithHeader("Authorization", "Bearer "+token).
		Expect().
		Status(200).
		JSON().
		Path("$.tags").
		Decode(&tags)

	// create random media with random tag list
	media := randomMedia()
	media.Tags = randomTagList(tags, tagCount)
	mediaStr, err := json.Marshal(media)
	require.NoError(t, err)

	// post "old" media
	id := e.POST("/library/media").
		WithHeader("Authorization", "Bearer "+token).
		WithMultipart().
		WithFile("source", sourceFile).
		WithFormField("media", string(mediaStr)).
		Expect().
		Status(200).
		JSON().
		Path("$.id").
		Number().
		Raw()

	newMedia := media
	newMedia.ID = ptr.Ptr(int64(id))
	newMedia.Tags = make(models.TagList, 0)

	// put "new" media
	e.PUT("/library/media").
		WithHeader("Authorization", "Bearer "+token).
		WithJSON(struct {
			Media models.Media `json:"media"`
		}{
			Media: newMedia,
		}).
		Expect().
		Status(200)

	// get "new" media
	json := e.GET("/library/media/{id}", id).
		WithHeader("Authorization", "Bearer "+token).
		Expect().
		Status(200).
		JSON().
		Path("$.media")

	var mediaRes models.Media
	json.Decode(&mediaRes)

	// Original media does not have this information.
	mediaRes.Duration = nil

	require.Equal(t, newMedia, mediaRes)
}

func TestMultiTagMedia(t *testing.T) {
	token, err := suite.RootLogin()
	require.NoError(t, err)

	u := url.URL{
		Scheme: "http",
		Host:   cfg.Address,
	}
	e := httpexpect.Default(t, u.String())

	// get available tags
	tags := make(models.TagList, 0)
	e.GET("/library/tag").
		WithHeader("Authorization", "Bearer "+token).
		Expect().
		Status(200).
		JSON().
		Path("$.tags").
		Decode(&tags)

	const mediaCount = 2

	ids := make([]int64, mediaCount)
	for i := 0; i < mediaCount; i++ {
		media := randomMedia()
		media.Tags = make(models.TagList, 0)
		mediaStr, err := json.Marshal(media)
		require.NoError(t, err)

		res := e.POST("/library/media").
			WithHeader("Authorization", "Bearer "+token).
			WithMultipart().
			WithFile("source", sourceFile).
			WithFormField("media", string(mediaStr)).
			Expect().
			Status(200).
			JSON().
			Path("$.id").
			Number().
			Raw()
		ids[i] = int64(res)
	}

	tagId := gofakeit.IntRange(0, len(tags)-1)

	e.POST("/library/tag/multi/{id}", tags[tagId].ID).
		WithHeader("Authorization", "Bearer "+token).
		WithJSON(struct {
			Ids []int64 `json:"ids"`
		}{
			Ids: ids,
		}).Expect().
		Status(200)

	var tag models.Tag
	for i := 0; i < mediaCount; i++ {
		e.GET("/library/media/{id}", ids[i]).
			WithHeader("Authorization", "Bearer "+token).
			Expect().
			Status(200).
			JSON().
			Path("$.media.tags[0]").
			Decode(&tag)
		assert.Equal(t, tags[tagId].ID, tag.ID)
	}
}

func TestDeleteMedia(t *testing.T) {
	token, err := suite.RootLogin()
	require.NoError(t, err)

	u := url.URL{
		Scheme: "http",
		Host:   cfg.Address,
	}
	e := httpexpect.Default(t, u.String())

	// Create new editor
	media := randomMedia()
	mediaStr, err := json.Marshal(media)
	require.NoError(t, err)

	id := e.POST("/library/media").
		WithHeader("Authorization", "Bearer "+token).
		WithMultipart().
		WithFile("source", "./source/sample-9s.mp3").
		WithFormField("media", string(mediaStr)).
		Expect().
		Status(200).
		JSON().
		Path("$.id").
		Number().
		Raw()

	// Delete editor
	e.DELETE("/library/media/{id}", id).
		WithHeader("Authorization", "Bearer "+token).
		Expect().
		Status(200)

	// Check the deletion
	json := e.GET("/library/media/{id}", id).
		WithHeader("Authorization", "Bearer "+token).
		Expect().
		Status(400).
		JSON()

	json.Object().Keys().ContainsOnly("error")
	json.Path("$.error").String().IsEqualFold("media not found")
}

func TestDeleteNotExistingMedia(t *testing.T) {
	token, err := suite.RootLogin()
	require.NoError(t, err)

	u := url.URL{
		Scheme: "http",
		Host:   cfg.Address,
	}
	e := httpexpect.Default(t, u.String())

	// Trying to delete not existing editor
	json := e.DELETE("/library/media/{id}", gofakeit.Uint32()).
		WithHeader("Authorization", "Bearer "+token).
		Expect().
		Status(400).
		JSON()

	json.Object().Keys().ContainsOnly("error")
	json.Path("$.error").String().IsEqualFold("media not found")
}

func TestTagTypes(t *testing.T) {
	token, err := suite.RootLogin()
	require.NoError(t, err)

	u := url.URL{
		Scheme: "http",
		Host:   cfg.Address,
	}
	e := httpexpect.Default(t, u.String())

	json := e.GET("/library/tag/types").
		WithHeader("Authorization", "Bearer "+token).
		Expect().
		Status(200).
		JSON()

	json.Object().Keys().ContainsOnly("types")
	for _, value := range json.Path("$.types").Array().Iter() {
		value.Object().Keys().ContainsOnly("id", "name")
	}
}

func TestNewTag(t *testing.T) {
	token, err := suite.RootLogin()
	require.NoError(t, err)

	u := url.URL{
		Scheme: "http",
		Host:   cfg.Address,
	}
	e := httpexpect.Default(t, u.String())

	res := e.POST("/library/tag").
		WithHeader("Authorization", "Bearer "+token).
		WithJSON(struct {
			Tag models.Tag `json:"tag"`
		}{
			Tag: randomTag(),
		}).
		Expect()

	res.Status(200).
		JSON().
		Object().
		Keys().
		ContainsOnly("id")
}

func TestAllTags(t *testing.T) {
	token, err := suite.RootLogin()
	require.NoError(t, err)

	u := url.URL{
		Scheme: "http",
		Host:   cfg.Address,
	}
	e := httpexpect.Default(t, u.String())

	json := e.GET("/library/tag").
		WithHeader("Authorization", "Bearer "+token).
		Expect().
		Status(200).
		JSON()

	json.Object().Keys().ContainsOnly("tags")
	for _, value := range json.Path("$.tags").Array().Iter() {
		value.Object().Keys().ContainsOnly("id", "name", "type")
		value.Path("$.type").Object().Keys().ContainsOnly("id", "name")
	}
}

func TestDeleteTag(t *testing.T) {
	token, err := suite.RootLogin()
	require.NoError(t, err)

	u := url.URL{
		Scheme: "http",
		Host:   cfg.Address,
	}
	e := httpexpect.Default(t, u.String())

	res := e.POST("/library/tag").
		WithHeader("Authorization", "Bearer "+token).
		WithJSON(struct {
			Tag models.Tag `json:"tag"`
		}{
			Tag: randomTag(),
		}).
		Expect()

	idRaw := res.
		Status(200).
		JSON().
		Path("$.id").
		Number().
		Raw()
	id := int(idRaw)

	e.DELETE("/library/tag/{id}", id).
		WithHeader("Authorization", "Bearer "+token).
		Expect().
		Status(200)
}

func TestDeleteNotExistingTag(t *testing.T) {
	token, err := suite.RootLogin()
	require.NoError(t, err)

	u := url.URL{
		Scheme: "http",
		Host:   cfg.Address,
	}
	e := httpexpect.Default(t, u.String())

	// Trying to delete not existing editor
	json := e.DELETE("/library/tag/{id}", gofakeit.Uint32()).
		WithHeader("Authorization", "Bearer "+token).
		Expect().
		Status(400).
		JSON()

	json.Object().Keys().ContainsOnly("error")
	json.Path("$.error").String().IsEqualFold("tag not found")
}

func randomMedia() models.Media {
	return models.Media{
		Name:   ptr.Ptr(gofakeit.MovieName()),
		Author: ptr.Ptr(gofakeit.Name()),
	}
}

func randomTag() models.Tag {
	return models.Tag{
		Name: gofakeit.Adjective(),
		Type: models.TagType{
			ID: int64(gofakeit.IntRange(1, len(tagTypes))),
		},
	}
}

func randomTagList(tags models.TagList, tagLen int) models.TagList {
	list := make(models.TagList, tagLen)

	for i := 0; i < tagLen; i++ {
		id := gofakeit.IntRange(0, len(tags)-1)
		list[i] = tags[id]
	}

	return list
}
