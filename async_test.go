package json

import (
	"encoding/json"
	"io"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var _ io.Reader = &slowReader{}

type slowReader struct {
	src   []byte
	index int
	len   int
	pause time.Duration
}

func newSlowReader(src []byte, pause time.Duration) *slowReader {
	return &slowReader{
		src:   src,
		len:   1,
		pause: pause,
	}
}

func (s *slowReader) Work() {
	for i := s.len; i <= len(s.src); i++ {
		time.Sleep(s.pause)
		s.len = i
	}
}

func (s *slowReader) Read(p []byte) (int, error) {
	n := len(p)
	readerLen := s.len
	if s.index == len(s.src) {
		return 0, io.EOF
	}
	if s.index+n <= readerLen {
		copy(p, s.src[s.index:s.index+n])
		s.index += n
		return n, nil
	}
	length := readerLen - s.index
	copy(p, s.src[s.index:readerLen])
	s.index = readerLen
	return length, nil
}

func TestUnmarshal_SlowReader(t *testing.T) {
	initBig()
	data := newSlowReader(jsonBig, time.Microsecond)
	res := new(map[string]interface{})
	expected := new(map[string]interface{})
	require.NoError(t, json.Unmarshal(jsonBig, expected))
	go data.Work()
	require.NoError(t, Unmarshal(data, res))
	assert.Equal(t, expected, res)
}
