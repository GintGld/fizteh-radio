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
	json.Path("$.segment").Object().Keys().ContainsOnly("id", "mediaID", "start", "beginCut", "stopCut", "protected")
	json.Path("$.segment.mediaID").Number().IsEqual(mediaID)
	json.Path("$.segment.beginCut").Number().IsEqual(*segment.BeginCut)
	json.Path("$.segment.stopCut").Number().IsEqual(*segment.StopCut)
	json.Path("$.segment.protected").Boolean().IsEqual(false)

	gotTime, err := time.Parse(
		"2006-01-02T15:04:05.999999999-07:00",
		json.Path("$.segment.start").String().Raw(),
	)
	require.NoError(t, err)
	assert.Equal(t, segment.Start.UnixMilli(), gotTime.UnixMilli())
}

func TestGetProtectedSegment(t *testing.T) {
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
	segment.Protected = true

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
	json.Path("$.segment").Object().Keys().ContainsOnly("id", "mediaID", "start", "beginCut", "stopCut", "protected")
	json.Path("$.segment.mediaID").Number().IsEqual(mediaID)
	json.Path("$.segment.beginCut").Number().IsEqual(*segment.BeginCut)
	json.Path("$.segment.stopCut").Number().IsEqual(*segment.StopCut)
	json.Path("$.segment.protected").Boolean().IsEqual(true)

	gotTime, err := time.Parse(
		"2006-01-02T15:04:05.999999999-07:00",
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
		value.Object().Keys().ContainsOnly("id", "mediaID", "start", "beginCut", "stopCut", "protected")
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

	now := time.Now()

	segments := []models.Segment{
		{
			Start: ptr.Ptr(now.Add(-sourceDuration * 5)),
		},
		{
			Start:     ptr.Ptr(now.Add(-sourceDuration / 2)),
			Protected: false,
		},
		{
			Start:     ptr.Ptr(now.Add(-sourceDuration / 2)),
			Protected: true,
		},
		{
			Start:     ptr.Ptr(now.Add(sourceDuration * 5)),
			Protected: false,
		},
		{
			Start:     ptr.Ptr(now.Add(sourceDuration * 5)),
			Protected: true,
		},
	}

	expectedRes := []int{200, 400, 200, 400, 200}
	ids := make([]int64, 5)

	for i, segment := range segments {
		segment.MediaID = ptr.Ptr(int64(mediaId))
		segment.BeginCut = ptr.Ptr[time.Duration](0)
		segment.StopCut = ptr.Ptr(sourceDuration)

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
	for i, id := range ids {
		e.GET("/schedule/{id}", id).
			WithHeader("Authorization", "Bearer "+token).
			Expect().
			Status(expectedRes[i])
	}
}

// randomSegment creates segment (id and mediaId fields are not specified)
func randomSegment() models.Segment {
	begin := time.Duration(gofakeit.Uint32())
	stop := begin + time.Duration(gofakeit.Uint32())

	// all time is stored with precision
	// to microseconds
	begin = begin.Truncate(time.Microsecond)
	stop = stop.Truncate(time.Microsecond)

	return models.Segment{
		Start:    ptr.Ptr(gofakeit.Date()),
		BeginCut: &begin,
		StopCut:  &stop,
	}
}
