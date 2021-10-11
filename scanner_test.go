// Copyright 2010 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package json

import (
	"bytes"
	"math"
	"math/rand"
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/wal-g/json/mocks"
)

var validTests = []struct {
	data string
	ok   bool
}{
	{`foo`, false},
	{`}{`, false},
	{`{]`, false},
	{`{}`, true},
	{`{"foo":"bar"}`, true},
	{`{"foo":"bar","bar":{"baz":["qux"]}}`, true},
}

func TestValid(t *testing.T) {
	for _, tt := range validTests {
		assert.Equal(t, tt.ok, Valid([]byte(tt.data)), "Valid(%#q)", tt.data)
	}
}

// Tests of simple examples.

type example struct {
	compact string
	indent  string
}

var examples = []example{
	{`1`, `1`},
	{`{}`, `{}`},
	{`[]`, `[]`},
	{`{"":2}`, "{\n\t\"\": 2\n}"},
	{`[3]`, "[\n\t3\n]"},
	{`[1,2,3]`, "[\n\t1,\n\t2,\n\t3\n]"},
	{`{"x":1}`, "{\n\t\"x\": 1\n}"},
	{ex1, ex1i},
	{"{\"\":\"<>&\u2028\u2029\"}", "{\n\t\"\": \"<>&\u2028\u2029\"\n}"}, // See golang.org/issue/34070
}

var ex1 = `[true,false,null,"x",1,1.5,0,-5e+2]`

var ex1i = `[
	true,
	false,
	null,
	"x",
	1,
	1.5,
	0,
	-5e+2
]`

func TestCompact(t *testing.T) {
	var buf bytes.Buffer
	for _, tt := range examples {
		buf.Reset()
		assert.NoError(t, Compact(&buf, []byte(tt.compact)))
		assert.Equal(t, tt.compact, buf.String())

		buf.Reset()
		assert.NoError(t, Compact(&buf, []byte(tt.indent)))
		assert.Equal(t, tt.compact, buf.String())
	}
}

func TestCompactSeparators(t *testing.T) {
	// U+2028 and U+2029 should be escaped inside strings.
	// They should not appear outside strings.
	tests := []struct {
		in, compact string
	}{
		{"{\"\u2028\": 1}", "{\"\u2028\":1}"},
		{"{\"\u2029\" :2}", "{\"\u2029\":2}"},
	}
	for _, tt := range tests {
		var buf bytes.Buffer
		assert.NoError(t, Compact(&buf, []byte(tt.in)))
		assert.Equal(t, tt.compact, buf.String())
	}
}

func TestIndent(t *testing.T) {
	var buf bytes.Buffer
	for _, tt := range examples {
		buf.Reset()
		assert.NoError(t, Indent(&buf, []byte(tt.indent), "", "\t"))
		assert.Equal(t, tt.indent, buf.String())

		buf.Reset()
		assert.NoError(t, Indent(&buf, []byte(tt.compact), "", "\t"))
		assert.Equal(t, tt.indent, buf.String())
	}
}

// Tests of a large random structure.

func TestCompactBig(t *testing.T) {
	initBig()
	var buf bytes.Buffer
	require.NoError(t, Compact(&buf, jsonBig))
	assert.Equal(t, jsonBig, buf.Bytes())
}

func TestIndentBig(t *testing.T) {
	t.Parallel()
	initBig()
	var buf bytes.Buffer
	require.NoError(t, Indent(&buf, jsonBig, "", "\t"))
	b := buf.Bytes()

	// jsonBig is compact (no unnecessary spaces);
	// indenting should make it bigger
	require.Greater(t, len(b), len(jsonBig))

	// should be idempotent
	var buf1 bytes.Buffer
	require.NoError(t, Indent(&buf1, b, "", "\t"))
	b1 := buf1.Bytes()
	assert.Equal(t, b, b1, "Indent(Indent(jsonBig)) != Indent(jsonBig)")

	// should get back to original
	buf1.Reset()
	require.NoError(t, Compact(&buf1, b))
	b1 = buf1.Bytes()
	assert.Equal(t, jsonBig, b1)
}

type indentErrorTest struct {
	in  string
	err error
}

var indentErrorTests = []indentErrorTest{
	{`{"X": "foo", "Y"}`, &SyntaxError{"invalid character '}' after object key", 17}},
	{`{"X": "foo" "Y": "bar"}`, &SyntaxError{"invalid character '\"' after object key:value pair", 13}},
}

func TestIndentErrors(t *testing.T) {
	for i, tt := range indentErrorTests {
		slice := make([]uint8, 0)
		buf := bytes.NewBuffer(slice)
		if err := Indent(buf, []uint8(tt.in), "", ""); err != nil {
			if !reflect.DeepEqual(err, tt.err) {
				t.Errorf("#%d: Indent: %#v", i, err)
				continue
			}
		}
	}
}

// Generate a random JSON object.

var jsonBig []byte

func initBig() {
	n := 10000
	if testing.Short() {
		n = 100
	}
	buf := mocks.NewBuildCloser()
	err := Marshal(genValue(n), buf)
	if err != nil {
		panic(err)
	}
	jsonBig = []byte(buf.String())
}

func genValue(n int) interface{} {
	if n > 1 {
		switch rand.Intn(2) {
		case 0:
			return genArray(n)
		case 1:
			return genMap(n)
		}
	}
	switch rand.Intn(3) {
	case 0:
		return rand.Intn(2) == 0
	case 1:
		return rand.NormFloat64()
	case 2:
		return genString(30)
	}
	panic("unreachable")
}

func genString(stddev float64) string {
	n := int(math.Abs(rand.NormFloat64()*stddev + stddev/2))
	c := make([]rune, n)
	for i := range c {
		f := math.Abs(rand.NormFloat64()*64 + 32)
		if f > 0x10ffff {
			f = 0x10ffff
		}
		c[i] = rune(f)
	}
	return string(c)
}

func genArray(n int) []interface{} {
	f := int(math.Abs(rand.NormFloat64()) * math.Min(10, float64(n/2)))
	if f > n {
		f = n
	}
	if f < 1 {
		f = 1
	}
	x := make([]interface{}, f)
	for i := range x {
		x[i] = genValue(((i+1)*n)/f - (i*n)/f)
	}
	return x
}

func genMap(n int) map[string]interface{} {
	f := int(math.Abs(rand.NormFloat64()) * math.Min(10, float64(n/2)))
	if f > n {
		f = n
	}
	if n > 0 && f == 0 {
		f = 1
	}
	x := make(map[string]interface{})
	for i := 0; i < f; i++ {
		x[genString(10)] = genValue(((i+1)*n)/f - (i*n)/f)
	}
	return x
}
