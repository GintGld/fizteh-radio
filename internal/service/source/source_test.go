package service

import (
	"errors"
	"log/slog"
	"os"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

const (
	nestingDepth = 4
	maxId        = 10000
)

func TestInitFileSystem(t *testing.T) {
	tmpDir, _ := os.MkdirTemp(os.TempDir(), "")
	defer os.RemoveAll(tmpDir)

	s := Source{
		log: slog.New(
			slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}),
		),
		dir:          tmpDir,
		nestingDepth: nestingDepth,
		maxId:        maxId,
	}

	s.mustInitFilesystem()

	for depth := 1; depth < s.nestingDepth; depth++ {

		N := 1
		for i := 0; i < depth-1; i++ {
			N *= 10
		}

		splitted := make([]string, depth)
		for i := 0; i < N; i++ {
			str := strconv.Itoa(i)

			for j := 0; j < depth-len(str); j++ {
				splitted[j] = "0"
			}
			for j := depth - len(str); j < depth; j++ {
				splitted[j] = string(str[j-depth+len(str)])
			}

			entries, err := os.ReadDir(s.dir + "/" + strings.Join(splitted, "/"))
			assert.NoError(t, err)

			names := make([]string, 0, len(entries))
			for _, entry := range entries {
				names = append(names, entry.Name())
			}
			sort.Strings(names)
			assert.Equal(t, names, []string{"0", "1", "2", "3", "4", "5", "6", "7", "8", "9"})
		}
	}
}

func TestGetCurrentDir(t *testing.T) {
	tmpDir, _ := os.MkdirTemp(os.TempDir(), "")
	defer os.RemoveAll(tmpDir)

	testCases := []struct {
		desc        string
		id          int64
		expectValue string
		expectError error
	}{
		{
			desc:        "id shorter than max",
			id:          10,
			expectValue: tmpDir + "/0/0/1/0",
			expectError: nil,
		},
		{
			desc:        "id with max length",
			id:          1203,
			expectValue: tmpDir + "/1/2/0/3",
			expectError: nil,
		},
		{
			desc:        "negative id",
			id:          -21,
			expectValue: "",
			expectError: errors.New("Source.getCorrespondingDir: invalid media source id"),
		},
		{
			desc:        "id greater max",
			id:          103920,
			expectValue: "",
			expectError: errors.New("Source.getCorrespondingDir: invalid media source id"),
		},
	}

	s := Source{
		log: slog.New(
			slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}),
		),
		dir:          tmpDir,
		nestingDepth: nestingDepth,
		maxId:        maxId,
	}

	for _, tC := range testCases {
		t.Run(tC.desc, func(t *testing.T) {
			res, err := s.getCorrespondingDir(tC.id)
			assert.Equal(t, tC.expectValue, res)
			assert.Equal(t, tC.expectError, err)
		})
	}
}
