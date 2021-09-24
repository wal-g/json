package readbuffer

import "io"

const readBufSize = 1 << 10

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
	got := make([]byte, r.len-r.index)
	copy(got, r.buf[r.index:r.len])
	r.index = r.len
	n -= len(got)
	if err := r.load(); err == io.EOF && len(got) > 0 {
		return got, nil
	} else if err != nil {
		return got, err
	}
	if r.len-r.index >= n {
		res := append(got, r.buf[r.index:r.index+n]...)
		r.index += n
		return res, nil
	}
	res := append(got, r.buf[r.index:r.len]...)
	r.index = r.len
	return res, nil
}

func (r *ReadBuffer) load() error {
	n, err := r.src.Read(r.buf)
	r.len = n
	r.index = 0
	return err
}
