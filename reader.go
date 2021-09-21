package json

import (
	"fmt"
	"io"
	"strings"
)

type streamReader struct {
	buf      strings.Builder // TODO: decide on best DS
	src      io.Reader
	dropped  int
	finished bool
	scanner *scanner
}

func (s *streamReader) Len() int {
	return len(s.buf.String()) + s.dropped
}

func (s *streamReader) load(i int) error {
	if i < s.Len() {
		return nil
	}
	neededLen := i - s.Len() + 1
	buf := make([]byte, neededLen)
	n, err := s.src.Read(buf)
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

func (s *streamReader) get(i int) byte {
	return s.buf.String()[i-s.dropped]
}

func (s *streamReader) getRange(l, r int) []byte{
	return []byte(s.buf.String()[l - s.dropped:r - s.dropped])
}

func (s *streamReader) drop(i int) error {
	count := i - s.dropped
	if i <= 0 {
		return nil
	}
	str := s.buf.String()
	if count > len(str) {
		return fmt.Errorf("too many to drop, have %d, asked %d", len(str), count)
	}
	s.buf.Reset()
	s.buf.WriteString(str[count:])
	s.dropped = i
	return nil
}

func (s *streamReader) close() error {
	const closeBufSize = 2 << 10
	buf := make([]byte, closeBufSize)
	n, err := s.src.Read(buf)
	for err == nil {
		for i := 0; i < n; i++ {
			if opCode := s.scanner.step(s.scanner, buf[i]); opCode == scanError {
				return s.scanner.err
			}
		}
		n, err = s.src.Read(buf)
	}
	if opCode := s.scanner.eof(); opCode == scanError {
		return s.scanner.err
	} else {
		return nil
	}
}

func newStreamReader(stream io.Reader) *streamReader {
	return &streamReader{
		buf: strings.Builder{},
		src: stream,
		scanner: newScanner(),
	}
}

