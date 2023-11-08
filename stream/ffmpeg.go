package stream

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
)

/*
 * FFmpeg wrapper
 * All ffmpeg commands are inspired by https://codemore.ru/2021/05/01/video-streaming.html
 *
 * No libraries are used, because none of them don't provide necessary functional.
 * Just direct bash call
 *
 * Gintaras Gliaudelis 08.11.2023
 */

// Important information from metadata
type metadata struct {
	duration      float64 // duration
	bitrate       int     // bitrate of composition
	sampling_rate int     // sampling rate of composition
	channels      int     // number of channels (1 or 2)
}

// Availability check "ffmpeg" and "ffprobe" executables
func checkFFmpeg() error {
	c1 := exec.Command("ffmpeg", "-version")
	c2 := exec.Command("ffprobe", "-version")

	err := c1.Run()
	if err != nil {
		return errors.New(`can't find ffmpeg executable (ran "ffmpeg -version)`)
	}

	err = c2.Run()
	if err != nil {
		return errors.New(`can't find ffprobe executable (ran "ffmpeg -version)`)
	}

	return nil
}

// Generate segments
func generateDASHFiles(cmp *composition) error {
	// Check for no intersections
	// between different compositions
	if _, err := os.Stat(BaseDir + "/" + *cmp.name); err == nil {
		return fmt.Errorf("dir with name %s already exists", *cmp.name)
	} else if !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("tried to check dir %s existance and failed with error %s", *cmp.name, err)
	}

	err := os.Mkdir(BaseDir+"/"+*cmp.name, 0777)
	if err != nil {
		return nil
	}

	// set the limit for bitrate (crutch untill bitrateSwitching)
	bitrate := 128000
	if cmp.meta.bitrate < 128000 {
		bitrate = cmp.meta.bitrate
	}
	// set the limit for sampling rate
	sampling_rate := 48000
	if cmp.meta.sampling_rate < 48000 {
		sampling_rate = cmp.meta.sampling_rate
	}

	cmd := exec.Command(
		"ffmpeg",        //																call converter
		"-hide_banner",  //																hide banner
		"-y",            //																force rewriting file
		"-i", *cmp.file, //																input file
		"-c:a", "aac", //																choose codec
		"-b:a", strconv.Itoa(bitrate), //												choose bitrate (TODO: make different bitrate to enable bitrateSwitching)
		"-ac", strconv.Itoa(cmp.meta.channels), //										number of channels (1 - mono, 2 - stereo)
		"-ar", strconv.Itoa(sampling_rate), // 											sampling frequency (usually 41000/48000)
		"-dash_segment_type", "mp4", //													container segments format
		"-use_template", "1", //														use template instead of enumerate (shorter output)
		"-use_timeline", "1", //														more information about timing for all segments
		"-init_seg_name", *cmp.name+`/init-$RepresentationID$.$ext$`, //				template for initialization segment
		"-media_seg_name", *cmp.name+`/chunk-$RepresentationID$-$Number%05d$.$ext$`, //	template for data segments
		"-seg_duration", strconv.FormatFloat(cmp.segmentDuration, 'g', -1, 64), //		duration of each segment
		"-f", "dash", //																choose dash format
		BaseDir+"/"+*cmp.name+".mpd", //												output file
	)

	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	err = cmd.Run()

	return err
}

// Read nececcary metadata from file
func newMeta(file *string) (*metadata, error) {
	// get bitrate
	res, err := getMetadataParameter(file, "bit_rate")
	if err != nil {
		return nil, err
	}
	bitrate, err := strconv.Atoi(res)
	if err != nil {
		return nil, err
	}

	// get sampling rate
	res, err = getMetadataParameter(file, "sample_rate")
	if err != nil {
		return nil, err
	}
	sample_rate, err := strconv.Atoi(res)
	if err != nil {
		return nil, err
	}

	// get channels
	res, err = getMetadataParameter(file, "channels")
	if err != nil {
		return nil, err
	}
	channels, err := strconv.Atoi(res)
	if err != nil {
		return nil, err
	}

	// get duration
	res, err = getMetadataParameter(file, "duration")
	if err != nil {
		return nil, err
	}
	duration, err := strconv.ParseFloat(res, 32)
	if err != nil {
		return nil, err
	}

	meta := metadata{
		bitrate:       bitrate,
		sampling_rate: sample_rate,
		channels:      channels,
		duration:      duration,
	}

	return &meta, nil
}

// Extract from metadata parameter
func getMetadataParameter(file *string, par string) (string, error) {
	cmd := exec.Command(
		"ffprobe",            //						call ffprobe
		"-loglevel", "error", //						set loglevel
		"-show_entries", "stream="+par, // 				set parameter to write
		"-of", "default=noprint_wrappers=1:nokey=1", //	write only the result (without key)
		*file, //									target file
	)

	stdout, err := cmd.Output()

	if err != nil {
		return "", err
	}

	return strings.Trim(string(stdout), "\n"), nil
}
