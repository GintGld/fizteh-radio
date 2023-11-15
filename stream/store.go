package stream

import (
	"time"
)

type contentGenerator interface {
	getNextComposition() (*composition, error)
}

type mixer struct {
	content []string
	id      int
}

func NewMixer(paths ...string) *mixer {
	var m mixer
	m.id = 0
	m.content = paths
	return &m
}

func (m *mixer) getNextComposition() (*composition, error) {
	filePath := m.content[m.id]

	cmp, err := NewComp(m.id, filePath, time.Second)

	m.id++
	if m.id == len(m.content) {
		m.id = 0
	}

	return cmp, err
}
