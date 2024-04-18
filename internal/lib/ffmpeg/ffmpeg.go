package ffmpeg

import (
	"fmt"
	"os/exec"
	"strconv"
	"strings"
)

func Dir(id int64) string {
	return strconv.FormatInt(id, 10)
}

func DirLive(id int64) string {
	return fmt.Sprintf("live-%d", id)
}

func InitFileBase() string {
	return "init.m4s"
}

func ChunkFileBase() string {
	return `$Number%05d$.m4s`
}

func InitFile(id int64) string {
	return Dir(id) + "/" + InitFileBase()
}

func InitFileLive(id int64) string {
	return DirLive(id) + "/" + InitFileBase()
}

// max length of one mpd period is
// 99999 * segLen(=2s) ~= 55.5 hours.
func ChunkFile(id int64) string {
	return Dir(id) + "/" + ChunkFileBase()
}

func ChunkFileLive(id int64) string {
	return DirLive(id) + "/" + ChunkFileBase()
}

func ChunkFileLiveCurrent(id int64, chunkId int) string {
	s := strconv.Itoa(chunkId)
	s = strings.Repeat("0", 5-len(s)) + s
	return fmt.Sprintf("%s/%s.m4s", DirLive(id), s)
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
