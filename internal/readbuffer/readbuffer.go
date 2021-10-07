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
	buf      []byte
	index    int
	len      int
	src      io.Reader
	finished bool
}

func New(stream io.Reader) ReadBuffer {
	return ReadBuffer{
		buf: make([]byte, readBufSize),
		src: stream,
	}
}

func (r *ReadBuffer) Get(n int) (res []byte, err error) {
	n, res = r.appendFromBuffer(n, res)
	if n == 0 {
		return
	}
	for err = r.load(); err == nil; err = r.load() {
		n, res = r.appendFromBuffer(n, res)
		if n == 0 {
			return
		}
		time.Sleep(readTimeout)
	}
	if n == 0 {
		return
	}
	if err == io.EOF {
		r.finished = true
		err = nil
	} else {
		return nil, err
	}
	n, res = r.appendFromBuffer(n, res)
	if n > 0 && r.finished {
		return res, io.EOF
	}
	return
}

func (r *ReadBuffer) appendFromBuffer(n int, dst []byte) (int, []byte) {
	length := r.len - r.index
	if length >= n {
		res := append(dst, r.buf[r.index:r.index+n]...)
		r.index += n
		return 0, res
	}
	res := append(dst, r.buf[r.index:r.len]...)
	r.index = r.len
	return n - length, res
}

func (r *ReadBuffer) load() error {
	n, err := r.src.Read(r.buf)
	r.len = n
	r.index = 0
	return err
}
