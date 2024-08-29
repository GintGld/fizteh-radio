package tests

import (
	"encoding/json"
	"math/rand"
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
		Host:   cfg.HttpServer.Address,
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

func TestCreateSegmentCreateIntersections(t *testing.T) {
	token, err := suite.RootLogin()
	require.NoError(t, err)

	u := url.URL{
		Scheme: "http",
		Host:   cfg.HttpServer.Address,
	}
	e := httpexpect.Default(t, u.String())

	// Create new media
	media := randomMedia()
	mediaStr, err := json.Marshal(media)
	require.NoError(t, err)

	rawMediaID := e.POST("/library/media").
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

	mediaId := int64(rawMediaID)

	d := 4500 * time.Millisecond
	delta := time.Duration(1000000)

	date := time.Date(1980, 0, 0, 0, 0, 0, 0, time.UTC)

	segments := []models.Segment{
		{
			MediaID:   &mediaId,
			Start:     ptr.Ptr(date.Add(-delta)),
			BeginCut:  ptr.Ptr[time.Duration](0),
			StopCut:   ptr.Ptr(d),
			Protected: false,
		},
		{
			MediaID:   &mediaId,
			Start:     ptr.Ptr(date.Add(d - delta)),
			BeginCut:  ptr.Ptr[time.Duration](0),
			StopCut:   ptr.Ptr(2 * delta),
			Protected: false,
		},
		{
			MediaID:   &mediaId,
			Start:     ptr.Ptr(date.Add(d + delta)),
			BeginCut:  ptr.Ptr[time.Duration](0),
			StopCut:   ptr.Ptr(d),
			Protected: false,
		},
	}

	for _, s := range segments {
		e.POST("/schedule").
			WithHeader("Authorization", "Bearer "+token).
			WithJSON(map[string]models.Segment{
				"segment": s,
			}).
			Expect().
			Status(200).
			JSON().
			Path("$.id").
			Number().
			Raw()
	}

	e.POST("/schedule").
		WithHeader("Authorization", "Bearer "+token).
		WithJSON(map[string]models.Segment{
			"segment": {
				MediaID:   &mediaId,
				Start:     ptr.Ptr(date),
				BeginCut:  ptr.Ptr[time.Duration](0),
				StopCut:   ptr.Ptr(2 * d),
				Protected: true,
			},
		}).
		Expect().
		Status(200).
		JSON().
		Path("$.id").
		Number().
		Raw()
}

func TestCreateSegmentWithLive(t *testing.T) {
	token, err := suite.RootLogin()
	require.NoError(t, err)

	u := url.URL{
		Scheme: "http",
		Host:   cfg.HttpServer.Address,
	}
	e := httpexpect.Default(t, u.String())

	// WARN: db should have with id 1

	rawLives := e.GET("/schedule/lives").
		WithHeader("Authorization", "Bearer "+token).
		Expect().
		Status(200).
		Body().
		Raw()

	var lives struct {
		Lives []models.Live `json:"lives"`
	}

	err = json.Unmarshal([]byte(rawLives), &lives)
	require.NoError(t, err)

	require.Greater(t, len(lives.Lives), 0)
	live := lives.Lives[0]

	// Create new media
	media := randomMedia()
	mediaStr, err := json.Marshal(media)
	require.NoError(t, err)

	rawMediaID := e.POST("/library/media").
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

	segment := randomSegment()
	segment.MediaID = ptr.Ptr(int64(rawMediaID))
	segment.Protected = true
	segment.LiveId = live.ID

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
		Host:   cfg.HttpServer.Address,
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
		Host:   cfg.HttpServer.Address,
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

func TestGetLiveSegment(t *testing.T) {
	token, err := suite.RootLogin()
	require.NoError(t, err)

	u := url.URL{
		Scheme: "http",
		Host:   cfg.HttpServer.Address,
	}
	e := httpexpect.Default(t, u.String())

	// WARN: db should have at least on live obj

	rawLives := e.GET("/schedule/lives").
		WithHeader("Authorization", "Bearer "+token).
		Expect().
		Status(200).
		Body().
		Raw()

	var lives struct {
		Lives []models.Live `json:"lives"`
	}

	err = json.Unmarshal([]byte(rawLives), &lives)
	require.NoError(t, err)

	require.Greater(t, len(lives.Lives), 0)
	live := lives.Lives[0]

	// Create new media
	media := randomMedia()
	mediaStr, err := json.Marshal(media)
	require.NoError(t, err)

	rawMediaID := e.POST("/library/media").
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

	segment := randomSegment()
	segment.MediaID = ptr.Ptr(int64(rawMediaID))
	segment.Protected = true
	segment.LiveId = live.ID

	res := e.POST("/schedule").
		WithHeader("Authorization", "Bearer "+token).
		WithJSON(map[string]models.Segment{
			"segment": segment,
		}).
		Expect()

	rawId := res.Status(200).
		JSON().
		Path("$.id").
		Number().
		Raw()

	json := e.GET("/schedule/{id}", int(rawId)).
		WithHeader("Authorization", "Bearer "+token).
		Expect().
		Status(200).
		JSON()

	json.Object().Keys().ContainsOnly("segment")
	json.Path("$.segment").Object().Keys().ContainsOnly("id", "mediaID", "start", "beginCut", "stopCut", "protected", "liveId")
	json.Path("$.segment.mediaID").Number().IsEqual(int(rawMediaID))
	json.Path("$.segment.beginCut").Number().IsEqual(*segment.BeginCut)
	json.Path("$.segment.stopCut").Number().IsEqual(*segment.StopCut)
	json.Path("$.segment.liveId").Number().IsEqual(segment.LiveId)
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
		Host:   cfg.HttpServer.Address,
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
		Host:   cfg.HttpServer.Address,
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
		Host:   cfg.HttpServer.Address,
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
		Host:   cfg.HttpServer.Address,
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
		Host:   cfg.HttpServer.Address,
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

	for i, segment := range segments {
		segment.MediaID = ptr.Ptr(int64(mediaId))
		segment.BeginCut = ptr.Ptr[time.Duration](0)
		segment.StopCut = ptr.Ptr(sourceDuration)

		rawId := e.POST("/schedule").
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

		id := int(rawId)

		e.DELETE("/schedule").
			WithHeader("Authorization", "Bearer "+token).
			WithQuery("from", now.Unix()).
			Expect().
			Status(200)

		e.GET("/schedule/{id}", id).
			WithHeader("Authorization", "Bearer "+token).
			Expect().
			Status(expectedRes[i])
	}

}

// randomSegment creates segment (id and mediaId fields are not specified)
func randomSegment() models.Segment {
	begin := time.Duration(rand.Intn(int(sourceDuration)))
	stop := begin + time.Duration(rand.Intn(int(sourceDuration)))
	if stop > sourceDuration {
		stop = sourceDuration
	}

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
