package mocks

import (
	"io"
	"strings"
)

var _ io.WriteCloser = &BuildCloser{}

type BuildCloser struct {
	strings.Builder
}

func (b *BuildCloser) Close() error {
	return nil
}

func NewBuildCloser() *BuildCloser {
	return &BuildCloser{strings.Builder{}}
}
