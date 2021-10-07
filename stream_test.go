// Copyright 2010 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package json

import (
	"bytes"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	"github.com/EinKrebs/json/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Test values for the stream test.
// One of each JSON kind.
var streamTest = []interface{}{
	0.1,
	"hello",
	nil,
	true,
	false,
	[]interface{}{"a", "b", "c"},
	map[string]interface{}{"K": "Kelvin", "ß": "long s"},
	3.14, // another value to make sure something can follow map
}

var streamEncoded = `0.1
"hello"
null
true
false
["a","b","c"]
{"ß":"long s","K":"Kelvin"}
3.14
`

func TestEncoder(t *testing.T) {
	for i := 0; i <= len(streamTest); i++ {
		var buf bytes.Buffer
		enc := NewEncoder(&buf)
		// Check that enc.SetIndent("", "") turns off indentation.
		enc.SetIndent(">", ".")
		enc.SetIndent("", "")
		for _, v := range streamTest[0:i] {
			require.NoError(t, enc.Encode(v))
		}
		assert.Equal(t, nlines(streamEncoded, i), buf.String())
	}
}

var streamEncodedIndent = `0.1
"hello"
null
true
false
[
>."a",
>."b",
>."c"
>]
{
>."ß": "long s",
>."K": "Kelvin"
>}
3.14
`

func TestEncoderIndent(t *testing.T) {
	var buf bytes.Buffer
	enc := NewEncoder(&buf)
	enc.SetIndent(">", ".")
	for _, v := range streamTest {
		_ = enc.Encode(v)
	}
	assert.Equal(t, streamEncodedIndent, buf.String())
}

type strMarshaler string

func (s strMarshaler) MarshalJSON() ([]byte, error) {
	return []byte(s), nil
}

type strPtrMarshaler string

func (s *strPtrMarshaler) MarshalJSON() ([]byte, error) {
	return []byte(*s), nil
}

func TestEncoderSetEscapeHTML(t *testing.T) {
	var c C
	var ct CText
	var tagStruct struct {
		Valid   int `json:"<>&#! "`
		Invalid int `json:"\\"` //nolint:staticcheck
	}

	// This case is particularly interesting, as we force the encoder to
	// take the address of the Ptr field to use its MarshalJSON method. This
	// is why the '&' is important.
	marshalerStruct := &struct {
		NonPtr strMarshaler
		Ptr    strPtrMarshaler
	}{`"<str>"`, `"<str>"`}

	// https://golang.org/issue/34154
	stringOption := struct {
		Bar string `json:"bar,string"`
	}{`<html>foobar</html>`}

	for _, tt := range []struct {
		name       string
		v          interface{}
		wantEscape string
		want       string
	}{
		{"c", c, `"\u003c\u0026\u003e"`, `"<&>"`},
		{"ct", ct, `"\"\u003c\u0026\u003e\""`, `"\"<&>\""`},
		{`"<&>"`, "<&>", `"\u003c\u0026\u003e"`, `"<&>"`},
		{
			"tagStruct", tagStruct,
			`{"\u003c\u003e\u0026#! ":0,"Invalid":0}`,
			`{"<>&#! ":0,"Invalid":0}`,
		},
		{
			`"<str>"`, marshalerStruct,
			`{"NonPtr":"\u003cstr\u003e","Ptr":"\u003cstr\u003e"}`,
			`{"NonPtr":"<str>","Ptr":"<str>"}`,
		},
		{
			"stringOption", stringOption,
			`{"bar":"\"\\u003chtml\\u003efoobar\\u003c/html\\u003e\""}`,
			`{"bar":"\"<html>foobar</html>\""}`,
		},
	} {
		var buf bytes.Buffer
		enc := NewEncoder(&buf)
		assert.NoError(t, enc.Encode(tt.v))
		assert.Equal(t, tt.wantEscape, strings.TrimSpace(buf.String()))
		buf.Reset()
		enc.SetEscapeHTML(false)
		assert.NoError(t, enc.Encode(tt.v))
		assert.Equal(t, tt.want, strings.TrimSpace(buf.String()))
	}
}

func TestDecoder(t *testing.T) {
	for i := 0; i <= len(streamTest); i++ {
		// Use stream without newlines as input,
		// just to stress the decoder even more.
		// Our test input does not include back-to-back numbers.
		// Otherwise, stripping the newlines would
		// merge two adjacent JSON values.
		var buf bytes.Buffer
		for _, c := range nlines(streamEncoded, i) {
			if c != '\n' {
				buf.WriteRune(c)
			}
		}
		out := make([]interface{}, i)
		dec := NewDecoder(&buf)
		for j := range out {
			require.NoError(t, dec.Decode(&out[j]))
		}
		if !reflect.DeepEqual(out, streamTest[0:i]) {
			t.Errorf("decoding %d items: mismatch", i)
			for j := range out {
				if !reflect.DeepEqual(out[j], streamTest[j]) {
					t.Errorf("#%d: have %v want %v", j, out[j], streamTest[j])
				}
			}
			break
		}
	}
}

func TestDecoderBuffered(t *testing.T) {
	r := strings.NewReader(`{"Name": "Gopher"} extra `)
	var m struct {
		Name string
	}
	d := NewDecoder(r)
	err := d.Decode(&m)
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, "Gopher", m.Name)
	rest, err := io.ReadAll(d.Buffered())
	require.NoError(t, err)
	assert.Equal(t, " extra ", string(rest))
}

func nlines(s string, n int) string {
	if n <= 0 {
		return ""
	}
	for i, c := range s {
		if c == '\n' {
			if n--; n == 0 {
				return s[0 : i+1]
			}
		}
	}
	return s
}

func TestRawMessage(t *testing.T) {
	var data struct {
		X  float64
		Id RawMessage
		Y  float32
	}
	const raw = `["\u0056",null]`
	const msg = `{"X":0.1,"Id":["\u0056",null],"Y":0.2}`
	require.NoError(t, Unmarshal(strings.NewReader(msg), &data))
	require.Equal(t, raw, string(data.Id))

	buf := mocks.NewBuildCloser()
	require.NoError(t, Marshal(&data, buf))
	assert.Equal(t, msg, buf.String())
}

func TestNullRawMessage(t *testing.T) {
	var data struct {
		X     float64
		Id    RawMessage
		IdPtr *RawMessage
		Y     float32
	}
	const msg = `{"X":0.1,"Id":null,"IdPtr":null,"Y":0.2}`
	require.NoError(t, Unmarshal(strings.NewReader(msg), &data))
	require.Equal(t, "null", string(data.Id))
	require.Equal(t, (*RawMessage)(nil), data.IdPtr)
	buf := mocks.NewBuildCloser()
	require.NoError(t, Marshal(&data, buf))
	assert.Equal(t, msg, buf.String())
}

var blockingTests = []string{
	`{"x": 1}`,
	`[1, 2, 3]`,
}

func TestBlocking(t *testing.T) {
	for _, enc := range blockingTests {
		r, w := net.Pipe()
		go func() {
			_, _ = w.Write([]byte(enc))
		}()
		var val interface{}

		// If Decode reads beyond what w.Write writes above,
		// it will block, and the test will deadlock.
		assert.NoError(t, NewDecoder(r).Decode(&val))
		r.Close()
		w.Close()
	}
}

func BenchmarkEncoderEncode(b *testing.B) {
	b.ReportAllocs()
	type T struct {
		X, Y string
	}
	v := &T{"foo", "bar"}
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			require.NoError(b, NewEncoder(io.Discard).Encode(v))
		}
	})
}

type tokenStreamCase struct {
	json      string
	expTokens []interface{}
}

type decodeThis struct {
	v interface{}
}

var tokenStreamCases = []tokenStreamCase{
	// streaming token cases
	{json: `10`, expTokens: []interface{}{float64(10)}},
	{json: ` [10] `, expTokens: []interface{}{
		Delim('['), float64(10), Delim(']')}},
	{json: ` [false,10,"b"] `, expTokens: []interface{}{
		Delim('['), false, float64(10), "b", Delim(']')}},
	{json: `{ "a": 1 }`, expTokens: []interface{}{
		Delim('{'), "a", float64(1), Delim('}')}},
	{json: `{"a": 1, "b":"3"}`, expTokens: []interface{}{
		Delim('{'), "a", float64(1), "b", "3", Delim('}')}},
	{json: ` [{"a": 1},{"a": 2}] `, expTokens: []interface{}{
		Delim('['),
		Delim('{'), "a", float64(1), Delim('}'),
		Delim('{'), "a", float64(2), Delim('}'),
		Delim(']')}},
	{json: `{"obj": {"a": 1}}`, expTokens: []interface{}{
		Delim('{'), "obj", Delim('{'), "a", float64(1), Delim('}'),
		Delim('}')}},
	{json: `{"obj": [{"a": 1}]}`, expTokens: []interface{}{
		Delim('{'), "obj", Delim('['),
		Delim('{'), "a", float64(1), Delim('}'),
		Delim(']'), Delim('}')}},

	// streaming tokens with intermittent Decode()
	{json: `{ "a": 1 }`, expTokens: []interface{}{
		Delim('{'), "a",
		decodeThis{float64(1)},
		Delim('}')}},
	{json: ` [ { "a" : 1 } ] `, expTokens: []interface{}{
		Delim('['),
		decodeThis{map[string]interface{}{"a": float64(1)}},
		Delim(']')}},
	{json: ` [{"a": 1},{"a": 2}] `, expTokens: []interface{}{
		Delim('['),
		decodeThis{map[string]interface{}{"a": float64(1)}},
		decodeThis{map[string]interface{}{"a": float64(2)}},
		Delim(']')}},
	{json: `{ "obj" : [ { "a" : 1 } ] }`, expTokens: []interface{}{
		Delim('{'), "obj", Delim('['),
		decodeThis{map[string]interface{}{"a": float64(1)}},
		Delim(']'), Delim('}')}},

	{json: `{"obj": {"a": 1}}`, expTokens: []interface{}{
		Delim('{'), "obj",
		decodeThis{map[string]interface{}{"a": float64(1)}},
		Delim('}')}},
	{json: `{"obj": [{"a": 1}]}`, expTokens: []interface{}{
		Delim('{'), "obj",
		decodeThis{[]interface{}{
			map[string]interface{}{"a": float64(1)},
		}},
		Delim('}')}},
	{json: ` [{"a": 1} {"a": 2}] `, expTokens: []interface{}{
		Delim('['),
		decodeThis{map[string]interface{}{"a": float64(1)}},
		decodeThis{&SyntaxError{"expected comma after array element", 11}},
	}},
	{json: `{ "` + strings.Repeat("a", 513) + `" 1 }`, expTokens: []interface{}{
		Delim('{'), strings.Repeat("a", 513),
		decodeThis{&SyntaxError{"expected colon after object key", 518}},
	}},
	{json: `{ "\a" }`, expTokens: []interface{}{
		Delim('{'),
		&SyntaxError{"invalid character 'a' in string escape code", 3},
	}},
	{json: ` \a`, expTokens: []interface{}{
		&SyntaxError{"invalid character '\\\\' looking for beginning of value", 1},
	}},
}

func TestDecodeInStream(t *testing.T) {
	for ci, tcase := range tokenStreamCases {

		dec := NewDecoder(strings.NewReader(tcase.json))
		for i, etk := range tcase.expTokens {

			var tk interface{}
			var err error

			if dt, ok := etk.(decodeThis); ok {
				etk = dt.v
				err = dec.Decode(&tk)
			} else {
				tk, err = dec.Token()
			}
			if experr, ok := etk.(error); ok {
				if err == nil || !reflect.DeepEqual(err, experr) {
					t.Errorf("case %v: Expected error %#v in %q, but was %#v", ci, experr, tcase.json, err)
				}
				break
			} else if err == io.EOF {
				t.Errorf("case %v: Unexpected EOF in %q", ci, tcase.json)
				break
			} else if err != nil {
				t.Errorf("case %v: Unexpected error '%#v' in %q", ci, err, tcase.json)
				break
			}
			if !reflect.DeepEqual(tk, etk) {
				t.Errorf(`case %v: %q @ %v expected %T(%v) was %T(%v)`, ci, tcase.json, i, etk, etk, tk, tk)
				break
			}
		}
	}
}

// Test from golang.org/issue/11893
func TestHTTPDecoding(t *testing.T) {
	const raw = `{ "foo": "bar" }`

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(raw))
	}))
	defer ts.Close()
	res, err := http.Get(ts.URL)
	assert.NoError(t, err)
	defer res.Body.Close()

	foo := struct {
		Foo string
	}{}

	d := NewDecoder(res.Body)
	require.NoError(t, d.Decode(&foo))
	assert.Equal(t, "bar", foo.Foo)

	// make sure we get the EOF the second time
	assert.ErrorIs(t, d.Decode(&foo), io.EOF)
}
