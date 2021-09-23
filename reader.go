package json

import (
	"io"
	"strings"
)

const (
	readBufSize = 1 << 10
	closeBufSize = 1 << 10
)

type streamReader struct {
	buf      strings.Builder // TODO: decide on best DS
	readBuf  readBuffer
	dropped  int
	finished bool
	scanner  *scanner
}

func newStreamReader(stream io.Reader) *streamReader {
	return &streamReader{
		buf:     strings.Builder{},
		readBuf: newReadBuffer(stream),
		scanner: newScanner(),
	}
}

func (s *streamReader) Len() int {
	return len(s.buf.String()) + s.dropped
}

func (s *streamReader) Load(i int) error {
	if i < s.Len() {
		return nil
	}
	neededLen := i - s.Len() + 1
	buf, err := s.readBuf.Get(neededLen)
	n := len(buf)
	for j := 0; j < n; j++ {
		if opcode := s.scanner.step(s.scanner, buf[j]); opcode == scanError {
			return s.scanner.err
		}
	}
	s.buf.Write(buf[:n])
	if n < neededLen {
		if code := s.scanner.eof(); code == scanError {
			return s.scanner.err
		}
	}
	if err == io.EOF {
		s.finished = true
	}
	return err
}

func (s *streamReader) Get(i int) byte {
	return s.buf.String()[i-s.dropped]
}

func (s *streamReader) Range(l, r int) []byte {
	return []byte(s.buf.String()[l-s.dropped : r-s.dropped])
}

func (s *streamReader) Drop() {
	s.dropped += s.buf.Len()
	s.buf.Reset()
}

func (s *streamReader) Close() error {
	buf, err := s.readBuf.Get(closeBufSize)
	for err == nil {
		for i := 0; i < len(buf); i++ {
			if opCode := s.scanner.step(s.scanner, buf[i]); opCode == scanError {
				return s.scanner.err
			}
		}
		buf, err = s.readBuf.Get(closeBufSize)
	}
	if opCode := s.scanner.eof(); opCode == scanError {
		return s.scanner.err
	} else {
		return nil
	}
}

type readBuffer struct {
	buf   []byte
	index int
	len   int
	src   io.Reader
}

func newReadBuffer(stream io.Reader) readBuffer {
	return readBuffer{
		buf: make([]byte, readBufSize),
		src: stream,
	}
}

func (r *readBuffer) Get(n int) ([]byte, error) {
	if r.len - r.index >= n {
		res := r.buf[r.index : r.index+n]
		r.index += n
		return res, nil
	}
	got := make([]byte, r.len - r.index)
	copy(got, r.buf[r.index:r.len])
	r.index = r.len
	n -= len(got)
	if err := r.load(); err == io.EOF && len(got) > 0 {
		return got, nil
	} else if err != nil {
		return got, err
	}
	if r.len - r.index >= n {
		res := append(got, r.buf[r.index:r.index + n]...)
		r.index += n
		return res, nil
	}
	res := append(got, r.buf[r.index:r.len]...)
	r.index = r.len
	return res, nil
}

func (r *readBuffer) load() error {
	n, err := r.src.Read(r.buf)
	r.len = n
	r.index = 0
	return err
}
