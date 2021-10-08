package json

import (
	"encoding/json"
	"fmt"
	"io"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var (
	_         io.Reader = &slowReader{}
	errNotSet           = fmt.Errorf("marshalErr not set yet")
)

type slowReader struct {
	src   []byte
	index int
	len   int
	mutex sync.Mutex
	pause time.Duration
}

func newSlowReader(src []byte, pause time.Duration) *slowReader {
	return &slowReader{
		src:   src,
		len:   1,
		pause: pause,
		mutex: sync.Mutex{},
	}
}

func (s *slowReader) Work() {
	s.mutex.Lock()
	i := s.len
	s.mutex.Unlock()
	for ; i <= len(s.src); i++ {
		time.Sleep(s.pause)
		s.mutex.Lock()
		s.len = i
		s.mutex.Unlock()
	}
}

func (s *slowReader) Read(p []byte) (int, error) {
	n := len(p)
	s.mutex.Lock()
	readerLen := s.len
	s.mutex.Unlock()
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
	data := newSlowReader(jsonBig, 100*time.Nanosecond)
	res := new(map[string]interface{})
	expected := new(map[string]interface{})
	require.NoError(t, json.Unmarshal(jsonBig, expected))
	go data.Work()
	require.NoError(t, Unmarshal(data, res))
	assert.Equal(t, expected, res)
}

func TestMarshalUnmarshalAsync(t *testing.T) {
	want := genValue(100000)
	r, w := io.Pipe()
	mutex := sync.Mutex{}
	err := errNotSet
	go func() {
		mutex.Lock()
		err = Marshal(want, w)
		mutex.Unlock()
	}()
	var got interface{}
	require.NoError(t, Unmarshal(r, &got))
	mutex.Lock()
	require.NoError(t, err)
	mutex.Unlock()
	assert.Equal(t, want, got)
}

func TestMarshalAsync(t *testing.T) {
	r, w := io.Pipe()
	mutex := sync.Mutex{}
	marshalErr := errNotSet
	go func() {
		mutex.Lock()
		marshalErr = Marshal(allValue, w)
		mutex.Unlock()
	}()
	data, err := io.ReadAll(r)
	require.NoError(t, err)
	mutex.Lock()
	require.NoError(t, marshalErr)
	mutex.Unlock()
	require.Equal(t, string(data), allValueCompact)
}
