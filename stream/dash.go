package stream

import (
	//"time"

	"github.com/zencoder/go-dash/v3/mpd"
)

// Manifest for streaming
// Been initialized in func initMPD
var manifest *mpd.MPD

// Create dynamic manifest
func initMPD() {
	manifest = mpd.NewDynamicMPD(
		mpd.DASH_PROFILE_LIVE,
		ScheduleGlobalConfig.AvailabilityStartTime,
		ScheduleGlobalConfig.MinBufferTime,
		mpd.AttrPublishTime(ScheduleGlobalConfig.AvailabilityStartTime),
		mpd.AttrMinimumUpdatePeriod(ScheduleGlobalConfig.MinimumUpdatePeriod),
	)
}

func addNewPeriod(cmp *composition) error {
	tmpMPD, err := mpd.ReadFromFile(BaseDir + "/" + *cmp.name + ".mpd")
	if err != nil {
		return err
	}

}
