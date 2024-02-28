package service

import (
	"testing"

	"github.com/stretchr/testify/assert"

	ptr "github.com/GintGld/fizteh-radio/internal/lib/utils/pointers"
	"github.com/GintGld/fizteh-radio/internal/models"
)

func TestTagContains(t *testing.T) {
	testCases := []struct {
		desc     string
		media    models.Media
		tagNames []string
		expect   bool
	}{
		{
			desc: "source == target != 0",
			media: models.Media{
				Tags: models.TagList{{Name: "tag1"}},
			},
			tagNames: []string{"tag1"},
			expect:   true,
		},
		{
			desc: "source > target",
			media: models.Media{
				Tags: models.TagList{{Name: "tag1"}, {Name: "tag2"}},
			},
			tagNames: []string{"tag1"},
			expect:   true,
		},
		{
			desc: "source < target",
			media: models.Media{
				Tags: models.TagList{{Name: "tag1"}},
			},
			tagNames: []string{"tag1", "tag2"},
			expect:   false,
		},
		{
			desc: "source & target != 0",
			media: models.Media{
				Tags: models.TagList{{Name: "tag1"}, {Name: "tag2"}},
			},
			tagNames: []string{"tag1", "tag3"},
			expect:   false,
		},
		{
			desc: "source & target == 0; source, target != 0",
			media: models.Media{
				Tags: models.TagList{{Name: "tag1"}, {Name: "tag2"}},
			},
			tagNames: []string{"tag3"},
			expect:   false,
		},
		{
			desc: "source == 0; target != 0",
			media: models.Media{
				Tags: models.TagList{},
			},
			tagNames: []string{"tag3"},
			expect:   false,
		},
		{
			desc: "source != 0; target == 0",
			media: models.Media{
				Tags: models.TagList{{Name: "tag1"}},
			},
			tagNames: []string{},
			expect:   true,
		},
		{
			desc: "source == target == 0",
			media: models.Media{
				Tags: models.TagList{},
			},
			tagNames: []string{},
			expect:   true,
		},
	}

	for _, tC := range testCases {
		t.Run(tC.desc, func(t *testing.T) {
			res := tagContains(tC.media, tC.tagNames)
			assert.Equal(t, tC.expect, res)
		})
	}
}

func TestFilterRank(t *testing.T) {
	testCases := []struct {
		source []models.Media
		filter models.MediaFilter
		expect []mediaRank
		desc   string
	}{
		{
			desc: "name/author ordering",
			source: []models.Media{
				{
					Name:   ptr.Ptr("a a"),
					Author: ptr.Ptr("amaspdmsap"),
				},
				{
					Name:   ptr.Ptr("apple"),
					Author: ptr.Ptr("hété"),
				},
				{
					Name:   ptr.Ptr("apple juice"),
					Author: ptr.Ptr("bitbit"),
				},
			},
			filter: models.MediaFilter{
				Name:   "aaa",
				Author: "hit",
			},
			expect: []mediaRank{
				{
					media: models.Media{
						Name:   ptr.Ptr("a a"),
						Author: ptr.Ptr("amaspdmsap"),
					},
					rank: 1,
				},
				{
					media: models.Media{
						Name:   ptr.Ptr("apple"),
						Author: ptr.Ptr("hété"),
					},
					rank: 2,
				},
				{
					media: models.Media{
						Name:   ptr.Ptr("apple juice"),
						Author: ptr.Ptr("bitbit"),
					},
					rank: 4,
				},
			},
		},
		{
			desc: "tag filtering",
			source: []models.Media{
				{
					Name:   ptr.Ptr("a"),
					Author: ptr.Ptr("b"),
					Tags:   models.TagList{{Name: "tag1"}, {Name: "tag2"}},
				},
				{
					Name:   ptr.Ptr("a"),
					Author: ptr.Ptr("b"),
					Tags:   models.TagList{{Name: "tag1"}, {Name: "tag3"}},
				},
				{
					Name:   ptr.Ptr("a"),
					Author: ptr.Ptr("b"),
				},
			},
			filter: models.MediaFilter{Name: "sun", Author: "dy", Tags: []string{"tag2"}},
			expect: []mediaRank{
				{
					media: models.Media{
						Name:   ptr.Ptr("a"),
						Author: ptr.Ptr("b"),
						Tags:   models.TagList{{Name: "tag1"}, {Name: "tag2"}},
					},
					rank: 2,
				},
			},
		},
		{
			desc: "name/author ordering + tag filtering",
			source: []models.Media{
				{
					Name:   ptr.Ptr("a a"),
					Author: ptr.Ptr("amaspdmsap"),
					Tags:   models.TagList{{Name: "tag1"}, {Name: "tag2"}},
				},
				{
					Name:   ptr.Ptr("apple"),
					Author: ptr.Ptr("hété"),
					Tags:   models.TagList{{Name: "tag1"}, {Name: "tag3"}},
				},
				{
					Name:   ptr.Ptr("apple juice"),
					Author: ptr.Ptr("bitbit"),
					Tags:   models.TagList{{Name: "tag3"}},
				},
			},
			filter: models.MediaFilter{
				Name:   "aaa",
				Author: "hit",
				Tags:   []string{"tag3"},
			},
			expect: []mediaRank{
				{
					media: models.Media{
						Name:   ptr.Ptr("apple"),
						Author: ptr.Ptr("hété"),
						Tags:   models.TagList{{Name: "tag1"}, {Name: "tag3"}},
					},
					rank: 2,
				},
				{
					media: models.Media{
						Name:   ptr.Ptr("apple juice"),
						Author: ptr.Ptr("bitbit"),
						Tags:   models.TagList{{Name: "tag3"}},
					},
					rank: 4,
				},
			},
		},
	}

	for _, tC := range testCases {
		t.Run(tC.desc, func(t *testing.T) {
			res := filterRank(tC.source, tC.filter)
			assert.Equal(t, tC.expect, res)
		})
	}
}

func TestMergeLibs(t *testing.T) {
	testCases := []struct {
		desc   string
		l1     []mediaRank
		l2     []mediaRank
		expect []mediaRank
	}{
		{
			desc: "",
			l1: []mediaRank{
				{
					media: models.Media{ID: ptr.Ptr[int64](1)},
					rank:  2,
				},
				{
					media: models.Media{ID: ptr.Ptr[int64](2)},
					rank:  10,
				},
			},
			l2: []mediaRank{
				{
					media: models.Media{ID: ptr.Ptr[int64](3)},
					rank:  3,
				},
				{
					media: models.Media{ID: ptr.Ptr[int64](4)},
					rank:  4,
				},
				{
					media: models.Media{ID: ptr.Ptr[int64](5)},
					rank:  5,
				},
			},
			expect: []mediaRank{
				{
					media: models.Media{ID: ptr.Ptr[int64](1)},
					rank:  2,
				},
				{
					media: models.Media{ID: ptr.Ptr[int64](3)},
					rank:  3,
				},
				{
					media: models.Media{ID: ptr.Ptr[int64](4)},
					rank:  4,
				},
				{
					media: models.Media{ID: ptr.Ptr[int64](5)},
					rank:  5,
				},
				{
					media: models.Media{ID: ptr.Ptr[int64](2)},
					rank:  10,
				},
			},
		},
	}

	for _, tC := range testCases {
		t.Run(tC.desc, func(t *testing.T) {
			res := mergeLibs(tC.l1, tC.l2)
			assert.Equal(t, tC.expect, res)
		})
	}
}
