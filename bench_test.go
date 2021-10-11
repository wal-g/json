// Copyright 2011 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Large reader benchmark.
// The JSON reader is a summary of agl's changes in the
// go, webkit, and chromium open source projects.
// We benchmark converting between the JSON form
// and in-memory reader structures.

package json

import (
	"bytes"
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"reflect"
	"runtime"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/wal-g/json/mocks"
)

type codeResponse struct {
	Tree     *codeNode `json:"tree"`
	Username string    `json:"username"`
}

type codeNode struct {
	Name     string      `json:"name"`
	Kids     []*codeNode `json:"kids"`
	CLWeight float64     `json:"cl_weight"`
	Touches  int         `json:"touches"`
	MinT     int64       `json:"min_t"`
	MaxT     int64       `json:"max_t"`
	MeanT    int64       `json:"mean_t"`
}

var codeJSON []byte
var codeStruct codeResponse

func codeInit() {
	f, err := os.Open("testdata/code.json.gz")
	if err != nil {
		panic(err)
	}
	defer f.Close()
	gz, err := gzip.NewReader(f)
	if err != nil {
		panic(err)
	}
	data, err := io.ReadAll(gz)
	if err != nil {
		panic(err)
	}

	codeJSON = data

	if err := Unmarshal(bytes.NewReader(codeJSON), &codeStruct); err != nil {
		panic("unmarshal code.json: " + err.Error())
	}

	b := mocks.NewBuildCloser()
	if err = Marshal(&codeStruct, b); err != nil {
		panic("marshal code.json: " + err.Error())
	}
	data = []byte(b.String())

	if !bytes.Equal(data, codeJSON) {
		println("different lengths", len(data), len(codeJSON))
		for i := 0; i < len(data) && i < len(codeJSON); i++ {
			if data[i] != codeJSON[i] {
				println("re-marshal: changed at byte", i)
				println("orig: ", string(codeJSON[i-10:i+10]))
				println("new: ", string(data[i-10:i+10]))
				break
			}
		}
		panic("re-marshal code.json: different result")
	}
}

func BenchmarkCodeEncoder(b *testing.B) {
	b.ReportAllocs()
	if codeJSON == nil {
		b.StopTimer()
		codeInit()
		b.StartTimer()
	}
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			assert.NoError(b, Marshal(&codeStruct, mocks.NewDiscardCloser()))
		}
	})
	b.SetBytes(int64(len(codeJSON)))
}

func BenchmarkCodeMarshal(b *testing.B) {
	b.ReportAllocs()
	if codeJSON == nil {
		b.StopTimer()
		codeInit()
		b.StartTimer()
	}
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			builder := mocks.NewBuildCloser()
			require.NoError(b, Marshal(&codeStruct, builder))
			builder.Reset()
		}
	})
	b.SetBytes(int64(len(codeJSON)))
}

func benchMarshalBytes(n int) func(*testing.B) {
	sample := []byte("hello world")
	// Use a struct pointer, to avoid an allocation when passing it as an
	// interface parameter to Marshal.
	v := &struct {
		Bytes []byte
	}{
		bytes.Repeat(sample, (n/len(sample))+1)[:n],
	}
	return func(b *testing.B) {
		builder := mocks.NewBuildCloser()
		for i := 0; i < b.N; i++ {
			require.NoError(b, Marshal(v, builder))
			builder.Reset()
		}
	}
}

func BenchmarkMarshalBytes(b *testing.B) {
	b.ReportAllocs()
	// 32 fits within encodeState.scratch.
	b.Run("32", benchMarshalBytes(32))
	// 256 doesn't fit in encodeState.scratch, but is small enough to
	// allocate and avoid the slower base64.NewEncoder.
	b.Run("256", benchMarshalBytes(256))
	// 4096 is large enough that we want to avoid allocating for it.
	b.Run("4096", benchMarshalBytes(4096))
}

func BenchmarkCodeDecoder(b *testing.B) {
	b.ReportAllocs()
	if codeJSON == nil {
		b.StopTimer()
		codeInit()
		b.StartTimer()
	}
	b.RunParallel(func(pb *testing.PB) {
		var buf bytes.Buffer
		var r codeResponse
		for pb.Next() {
			buf.Write(codeJSON)
			// hide EOF
			buf.WriteByte('\n')
			buf.WriteByte('\n')
			buf.WriteByte('\n')
			assert.NoError(b, Unmarshal(&buf, &r))
		}
	})
	b.SetBytes(int64(len(codeJSON)))
}

func BenchmarkUnicodeDecoder(b *testing.B) {
	b.ReportAllocs()
	j := []byte(`"\uD83D\uDE01"`)
	b.SetBytes(int64(len(j)))
	r := bytes.NewReader(j)
	var out string
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		assert.NoError(b, Unmarshal(r, &out))
		_, _ = r.Seek(0, 0)
	}
}

func BenchmarkCodeUnmarshal(b *testing.B) {
	b.ReportAllocs()
	if codeJSON == nil {
		b.StopTimer()
		codeInit()
		b.StartTimer()
	}
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			var r codeResponse
			assert.NoError(b, Unmarshal(bytes.NewReader(codeJSON), &r))
		}
	})
	b.SetBytes(int64(len(codeJSON)))
}

func BenchmarkCodeUnmarshalReuse(b *testing.B) {
	b.ReportAllocs()
	if codeJSON == nil {
		b.StopTimer()
		codeInit()
		b.StartTimer()
	}
	b.RunParallel(func(pb *testing.PB) {
		var r codeResponse
		for pb.Next() {
			assert.NoError(b, Unmarshal(bytes.NewReader(codeJSON), &r))
		}
	})
	b.SetBytes(int64(len(codeJSON)))
}

func BenchmarkUnmarshalString(b *testing.B) {
	b.ReportAllocs()
	b.RunParallel(func(pb *testing.PB) {
		var s string
		for pb.Next() {
			assert.NoError(b, Unmarshal(strings.NewReader(`"hello, world"`), &s))
		}
	})
}

func BenchmarkUnmarshalFloat64(b *testing.B) {
	b.ReportAllocs()
	b.RunParallel(func(pb *testing.PB) {
		var f float64
		for pb.Next() {
			assert.NoError(b, Unmarshal(strings.NewReader(`3.14`), &f))
		}
	})
}

func BenchmarkUnmarshalInt64(b *testing.B) {
	b.ReportAllocs()
	b.RunParallel(func(pb *testing.PB) {
		var x int64
		for pb.Next() {
			assert.NoError(b, Unmarshal(strings.NewReader(`3`), &x))
		}
	})
}

func BenchmarkIssue10335(b *testing.B) {
	b.ReportAllocs()
	b.RunParallel(func(pb *testing.PB) {
		var s struct{}
		for pb.Next() {
			assert.NoError(b, Unmarshal(strings.NewReader(`{"a":{ }}`), &s))
		}
	})
}

func BenchmarkIssue34127(b *testing.B) {
	b.ReportAllocs()
	j := struct {
		Bar string `json:"bar,string"`
	}{
		Bar: `foobar`,
	}
	b.RunParallel(func(pb *testing.PB) {
		builder := mocks.NewBuildCloser()
		for pb.Next() {
			assert.NoError(b, Marshal(&j, builder))
			builder.Reset()
		}
	})
}

func BenchmarkUnmapped(b *testing.B) {
	b.ReportAllocs()
	b.RunParallel(func(pb *testing.PB) {
		var s struct{}
		for pb.Next() {
			require.NoError(b, Unmarshal(strings.NewReader(`{"s": "hello", "y": 2, "o": {"x": 0}, "a": [1, 99, {"x": 1}]}`), &s))
		}
	})
}

func BenchmarkTypeFieldsCache(b *testing.B) {
	b.ReportAllocs()
	var maxTypes int = 1e6

	// Dynamically generate many new types.
	types := make([]reflect.Type, maxTypes)
	fs := []reflect.StructField{{
		Type:  reflect.TypeOf(""),
		Index: []int{0},
	}}
	for i := range types {
		fs[0].Name = fmt.Sprintf("TypeFieldsCache%d", i)
		types[i] = reflect.StructOf(fs)
	}

	// clearClear clears the cache. Other JSON operations, must not be running.
	clearCache := func() {
		fieldCache = sync.Map{}
	}

	// MissTypes tests the performance of repeated cache misses.
	// This measures the time to rebuild a cache of size nt.
	for nt := 1; nt <= maxTypes; nt *= 10 {
		ts := types[:nt]
		b.Run(fmt.Sprintf("MissTypes%d", nt), func(b *testing.B) {
			nc := runtime.GOMAXPROCS(0)
			for i := 0; i < b.N; i++ {
				clearCache()
				var wg sync.WaitGroup
				for j := 0; j < nc; j++ {
					wg.Add(1)
					go func(j int) {
						for _, t := range ts[(j*len(ts))/nc : ((j+1)*len(ts))/nc] {
							cachedTypeFields(t)
						}
						wg.Done()
					}(j)
				}
				wg.Wait()
			}
		})
	}

	// HitTypes tests the performance of repeated cache hits.
	// This measures the average time of each cache lookup.
	for nt := 1; nt <= maxTypes; nt *= 10 {
		// Pre-warm a cache of size nt.
		clearCache()
		for _, t := range types[:nt] {
			cachedTypeFields(t)
		}
		b.Run(fmt.Sprintf("HitTypes%d", nt), func(b *testing.B) {
			b.RunParallel(func(pb *testing.PB) {
				for pb.Next() {
					cachedTypeFields(types[0])
				}
			})
		})
	}
}

func BenchmarkEncodeMarshaler(b *testing.B) {
	b.ReportAllocs()

	m := struct {
		A int
		B RawMessage
	}{}

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			assert.NoError(b, Marshal(&m, mocks.NewDiscardCloser()))
		}
	})
}
