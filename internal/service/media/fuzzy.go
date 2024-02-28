package service

import (
	"slices"
	"unicode"
	"unicode/utf8"

	"github.com/GintGld/fizteh-radio/internal/models"
	"github.com/lithammer/fuzzysearch/fuzzy"
	"golang.org/x/text/runes"
	"golang.org/x/text/transform"
	"golang.org/x/text/unicode/norm"
)

/*
 * Code provided here was mostly taken from github.com/lithammer/fuzzysearch/fuzzy
 * It was not public for external use, so I copied and customised it.
 */

var (
	normalizeTransformer transform.Transformer = transform.Chain(norm.NFD, runes.Remove(runes.In(unicode.Mn)), norm.NFC)
	transformer                                = transform.Chain(normalizeTransformer, unicodeFoldTransformer{})
)

type mediaRank struct {
	media models.Media
	rank  int
}

func rankCmp(mr1, mr2 mediaRank) int {
	return mr1.rank - mr2.rank
}

// filterRank returns media with rank slice filtered by tags.
// The returned slice it sorted by rank ascending order.
func filterRank(lib []models.Media, filter models.MediaFilter) []mediaRank {
	out := make([]mediaRank, 0, len(lib))

	for _, media := range lib {
		// Check for containing wanted tags
		add := tagContains(media, filter.Tags)
		// Add media with its Levenshtein distance
		if add {
			out = append(out, mediaRank{
				media: media,
				rank: min(
					fuzzy.LevenshteinDistance(stringTransform(*media.Name), stringTransform(filter.Name)),
					fuzzy.LevenshteinDistance(stringTransform(*media.Author), stringTransform(filter.Author)),
				),
			})
		}
	}

	// sort slice
	slices.SortFunc(out, rankCmp)

	return out
}

func tagContains(media models.Media, tagNames []string) bool {
delete_loop:
	for _, tagName := range tagNames {
		for _, tag := range media.Tags {
			if tag.Name == tagName {
				continue delete_loop
			}
		}
		return false
	}
	return true
}

func stringTransform(s string) (transformed string) {
	var err error
	transformed, _, err = transform.String(transformer, s)
	if err != nil {
		transformed = s
	}

	return
}

type unicodeFoldTransformer struct{ transform.NopResetter }

func (unicodeFoldTransformer) Transform(dst, src []byte, atEOF bool) (nDst, nSrc int, err error) {
	// Converting src to a string allocates.
	// In theory, it need not; see https://go.dev/issue/27148.
	// It is possible to write this loop using utf8.DecodeRune
	// and thereby avoid allocations, but it is noticeably slower.
	// So just let's wait for the compiler to get smarter.
	for _, r := range string(src) {
		if r == utf8.RuneError {
			// Go spec for ranging over a string says:
			// If the iteration encounters an invalid UTF-8 sequence,
			// the second value will be 0xFFFD, the Unicode replacement character,
			// and the next iteration will advance a single byte in the string.
			nSrc++
		} else {
			nSrc += utf8.RuneLen(r)
		}
		r = unicode.ToLower(r)
		x := utf8.RuneLen(r)
		if x > len(dst[nDst:]) {
			err = transform.ErrShortDst
			break
		}
		nDst += utf8.EncodeRune(dst[nDst:], r)
	}
	return nDst, nSrc, err
}
