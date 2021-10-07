// Copyright 2011 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package json

import (
	"bytes"
	"encoding"
	"encoding/json"
	"fmt"
	"log"
	"math"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"unicode"

	"github.com/EinKrebs/json/mocks"
	"github.com/stretchr/testify/assert"

	"github.com/stretchr/testify/require"
)

type Optionals struct {
	Sr string `json:"sr"`
	So string `json:"so,omitempty"`
	Sw string `json:"-"`

	Ir int `json:"omitempty"` // actually named omitempty, not an option
	Io int `json:"io,omitempty"`

	Slr []string `json:"slr,random"`
	Slo []string `json:"slo,omitempty"`

	Mr map[string]interface{} `json:"mr"`
	Mo map[string]interface{} `json:",omitempty"`

	Fr float64 `json:"fr"`
	Fo float64 `json:"fo,omitempty"`

	Br bool `json:"br"`
	Bo bool `json:"bo,omitempty"`

	Ur uint `json:"ur"`
	Uo uint `json:"uo,omitempty"`

	Str struct{} `json:"str"`
	Sto struct{} `json:"sto,omitempty"`
}

// byte slices are special even if they're renamed types.
type renamedByte byte
type renamedByteSlice []byte
type renamedRenamedByteSlice []renamedByte

func TestEncodeRenamedByteSlice(t *testing.T) {
	s := renamedByteSlice("abc")
	buf := mocks.NewBuildCloser()
	require.NoError(t, Marshal(s, buf))
	expect := `"YWJj"`
	assert.Equal(t, buf.String(), expect, " got %s want %s", buf.String(), expect)

	r := renamedRenamedByteSlice("abc")
	buf.Reset()
	require.NoError(t, Marshal(r, buf))
	assert.Equal(t, buf.String(), expect, " got %s want %s", buf.String(), expect)
}

type SamePointerNoCycle struct {
	Ptr1, Ptr2 *SamePointerNoCycle
}

var samePointerNoCycle = &SamePointerNoCycle{}

type PointerCycle struct {
	Ptr *PointerCycle
}

var pointerCycle = &PointerCycle{}

type PointerCycleIndirect struct {
	Ptrs []interface{}
}

type RecursiveSlice []RecursiveSlice

var (
	pointerCycleIndirect = &PointerCycleIndirect{}
	mapCycle             = make(map[string]interface{})
	sliceCycle           = []interface{}{nil}
	sliceNoCycle         = []interface{}{nil, nil}
	recursiveSliceCycle  = []RecursiveSlice{nil}
)

func init() {
	ptr := &SamePointerNoCycle{}
	samePointerNoCycle.Ptr1 = ptr
	samePointerNoCycle.Ptr2 = ptr

	pointerCycle.Ptr = pointerCycle
	pointerCycleIndirect.Ptrs = []interface{}{pointerCycleIndirect}

	mapCycle["x"] = mapCycle
	sliceCycle[0] = sliceCycle
	sliceNoCycle[1] = sliceNoCycle[:1]
	for i := startDetectingCyclesAfter; i > 0; i-- {
		sliceNoCycle = []interface{}{sliceNoCycle}
	}
	recursiveSliceCycle[0] = recursiveSliceCycle
}

func TestSamePointerNoCycle(t *testing.T) {
	require.NoError(t, Marshal(samePointerNoCycle, mocks.NewBuildCloser()))
}

func TestSliceNoCycle(t *testing.T) {
	require.NoError(t, Marshal(sliceNoCycle, mocks.NewBuildCloser()))
}

var unsupportedValues = []interface{}{
	math.NaN(),
	math.Inf(-1),
	math.Inf(1),
	pointerCycle,
	pointerCycleIndirect,
	mapCycle,
	sliceCycle,
	recursiveSliceCycle,
}

func TestUnsupportedValues(t *testing.T) {
	for _, v := range unsupportedValues {
		err := Marshal(v, mocks.NewBuildCloser())
		require.Error(t, err)
		_, ok := err.(*UnsupportedValueError)
		assert.True(t, ok, "for %v, got %T want UnsupportedValueError", v, err)
	}
}

// Ref has Marshaler and Unmarshaler methods with pointer receiver.
type Ref int

func (*Ref) MarshalJSON() ([]byte, error) {
	return []byte(`"ref"`), nil
}

func (r *Ref) UnmarshalJSON([]byte) error {
	*r = 12
	return nil
}

// Val has Marshaler methods with value receiver.
type Val int

func (Val) MarshalJSON() ([]byte, error) {
	return []byte(`"val"`), nil
}

// RefText has Marshaler and Unmarshaler methods with pointer receiver.
type RefText int

func (*RefText) MarshalText() ([]byte, error) {
	return []byte(`"ref"`), nil
}

func (r *RefText) UnmarshalText([]byte) error {
	*r = 13
	return nil
}

// ValText has Marshaler methods with value receiver.
type ValText int

func (ValText) MarshalText() ([]byte, error) {
	return []byte(`"val"`), nil
}

func TestRefValMarshal(t *testing.T) {
	var s = struct {
		R0 Ref
		R1 *Ref
		R2 RefText
		R3 *RefText
		V0 Val
		V1 *Val
		V2 ValText
		V3 *ValText
	}{
		R0: 12,
		R1: new(Ref),
		R2: 14,
		R3: new(RefText),
		V0: 13,
		V1: new(Val),
		V2: 15,
		V3: new(ValText),
	}
	const want = `{"R0":"ref","R1":"ref","R2":"\"ref\"","R3":"\"ref\"","V0":"val","V1":"val","V2":"\"val\"","V3":"\"val\""}`
	buf := mocks.NewBuildCloser()
	require.NoError(t, Marshal(&s, buf))
	assert.Equal(t, want, buf.String())
}

// C implements Marshaler and returns unescaped JSON.
type C int

func (C) MarshalJSON() ([]byte, error) {
	return []byte(`"<&>"`), nil
}

// CText implements Marshaler and returns unescaped text.
type CText int

func (CText) MarshalText() ([]byte, error) {
	return []byte(`"<&>"`), nil
}

func TestMarshalerEscaping(t *testing.T) {
	var c C
	want := `"\u003c\u0026\u003e"`
	buf := mocks.NewBuildCloser()
	require.NoError(t, Marshal(c, buf))
	assert.Equal(t, want, buf.String())

	var ct CText
	want = `"\"\u003c\u0026\u003e\""`
	buf.Reset()
	require.NoError(t, Marshal(ct, buf))
	assert.Equal(t, want, buf.String())
}

func TestAnonymousFields(t *testing.T) {
	tests := []struct {
		label     string             // Test name
		makeInput func() interface{} // Function to create input value
		want      string             // Expected JSON output
	}{{
		// Both S1 and S2 have a field named X. From the perspective of S,
		// it is ambiguous which one X refers to.
		// This should not serialize either field.
		label: "AmbiguousField",
		makeInput: func() interface{} {
			type (
				S1 struct{ x, X int }
				S2 struct{ x, X int }
				S  struct {
					S1
					S2
				}
			)
			return S{S1{1, 2}, S2{3, 4}}
		},
		want: `{}`,
	}, {
		label: "DominantField",
		// Both S1 and S2 have a field named X, but since S has an X field as
		// well, it takes precedence over S1.X and S2.X.
		makeInput: func() interface{} {
			type (
				S1 struct{ x, X int }
				S2 struct{ x, X int }
				S  struct {
					S1
					S2
					x, X int
				}
			)
			return S{S1{1, 2}, S2{3, 4}, 5, 6}
		},
		want: `{"X":6}`,
	}, {
		// Unexported embedded field of non-struct type should not be serialized.
		label: "UnexportedEmbeddedInt",
		makeInput: func() interface{} {
			type (
				myInt int
				S     struct{ myInt }
			)
			return S{5}
		},
		want: `{}`,
	}, {
		// Exported embedded field of non-struct type should be serialized.
		label: "ExportedEmbeddedInt",
		makeInput: func() interface{} {
			type (
				MyInt int
				S     struct{ MyInt }
			)
			return S{5}
		},
		want: `{"MyInt":5}`,
	}, {
		// Unexported embedded field of pointer to non-struct type
		// should not be serialized.
		label: "UnexportedEmbeddedIntPointer",
		makeInput: func() interface{} {
			type (
				myInt int
				S     struct{ *myInt }
			)
			s := S{new(myInt)}
			*s.myInt = 5
			return s
		},
		want: `{}`,
	}, {
		// Exported embedded field of pointer to non-struct type
		// should be serialized.
		label: "ExportedEmbeddedIntPointer",
		makeInput: func() interface{} {
			type (
				MyInt int
				S     struct{ *MyInt }
			)
			s := S{new(MyInt)}
			*s.MyInt = 5
			return s
		},
		want: `{"MyInt":5}`,
	}, {
		// Exported fields of embedded structs should have their
		// exported fields be serialized regardless of whether the struct types
		// themselves are exported.
		label: "EmbeddedStruct",
		makeInput: func() interface{} {
			type (
				s1 struct{ x, X int }
				S2 struct{ y, Y int }
				S  struct {
					s1
					S2
				}
			)
			return S{s1{1, 2}, S2{3, 4}}
		},
		want: `{"X":2,"Y":4}`,
	}, {
		// Exported fields of pointers to embedded structs should have their
		// exported fields be serialized regardless of whether the struct types
		// themselves are exported.
		label: "EmbeddedStructPointer",
		makeInput: func() interface{} {
			type (
				s1 struct{ x, X int }
				S2 struct{ y, Y int }
				S  struct {
					*s1
					*S2
				}
			)
			return S{&s1{1, 2}, &S2{3, 4}}
		},
		want: `{"X":2,"Y":4}`,
	}, {
		// Exported fields on embedded unexported structs at multiple levels
		// of nesting should still be serialized.
		label: "NestedStructAndInts",
		makeInput: func() interface{} {
			type (
				MyInt1 int
				MyInt2 int
				myInt  int
				s2     struct {
					MyInt2
					myInt
				}
				s1 struct {
					MyInt1
					myInt
					s2
				}
				S struct {
					s1
					myInt
				}
			)
			return S{s1{1, 2, s2{3, 4}}, 6}
		},
		want: `{"MyInt1":1,"MyInt2":3}`,
	}, {
		// If an anonymous struct pointer field is nil, we should ignore
		// the embedded fields behind it. Not properly doing so may
		// result in the wrong output or reflect panics.
		label: "EmbeddedFieldBehindNilPointer",
		makeInput: func() interface{} {
			type (
				S2 struct{ Field string }
				S  struct{ *S2 }
			)
			return S{}
		},
		want: `{}`,
	}}

	for _, tt := range tests {
		t.Run(tt.label, func(t *testing.T) {
			buf := mocks.NewBuildCloser()
			require.NoError(t, Marshal(tt.makeInput(), buf))
			assert.Equal(t, tt.want, buf.String())
		})
	}
}

type BugA struct {
	S string
}

type BugB struct {
	BugA
	S string
}

type BugC struct {
	S string
}

// Legal Go: We never use the repeated embedded field (S).
type BugX struct {
	A int
	BugA
	BugB
}

// golang.org/issue/16042.
// Even if a nil interface value is passed in, as long as
// it implements Marshaler, it should be marshaled.
type nilJSONMarshaler string

func (nm *nilJSONMarshaler) MarshalJSON() ([]byte, error) {
	if nm == nil {
		return json.Marshal("0zenil0")
	}
	return json.Marshal("zenil:" + string(*nm))
}

// golang.org/issue/34235.
// Even if a nil interface value is passed in, as long as
// it implements encoding.TextMarshaler, it should be marshaled.
type nilTextMarshaler string

func (nm *nilTextMarshaler) MarshalText() ([]byte, error) {
	if nm == nil {
		return []byte("0zenil0"), nil
	}
	return []byte("zenil:" + string(*nm)), nil
}

// See golang.org/issue/16042 and golang.org/issue/34235.
func TestNilMarshal(t *testing.T) {
	testCases := []struct {
		v    interface{}
		want string
	}{
		{v: nil, want: `null`},
		{v: new(float64), want: `0`},
		{v: []interface{}(nil), want: `null`},
		{v: []string(nil), want: `null`},
		{v: map[string]string(nil), want: `null`},
		{v: []byte(nil), want: `null`},
		{v: struct{ M string }{"gopher"}, want: `{"M":"gopher"}`},
		{v: struct{ M Marshaler }{}, want: `{"M":null}`},
		{v: struct{ M Marshaler }{(*nilJSONMarshaler)(nil)}, want: `{"M":"0zenil0"}`},
		{v: struct{ M interface{} }{(*nilJSONMarshaler)(nil)}, want: `{"M":null}`},
		{v: struct{ M encoding.TextMarshaler }{}, want: `{"M":null}`},
		{v: struct{ M encoding.TextMarshaler }{(*nilTextMarshaler)(nil)}, want: `{"M":"0zenil0"}`},
		{v: struct{ M interface{} }{(*nilTextMarshaler)(nil)}, want: `{"M":null}`},
	}

	for _, tt := range testCases {
		buf := mocks.NewBuildCloser()
		require.NoError(t, Marshal(tt.v, buf))
		assert.Equal(t, tt.want, buf.String())
	}
}

// Issue 5245.
func TestEmbeddedBug(t *testing.T) {
	v := BugB{
		BugA{"A"},
		"B",
	}
	buf := mocks.NewBuildCloser()
	require.NoError(t, Marshal(v, buf))
	want := `{"S":"B"}`
	assert.Equal(t, want, buf.String())
	// Now check that the duplicate field, S, does not appear.
	x := BugX{
		A: 23,
	}
	buf.Reset()
	require.NoError(t, Marshal(x, buf))
	want = `{"A":23}`
	assert.Equal(t, want, buf.String())
}

type BugD struct { // Same as BugA after tagging.
	XXX string `json:"S"`
}

// BugD's tagged S field should dominate BugA's.
type BugY struct {
	BugA
	BugD
}

// Test that a field with a tag dominates untagged fields.
func TestTaggedFieldDominates(t *testing.T) {
	v := BugY{
		BugA{"BugA"},
		BugD{"BugD"},
	}
	buf := mocks.NewBuildCloser()
	require.NoError(t, Marshal(v, buf))
	want := `{"S":"BugD"}`
	assert.Equal(t, want, buf.String())
}

// There are no tags here, so S should not appear.
type BugZ struct {
	BugA
	BugC
	BugY // Contains a tagged S field through BugD; should not dominate.
}

func TestDuplicatedFieldDisappears(t *testing.T) {
	v := BugZ{
		BugA{"BugA"},
		BugC{"BugC"},
		BugY{
			BugA{"nested BugA"},
			BugD{"nested BugD"},
		},
	}
	buf := mocks.NewBuildCloser()
	require.NoError(t, Marshal(v, buf))
	want := `{}`
	assert.Equal(t, want, buf.String())
}

func TestStringBytes(t *testing.T) {
	t.Parallel()
	// Test that encodeState.stringBytes and encodeState.string use the same encoding.
	var r []rune
	for i := '\u0000'; i <= unicode.MaxRune; i++ {
		if testing.Short() && i > 1000 {
			i = unicode.MaxRune
		}
		r = append(r, i)
	}
	s := string(r) + "\xff\xff\xffhello" // some invalid UTF-8 too

	for _, escapeHTML := range []bool{true, false} {
		buf := strings.Builder{}
		es := newWriterEncodeState(&buf)
		require.NoError(t, es.string(s, escapeHTML))

		bufBytes := strings.Builder{}
		esBytes := newWriterEncodeState(&bufBytes)
		require.NoError(t, esBytes.stringBytes([]byte(s), escapeHTML))

		enc := buf.String()
		encBytes := bufBytes.String()
		if enc != encBytes {
			i := 0
			for i < len(enc) && i < len(encBytes) && enc[i] == encBytes[i] {
				i++
			}
			enc = enc[i:]
			encBytes = encBytes[i:]
			i = 0
			for i < len(enc) && i < len(encBytes) && enc[len(enc)-i-1] == encBytes[len(encBytes)-i-1] {
				i++
			}
			enc = enc[:len(enc)-i]
			encBytes = encBytes[:len(encBytes)-i]

			if len(enc) > 20 {
				enc = enc[:20] + "..."
			}
			if len(encBytes) > 20 {
				encBytes = encBytes[:20] + "..."
			}

			t.Errorf("with escapeHTML=%t, encodings differ at %#q vs %#q",
				escapeHTML, enc, encBytes)
		}
	}
}

func TestIssue10281(t *testing.T) {
	type Foo struct {
		N Number
	}
	x := Foo{Number(`invalid`)}

	assert.Error(t, Marshal(&x, mocks.NewBuildCloser()))
}

func TestHTMLEscape(t *testing.T) {
	var b, want bytes.Buffer
	m := `{"M":"<html>foo &` + "\xe2\x80\xa8 \xe2\x80\xa9" + `</html>"}`
	want.Write([]byte(`{"M":"\u003chtml\u003efoo \u0026\u2028 \u2029\u003c/html\u003e"}`))
	HTMLEscape(&b, []byte(m))
	assert.Equal(t, want.Bytes(), b.Bytes())
}

// golang.org/issue/8582
func TestEncodePointerString(t *testing.T) {
	type stringPointer struct {
		N *int64 `json:"n,string"`
	}
	var n int64 = 42
	buf := mocks.NewBuildCloser()
	require.NoError(t, Marshal(stringPointer{N: &n}, buf))
	assert.Equal(t, `{"n":"42"}`, buf.String())
	var back stringPointer
	require.NoError(t, Unmarshal(strings.NewReader(buf.String()), &back))
	require.NotEqual(t, (*int64)(nil), back.N)
	assert.Equal(t, int64(42), *back.N)
}

var encodeStringTests = []struct {
	in  string
	out string
}{
	{"\x00", `"\u0000"`},
	{"\x01", `"\u0001"`},
	{"\x02", `"\u0002"`},
	{"\x03", `"\u0003"`},
	{"\x04", `"\u0004"`},
	{"\x05", `"\u0005"`},
	{"\x06", `"\u0006"`},
	{"\x07", `"\u0007"`},
	{"\x08", `"\u0008"`},
	{"\x09", `"\t"`},
	{"\x0a", `"\n"`},
	{"\x0b", `"\u000b"`},
	{"\x0c", `"\u000c"`},
	{"\x0d", `"\r"`},
	{"\x0e", `"\u000e"`},
	{"\x0f", `"\u000f"`},
	{"\x10", `"\u0010"`},
	{"\x11", `"\u0011"`},
	{"\x12", `"\u0012"`},
	{"\x13", `"\u0013"`},
	{"\x14", `"\u0014"`},
	{"\x15", `"\u0015"`},
	{"\x16", `"\u0016"`},
	{"\x17", `"\u0017"`},
	{"\x18", `"\u0018"`},
	{"\x19", `"\u0019"`},
	{"\x1a", `"\u001a"`},
	{"\x1b", `"\u001b"`},
	{"\x1c", `"\u001c"`},
	{"\x1d", `"\u001d"`},
	{"\x1e", `"\u001e"`},
	{"\x1f", `"\u001f"`},
}

func TestEncodeString(t *testing.T) {
	for _, tt := range encodeStringTests {
		buf := mocks.NewBuildCloser()
		require.NoError(t, Marshal(tt.in, buf))
		assert.Equal(t, tt.out, buf.String())
	}
}

type jsonbyte byte

func (b jsonbyte) MarshalJSON() ([]byte, error) { return tenc(`{"JB":%d}`, b) }

type textbyte byte

func (b textbyte) MarshalText() ([]byte, error) { return tenc(`TB:%d`, b) }

type jsonint int

func (i jsonint) MarshalJSON() ([]byte, error) { return tenc(`{"JI":%d}`, i) }

type textint int

func (i textint) MarshalText() ([]byte, error) { return tenc(`TI:%d`, i) }

func tenc(format string, a ...interface{}) ([]byte, error) {
	var buf bytes.Buffer
	_, _ = fmt.Fprintf(&buf, format, a...)
	return buf.Bytes(), nil
}

// Issue 13783
func TestEncodeBytekind(t *testing.T) {
	testdata := []struct {
		data interface{}
		want string
	}{
		{byte(7), "7"},
		{jsonbyte(7), `{"JB":7}`},
		{textbyte(4), `"TB:4"`},
		{jsonint(5), `{"JI":5}`},
		{textint(1), `"TI:1"`},
		{[]byte{0, 1}, `"AAE="`},
		{[]jsonbyte{0, 1}, `[{"JB":0},{"JB":1}]`},
		{[][]jsonbyte{{0, 1}, {3}}, `[[{"JB":0},{"JB":1}],[{"JB":3}]]`},
		{[]textbyte{2, 3}, `["TB:2","TB:3"]`},
		{[]jsonint{5, 4}, `[{"JI":5},{"JI":4}]`},
		{[]textint{9, 3}, `["TI:9","TI:3"]`},
		{[]int{9, 3}, `[9,3]`},
	}
	for _, d := range testdata {
		buf := mocks.NewBuildCloser()
		require.NoError(t, Marshal(d.data, buf))
		assert.Equal(t, d.want, buf.String())
	}
}

func TestTextMarshalerMapKeysAreSorted(t *testing.T) {
	buf := mocks.NewBuildCloser()
	require.NoError(t, Marshal(map[unmarshalerText]int{
		{"x", "y"}: 1,
		{"y", "x"}: 2,
		{"a", "z"}: 3,
		{"z", "a"}: 4,
	}, buf))
	const want = `{"a:z":3,"x:y":1,"y:x":2,"z:a":4}`
	assert.Equal(t, want, buf.String())
}

// https://golang.org/issue/33675
func TestNilMarshalerTextMapKey(t *testing.T) {
	buf := mocks.NewBuildCloser()
	require.NoError(t, Marshal(map[*unmarshalerText]int{
		(*unmarshalerText)(nil): 1,
		{"A", "B"}:              2,
	}, buf))
	const want = `{"":1,"A:B":2}`
	assert.Equal(t, want, buf.String())
}

var re = regexp.MustCompile

// syntactic checks on form of marshaled floating point numbers.
var badFloatREs = []*regexp.Regexp{
	re(`p`),                     // no binary exponential notation
	re(`^\+`),                   // no leading + sign
	re(`^-?0[^.]`),              // no unnecessary leading zeros
	re(`^-?\.`),                 // leading zero required before decimal point
	re(`\.(e|$)`),               // no trailing decimal
	re(`\.[0-9]+0(e|$)`),        // no trailing zero in fraction
	re(`^-?(0|[0-9]{2,})\..*e`), // exponential notation must have normalized mantissa
	re(`e[0-9]`),                // positive exponent must be signed
	re(`e[+-]0`),                // exponent must not have leading zeros
	re(`e-[1-6]$`),              // not tiny enough for exponential notation
	re(`e+(.|1.|20)$`),          // not big enough for exponential notation
	re(`^-?0\.0000000`),         // too tiny, should use exponential notation
	re(`^-?[0-9]{22}`),          // too big, should use exponential notation
	re(`[1-9][0-9]{16}[1-9]`),   // too many significant digits in integer
	re(`[1-9][0-9.]{17}[1-9]`),  // too many significant digits in decimal
	// below here for float32 only
	re(`[1-9][0-9]{8}[1-9]`),  // too many significant digits in integer
	re(`[1-9][0-9.]{9}[1-9]`), // too many significant digits in decimal
}

func TestMarshalFloat(t *testing.T) {
	t.Parallel()
	nfail := 0
	test := func(f float64, bits int) {
		vf := interface{}(f)
		if bits == 32 {
			f = float64(float32(f)) // round
			vf = float32(f)
		}
		buf := mocks.NewBuildCloser()
		err := Marshal(vf, buf)
		if err != nil {
			t.Errorf("Marshal(%T(%g)): %v", vf, vf, err)
			nfail++
			return
		}
		out := buf.String()

		// result must convert back to the same float
		g, err := strconv.ParseFloat(out, bits)
		if err != nil {
			t.Errorf("Marshal(%T(%g)) = %q, cannot parse back: %v", vf, vf, out, err)
			nfail++
			return
		}
		if f != g || fmt.Sprint(f) != fmt.Sprint(g) { // fmt.Sprint handles ±0
			t.Errorf("Marshal(%T(%g)) = %q (is %g, not %g)", vf, vf, out, float32(g), vf)
			nfail++
			return
		}

		bad := badFloatREs
		if bits == 64 {
			bad = bad[:len(bad)-2]
		}
		for _, re := range bad {
			if re.MatchString(out) {
				t.Errorf("Marshal(%T(%g)) = %q, must not match /%s/", vf, vf, out, re)
				nfail++
				return
			}
		}
	}

	var (
		bigger  = math.Inf(+1)
		smaller = math.Inf(-1)
	)

	var digits = "1.2345678901234567890123"
	for i := len(digits); i >= 2; i-- {
		if testing.Short() && i < len(digits)-4 {
			break
		}
		for exp := -30; exp <= 30; exp++ {
			for _, sign := range "+-" {
				for bits := 32; bits <= 64; bits += 32 {
					s := fmt.Sprintf("%c%se%d", sign, digits[:i], exp)
					f, err := strconv.ParseFloat(s, bits)
					if err != nil {
						log.Fatal(err)
					}
					next := math.Nextafter
					if bits == 32 {
						next = func(g, h float64) float64 {
							return float64(math.Nextafter32(float32(g), float32(h)))
						}
					}
					test(f, bits)
					test(next(f, bigger), bits)
					test(next(f, smaller), bits)
					if nfail > 50 {
						t.Fatalf("stopping test early")
					}
				}
			}
		}
	}
	test(0, 64)
	test(math.Copysign(0, -1), 64)
	test(0, 32)
	test(math.Copysign(0, -1), 32)
}

func TestMarshalRawMessageValue(t *testing.T) {
	type (
		T1 struct {
			M RawMessage `json:",omitempty"`
		}
		T2 struct {
			M *RawMessage `json:",omitempty"`
		}
	)

	var (
		rawNil   = RawMessage(nil)
		rawEmpty = RawMessage([]byte{})
		rawText  = RawMessage([]byte(`"foo"`))
	)

	tests := []struct {
		in   interface{}
		want string
		ok   bool
	}{
		// Test with nil RawMessage.
		{rawNil, "null", true},
		{&rawNil, "null", true},
		{[]interface{}{rawNil}, "[null]", true},
		{&[]interface{}{rawNil}, "[null]", true},
		{[]interface{}{&rawNil}, "[null]", true},
		{&[]interface{}{&rawNil}, "[null]", true},
		{struct{ M RawMessage }{rawNil}, `{"M":null}`, true},
		{&struct{ M RawMessage }{rawNil}, `{"M":null}`, true},
		{struct{ M *RawMessage }{&rawNil}, `{"M":null}`, true},
		{&struct{ M *RawMessage }{&rawNil}, `{"M":null}`, true},
		{map[string]interface{}{"M": rawNil}, `{"M":null}`, true},
		{&map[string]interface{}{"M": rawNil}, `{"M":null}`, true},
		{map[string]interface{}{"M": &rawNil}, `{"M":null}`, true},
		{&map[string]interface{}{"M": &rawNil}, `{"M":null}`, true},
		{T1{rawNil}, "{}", true},
		{T2{&rawNil}, `{"M":null}`, true},
		{&T1{rawNil}, "{}", true},
		{&T2{&rawNil}, `{"M":null}`, true},

		// Test with empty, but non-nil, RawMessage.
		{rawEmpty, "", false},
		{&rawEmpty, "", false},
		{[]interface{}{rawEmpty}, "", false},
		{&[]interface{}{rawEmpty}, "", false},
		{[]interface{}{&rawEmpty}, "", false},
		{&[]interface{}{&rawEmpty}, "", false},
		{struct{ X RawMessage }{rawEmpty}, "", false},
		{&struct{ X RawMessage }{rawEmpty}, "", false},
		{struct{ X *RawMessage }{&rawEmpty}, "", false},
		{&struct{ X *RawMessage }{&rawEmpty}, "", false},
		{map[string]interface{}{"nil": rawEmpty}, "", false},
		{&map[string]interface{}{"nil": rawEmpty}, "", false},
		{map[string]interface{}{"nil": &rawEmpty}, "", false},
		{&map[string]interface{}{"nil": &rawEmpty}, "", false},
		{T1{rawEmpty}, "{}", true},
		{T2{&rawEmpty}, "", false},
		{&T1{rawEmpty}, "{}", true},
		{&T2{&rawEmpty}, "", false},

		// Test with RawMessage with some text.
		//
		// The tests below marked with Issue6458 used to generate "ImZvbyI=" instead "foo".
		// This behavior was intentionally changed in Go 1.8.
		// See https://golang.org/issues/14493#issuecomment-255857318
		{rawText, `"foo"`, true}, // Issue6458
		{&rawText, `"foo"`, true},
		{[]interface{}{rawText}, `["foo"]`, true},  // Issue6458
		{&[]interface{}{rawText}, `["foo"]`, true}, // Issue6458
		{[]interface{}{&rawText}, `["foo"]`, true},
		{&[]interface{}{&rawText}, `["foo"]`, true},
		{struct{ M RawMessage }{rawText}, `{"M":"foo"}`, true}, // Issue6458
		{&struct{ M RawMessage }{rawText}, `{"M":"foo"}`, true},
		{struct{ M *RawMessage }{&rawText}, `{"M":"foo"}`, true},
		{&struct{ M *RawMessage }{&rawText}, `{"M":"foo"}`, true},
		{map[string]interface{}{"M": rawText}, `{"M":"foo"}`, true},  // Issue6458
		{&map[string]interface{}{"M": rawText}, `{"M":"foo"}`, true}, // Issue6458
		{map[string]interface{}{"M": &rawText}, `{"M":"foo"}`, true},
		{&map[string]interface{}{"M": &rawText}, `{"M":"foo"}`, true},
		{T1{rawText}, `{"M":"foo"}`, true}, // Issue6458
		{T2{&rawText}, `{"M":"foo"}`, true},
		{&T1{rawText}, `{"M":"foo"}`, true},
		{&T2{&rawText}, `{"M":"foo"}`, true},
	}

	buf := mocks.NewBuildCloser()
	for i, tt := range tests {
		err := Marshal(tt.in, buf)
		if tt.ok {
			assert.NoError(t, err)
			assert.Equal(t, tt.want, buf.String(), "Test %d", i)
		} else {
			assert.Error(t, err)
		}
		buf.Reset()
	}
}

type marshalPanic struct{}

func (marshalPanic) MarshalJSON() ([]byte, error) { panic(0xdead) }

func TestMarshalPanic(t *testing.T) {
	assert.PanicsWithValue(t, 0xdead, func() { _ = Marshal(&marshalPanic{}, mocks.NewBuildCloser()) })
}

func TestMarshalUncommonFieldNames(t *testing.T) {
	v := struct {
		A0, À, Aβ int
	}{}
	buf := mocks.NewBuildCloser()
	require.NoError(t, Marshal(v, buf))
	assert.Equal(t, `{"A0":0,"À":0,"Aβ":0}`, buf.String())
}

func TestMarshalerError(t *testing.T) {
	s := "test variable"
	st := reflect.TypeOf(s)
	errText := "json: test error"

	tests := []struct {
		err  *MarshalerError
		want string
	}{
		{
			&MarshalerError{st, fmt.Errorf(errText), ""},
			"json: error calling MarshalJSON for type " + st.String() + ": " + errText,
		},
		{
			&MarshalerError{st, fmt.Errorf(errText), "TestMarshalerError"},
			"json: error calling TestMarshalerError for type " + st.String() + ": " + errText,
		},
	}

	for i, tt := range tests {
		got := tt.err.Error()
		if got != tt.want {
			t.Errorf("MarshalerError test %d, got: %s, want: %s", i, got, tt.want)
		}
	}
}
