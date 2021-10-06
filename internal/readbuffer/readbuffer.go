package readbuffer

import (
	"io"
	"time"
)

const (
	readBufSize = 1 << 10
	readTimeout = 100 * time.Millisecond
)

type ReadBuffer struct {
	buf   []byte
	index int
	len   int
	src   io.Reader
}

func New(stream io.Reader) ReadBuffer {
	return ReadBuffer{
		buf: make([]byte, readBufSize),
		src: stream,
	}
}

func (r *ReadBuffer) Get(n int) ([]byte, error) {
	if r.len-r.index >= n {
		res := r.buf[r.index : r.index+n]
		r.index += n
		return res, nil
	}
	res := make([]byte, r.len-r.index)
	copy(res, r.buf[r.index:r.len])
	r.index = r.len
	n -= len(res)
	var err error
	for err = r.load(); err == nil; err = r.load() {
		if r.len-r.index >= n {
			res = append(res, r.buf[r.index:r.index+n]...)
			r.index += n
			return res, nil
		}

		res = append(res, r.buf[r.index:r.len]...)
		n -= r.len - r.index
		r.index = r.len
		time.Sleep(readTimeout)
	}
	return res, err
}

func (r *ReadBuffer) load() error {
	n, err := r.src.Read(r.buf)
	r.len = n
	r.index = 0
	return err
}
