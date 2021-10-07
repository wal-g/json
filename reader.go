package json

import (
	"io"
	"strings"

	"github.com/EinKrebs/json/internal/readbuffer"
)

const (
	closeBufSize = 1 << 10
)

type streamReader struct {
	buf      strings.Builder
	readBuf  readbuffer.ReadBuffer
	dropped  int
	finished bool
	scanner  *scanner
}

func newStreamReader(stream io.Reader) *streamReader {
	return &streamReader{
		buf:     strings.Builder{},
		readBuf: readbuffer.New(stream),
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
	if err == io.EOF {
		s.finished = true
		if code := s.scanner.eof(); code == scanError {
			return s.scanner.err
		}
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
