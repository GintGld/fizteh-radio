package tests

import (
	"encoding/json"
	"net/url"
	"testing"
	"time"

	"github.com/brianvoe/gofakeit/v6"
	"github.com/gavv/httpexpect/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	ptr "github.com/GintGld/fizteh-radio/internal/lib/utils/pointers"
	"github.com/GintGld/fizteh-radio/internal/models"
	"github.com/GintGld/fizteh-radio/tests/suite"
)

func TestCreateNewSegment(t *testing.T) {
	token, err := suite.RootLogin()
	require.NoError(t, err)

	u := url.URL{
		Scheme: "http",
		Host:   cfg.Address,
	}
	e := httpexpect.Default(t, u.String())

	media := randomMedia()
	mediaStr, err := json.Marshal(media)
	require.NoError(t, err)

	mediaId := e.POST("/library/media").
		WithHeader("Authorization", "Bearer "+token).
		WithMultipart().
		WithFile("source", sourceFile).
		WithFormField("media", string(mediaStr)).
		Expect().Status(200).
		JSON().
		Path("$.id").
		Number().
		Raw()

	segment := randomSegment()
	segment.MediaID = ptr.Ptr(int64(mediaId))

	res := e.POST("/schedule").
		WithHeader("Authorization", "Bearer "+token).
		WithJSON(map[string]models.Segment{
			"segment": segment,
		}).
		Expect()

	res.Status(200).
		JSON().
		Object().
		Keys().
		ContainsOnly("id")
}

func TestGetSegment(t *testing.T) {
	token, err := suite.RootLogin()
	require.NoError(t, err)

	u := url.URL{
		Scheme: "http",
		Host:   cfg.Address,
	}
	e := httpexpect.Default(t, u.String())

	// Create new media
	media := randomMedia()
	mediaStr, err := json.Marshal(media)
	require.NoError(t, err)

	// Post media
	mediaID := e.POST("/library/media").
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

	// Create new source
	segment := randomSegment()
	segment.MediaID = ptr.Ptr(int64(mediaID))

	// Post segment
	id := e.POST("/schedule").
		WithHeader("Authorization", "Bearer "+token).
		WithJSON(map[string]models.Segment{
			"segment": segment,
		}).
		Expect().
		Status(200).
		JSON().
		Path("$.id").
		Number().
		Raw()

	// Get segment
	json := e.GET("/schedule/{id}", int64(id)).
		WithHeader("Authorization", "Bearer "+token).
		Expect().
		Status(200).
		JSON()

	// Check response
	json.Object().Keys().ContainsOnly("segment")
	json.Path("$.segment").Object().Keys().ContainsOnly("id", "mediaID", "start", "beginCut", "stopCut")
	json.Path("$.segment.mediaID").Number().IsEqual(mediaID)
	json.Path("$.segment.beginCut").Number().IsEqual(*segment.BeginCut)
	json.Path("$.segment.stopCut").Number().IsEqual(*segment.StopCut)
	gotTime, err := time.Parse(
		"2006-01-02T15:04:05.999999999Z",
		json.Path("$.segment.start").String().Raw(),
	)
	require.NoError(t, err)
	assert.Equal(t, segment.Start.UnixMilli(), gotTime.UnixMilli())
}

func TestGetNotExistingSegment(t *testing.T) {
	token, err := suite.RootLogin()
	require.NoError(t, err)

	u := url.URL{
		Scheme: "http",
		Host:   cfg.Address,
	}
	e := httpexpect.Default(t, u.String())

	// Get not existing editor
	json := e.GET("/schedule/{id}", gofakeit.Uint32()).
		WithHeader("Authorization", "Bearer "+token).
		Expect().
		Status(400).
		JSON()

	// Check response
	json.Object().Keys().ContainsOnly("error")
	json.Path("$.error").String().IsEqualFold("segment not found")
}

func TestDeleteSegment(t *testing.T) {
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

	// Post media
	mediaID := e.POST("/library/media").
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

	// Create new source
	segment := randomSegment()
	segment.MediaID = ptr.Ptr(int64(mediaID))

	// Post segment
	id := e.POST("/schedule").
		WithHeader("Authorization", "Bearer "+token).
		WithJSON(map[string]models.Segment{
			"segment": segment,
		}).
		Expect().
		Status(200).
		JSON().
		Path("$.id").
		Number().
		Raw()

	// Delete editor
	e.DELETE("/schedule/{id}", id).
		WithHeader("Authorization", "Bearer "+token).
		Expect().
		Status(200)

	// Check the deletion
	json := e.GET("/schedule/{id}", id).
		WithHeader("Authorization", "Bearer "+token).
		Expect().
		Status(400).
		JSON()

	json.Object().Keys().ContainsOnly("error")
	json.Path("$.error").String().IsEqualFold("segment not found")
}

func TestDeleteNotExistingSegment(t *testing.T) {
	token, err := suite.RootLogin()
	require.NoError(t, err)

	u := url.URL{
		Scheme: "http",
		Host:   cfg.Address,
	}
	e := httpexpect.Default(t, u.String())

	// Trying to delete not existing editor
	json := e.DELETE("/schedule/{id}", gofakeit.Uint32()).
		WithHeader("Authorization", "Bearer "+token).
		Expect().
		Status(400).
		JSON()

	json.Object().Keys().ContainsOnly("error")
	json.Path("$.error").String().IsEqualFold("segment not found")
}

func TestGetScheduleCut(t *testing.T) {
	token, err := suite.RootLogin()
	require.NoError(t, err)

	u := url.URL{
		Scheme: "http",
		Host:   cfg.Address,
	}
	e := httpexpect.Default(t, u.String())

	json := e.GET("/schedule").
		WithHeader("Authorization", "Bearer "+token).
		Expect().
		Status(200).
		JSON()

	json.Object().Keys().ContainsOnly("segments")
	for _, value := range json.Path("$.segments").Array().Iter() {
		value.Object().Keys().ContainsOnly("id", "mediaID", "start", "beginCut", "stopCut")
	}
}

func TestClearSchedule(t *testing.T) {
	token, err := suite.RootLogin()
	require.NoError(t, err)

	u := url.URL{
		Scheme: "http",
		Host:   cfg.Address,
	}
	e := httpexpect.Default(t, u.String())

	media := randomMedia()
	mediaStr, err := json.Marshal(media)
	require.NoError(t, err)

	// post media
	mediaId := e.POST("/library/media").
		WithHeader("Authorization", "Bearer "+token).
		WithMultipart().
		WithFile("source", sourceFile).
		WithFormField("media", string(mediaStr)).
		Expect().Status(200).
		JSON().
		Path("$.id").
		Number().
		Raw()

	// create 3 segments (before now, now, after now)
	// after clearing second and third must be deleted
	now := time.Now()
	segmentBefore := models.Segment{
		Start: ptr.Ptr(now.Add(-sourceDuration * 5)),
	}
	segmentNow := models.Segment{
		Start: ptr.Ptr(now.Add(-sourceDuration / 2)),
	}
	segmentAfter := models.Segment{
		Start: ptr.Ptr(now.Add(sourceDuration * 5)),
	}

	segmentBefore.MediaID = ptr.Ptr(int64(mediaId))
	segmentNow.MediaID = ptr.Ptr(int64(mediaId))
	segmentAfter.MediaID = ptr.Ptr(int64(mediaId))

	segments := []models.Segment{segmentBefore, segmentNow, segmentAfter}
	ids := make([]int64, 3)

	// post 3 segments
	for i, segment := range segments {
		index := e.POST("/schedule").
			WithHeader("Authorization", "Bearer "+token).
			WithJSON(map[string]models.Segment{
				"segment": segment,
			}).
			Expect().
			Status(200).
			JSON().
			Path("$.id").
			Number().
			Raw()
		ids[i] = int64(index)
	}

	e.DELETE("/schedule").
		WithHeader("Authorization", "Bearer "+token).
		WithQuery("from", now.Unix()).
		Expect().
		Status(200)

	// Check the deletion
	e.GET("/schedule/{id}", ids[0]).
		WithHeader("Authorization", "Bearer "+token).
		Expect().
		Status(200)
	e.GET("/schedule/{id}", ids[1]).
		WithHeader("Authorization", "Bearer "+token).
		Expect().
		Status(400)
	e.GET("/schedule/{id}", ids[2]).
		WithHeader("Authorization", "Bearer "+token).
		Expect().
		Status(400)
}

// TODO: enable beginCut stopCut when it will be fixed

// randomSegment creates segment (id and mediaId fields are not specified)
func randomSegment() models.Segment {
	// begin := time.Duration(gofakeit.Uint32())
	// stop := begin + time.Duration(gofakeit.Uint32())

	return models.Segment{
		Start: ptr.Ptr(gofakeit.Date()),
		// BeginCut: &begin,
		// StopCut:  &stop,
	}
}
