package models_test

import (
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	ptr "github.com/GintGld/fizteh-radio/internal/lib/utils/pointers"
	"github.com/GintGld/fizteh-radio/internal/models"
)

func TestSegmentMarshal(t *testing.T) {
	ti := time.Now()
	writeFormat := ti.Format(models.TimeFormat)

	testCase := struct {
		s      models.Segment
		expect string
	}{
		s: models.Segment{
			ID:      ptr.Ptr[int64](10),
			MediaID: ptr.Ptr[int64](1),
			Start:   ptr.Ptr(ti),
		},
		expect: fmt.Sprintf(`{"id":10,"mediaID":1,"start":"%s","beginCut":null,"stopCut":null}`, writeFormat),
	}

	res, err := json.Marshal(testCase.s)
	require.NoError(t, err)

	fmt.Println(string(res))
	fmt.Println(testCase.expect)

	require.JSONEq(t, testCase.expect, string(res))
}
