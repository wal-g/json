package mocks

import "io"

var _ io.WriteCloser = &DiscardCloser{}

type DiscardCloser struct {
	io.Writer
}

func (d *DiscardCloser) Close() error {
	return nil
}

func NewDiscardCloser() *DiscardCloser {
	return &DiscardCloser{io.Discard}
}
