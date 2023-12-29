package ffmpeg

import (
	"os/exec"
	"strconv"
	"strings"

	"github.com/GintGld/fizteh-radio/internal/models"
)

func Dir(s *models.Segment) string {
	return strconv.FormatInt(*s.ID, 10)
}

func InitFile(s *models.Segment) string {
	return Dir(s) + `/init.m4s`
}

func ChunkFile(s *models.Segment) string {
	return Dir(s) + `/chunk-$Number%05d$.m4s`
}

// GetMeta extracts metadata parameter
func GetMeta(file *string, par string) (string, error) {
	cmd := exec.Command(
		"ffprobe",            //						call ffprobe
		"-loglevel", "error", //						set loglevel
		"-show_entries", "stream="+par, // 				set parameter to write
		"-of", "default=noprint_wrappers=1:nokey=1", //	write only the result (without key)
		*file, //										target file
	)

	stdout, err := cmd.Output()

	if err != nil {
		return "", err
	}

	return strings.Trim(string(stdout), "\n"), nil
}
