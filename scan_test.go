// Copyright 2013 Gary Burd. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package json

import (
	"io"
	"reflect"
	"strconv"
	"strings"
	"testing"
)

type scan struct {
	k Kind   // s.Kind(), -1 for s.Scan() == false
	n string // s.Name()
	v string // s.Value()
	e string // s.Err().Error()
}

var eof = scan{k: -1}

func (s scan) String() string {
	var buf []byte
	switch s.k {
	case -1:
		if s == eof {
			return "{eof}"
		}
		buf = []byte("{err, e=")
		buf = append(buf, strconv.Quote(s.e)...)
		buf = append(buf, '}')
		return string(buf)
	case Null:
		buf = []byte("{null")
	case Bool:
		buf = []byte("{bool")
	case String:
		buf = []byte("{string")
	case Number:
		buf = []byte("{number")
	case Array:
		buf = []byte("{array")
	case Object:
		buf = []byte("{object")
	case End:
		buf = []byte("{end")
	default:
		buf = []byte("{unknown")
	}

	if len(s.n) > 0 {
		buf = append(buf, ", n="...)
		buf = append(buf, strconv.Quote(s.n)...)
	}

	if len(s.v) > 0 {
		buf = append(buf, ", v="...)
		buf = append(buf, strconv.Quote(s.v)...)
	}

	buf = append(buf, "}"...)
	return string(buf)
}

func syntaxError(b byte, expect string) scan {
	return scanError(&SyntaxError{b: b, expect: expect})
}

func scanError(e error) scan {
	return scan{k: -1, e: e.Error()}
}

var scannerTests = []struct {
	s     string
	scans []scan
}{
	{`null`, []scan{{k: Null, v: "null"}, eof}},
	{` null `, []scan{{k: Null, v: "null"}, eof}},
	{`false`, []scan{{k: Bool, v: "false"}, eof}},
	{` false `, []scan{{k: Bool, v: "false"}, eof}},
	{`true`, []scan{{k: Bool, v: "true"}, eof}},
	{` true `, []scan{{k: Bool, v: "true"}, eof}},

	{`nx`, []scan{syntaxError('x', expectNu)}},
	{`nux`, []scan{syntaxError('x', expectNul)}},
	{`nulx`, []scan{syntaxError('x', expectNull)}},
	{`fx`, []scan{syntaxError('x', expectFa)}},
	{`fax`, []scan{syntaxError('x', expectFal)}},
	{`falx`, []scan{syntaxError('x', expectFals)}},
	{`falsx`, []scan{syntaxError('x', expectFalse)}},
	{`tx`, []scan{syntaxError('x', expectTr)}},
	{`trx`, []scan{syntaxError('x', expectTru)}},
	{`trux`, []scan{syntaxError('x', expectTrue)}},

	{`[]`, []scan{{k: Array}, {k: End}, eof}},
	{`[[]]`, []scan{{k: Array}, {k: Array}, {k: End}, {k: End}, eof}},
	{`[true, false, null]`, []scan{{k: Array}, {k: Bool, v: "true"}, {k: Bool, v: "false"}, {k: Null, v: "null"}, {k: End}, eof}},
	{`["a", "b"]`, []scan{{k: Array}, {k: String, v: "a"}, {k: String, v: "b"}, {k: End}, eof}},

	{`[x`, []scan{{k: Array}, syntaxError('x', expectValue)}},
	{`["hello"x`, []scan{{k: Array}, {k: String, v: "hello"}, syntaxError('x', expectArrayCommaOrClose)}},
	{`["hello",x`, []scan{{k: Array}, {k: String, v: "hello"}, syntaxError('x', expectValue)}},

	{`]`, []scan{syntaxError(']', expectValue)}},

	{`""`, []scan{{k: String, v: ""}, eof}},
	{` "" `, []scan{{k: String, v: ""}, eof}},
	{`"hello"`, []scan{{k: String, v: "hello"}, eof}},
	{`"\"\\\/\b\f\n\r\t"`,
		[]scan{{k: String, v: "\"\\/\b\f\n\r\t"}, eof}},

	{`"\u004d\u0430\u4e8c\ud800\udf02"`,
		[]scan{{k: String, v: "Mа二𐌂"}, eof}},

	{`"\ud800"`, []scan{{k: String, v: "\uFFFD"}, eof}},
	{`"\u0000"`, []scan{{k: String, v: "\x00"}, eof}},
	{`"\ux"`, []scan{syntaxError('x', expectStringUnicodeEscape1)}},
	{`"\u0x"`, []scan{syntaxError('x', expectStringUnicodeEscape2)}},
	{`"\uabx"`, []scan{syntaxError('x', expectStringUnicodeEscape3)}},
	{`"\uABCx"`, []scan{syntaxError('x', expectStringUnicodeEscape4)}},

	{`"\u0414\u0430 \u041c\u0443 \u041C\u0443 \u0415\u0431\u0430 \u041c\u0430\u0439\u043a\u0430\u0442\u0430"`,
		[]scan{{k: String, v: "Да Му Му Еба Майката"}, eof}},
	{`"\u0066\u006f\u006f\u0062\u0061\u0072"`, []scan{{k: String, v: "foobar"}, eof}},

	{`"U+10ABCD: 􊯍" `, []scan{{k: String, v: "U+10ABCD: \U0010abcd"}, eof}},

	{`"matzue: 松江, asakusa: 浅草"`,
		[]scan{{k: String, v: "matzue: 松江, asakusa: 浅草"}, eof}},

	{`"Да Му Еба Майката"`,
		[]scan{{k: String, v: "Да Му Еба Майката"}, eof}},

	{`"\n foo \/ bar \r\f\\\uffff\t\b\"\\ and you can't escape thi\s"`,
		[]scan{syntaxError('s', expectStringEscape)}},

	{`"\u0123 \u4567 \u89ab \uc/ef \uABCD \uEFFE "`,
		[]scan{syntaxError('/', expectStringUnicodeEscape2)}},

	{`"replace: \uD834x\uDD1E"`, []scan{{k: String, v: "replace: \uFFFDx\uFFFD"}, eof}},
	{"\"replace in place: \\u0066\\u006f\\u006f\xd1\\u0062\\u0061\\u0072\"",
		[]scan{{k: String, v: "replace in place: foo\uFFFDbar"}, eof}},
	{"\"replace grow: foo \xd1 bar\"", []scan{{k: String, v: "replace grow: foo \uFFFD bar"}, eof}},
	{"\"replace more grow: \xd1\xd1\xd1\xd1\xd1\xd1\xd1\xd1\xd1\xd1\"",
		[]scan{{k: String, v: "replace more grow: \uFFFD\uFFFD\uFFFD\uFFFD\uFFFD\uFFFD\uFFFD\uFFFD\uFFFD\uFFFD"}, eof}},

	{`"This is ok \n, but not this
	    "`,
		[]scan{syntaxError('\n', expectStringNotControl)}},

	{`2009`, []scan{{k: Number, v: "2009"}, eof}},
	{` 2009 `, []scan{{k: Number, v: "2009"}, eof}},
	{`0`, []scan{{k: Number, v: "0"}, eof}},
	{`-0`, []scan{{k: Number, v: "-0"}, eof}},
	{`0 `, []scan{{k: Number, v: "0"}, eof}},
	{`1.0`, []scan{{k: Number, v: "1.0"}, eof}},
	{`1.0 `, []scan{{k: Number, v: "1.0"}, eof}},
	{`1.0e10`, []scan{{k: Number, v: "1.0e10"}, eof}},
	{`1.0e10 `, []scan{{k: Number, v: "1.0e10"}, eof}},
	{`1.0e-10`, []scan{{k: Number, v: "1.0e-10"}, eof}},
	{`1.0e-10 `, []scan{{k: Number, v: "1.0e-10"}, eof}},
	{`1.0e+10`, []scan{{k: Number, v: "1.0e+10"}, eof}},
	{`1.0e+10 `, []scan{{k: Number, v: "1.0e+10"}, eof}},

	{`- `, []scan{syntaxError(' ', expectNumberNeg)}},
	{`10.e2 `, []scan{syntaxError('e', expectNumberFrac)}},
	{`10. `, []scan{syntaxError(' ', expectNumberFrac)}},
	{`10e `, []scan{syntaxError(' ', expectNumberExp)}},
	{`10e+ `, []scan{syntaxError(' ', expectNumberExpDigit)}},
	{`10e- `, []scan{syntaxError(' ', expectNumberExpDigit)}},

	{`{}`, []scan{{k: Object}, {k: End}, eof}},
	{`{ "x": {}}`, []scan{{k: Object}, {n: "x", k: Object}, {k: End}, {k: End}, eof}},
	{`{x`, []scan{{k: Object}, syntaxError('x', expectObjectKeyOrClose)}},

	{`{"hello"x`, []scan{{k: Object}, syntaxError('x', expectObjectColon)}},
	{`{"hello":x`, []scan{{k: Object}, syntaxError('x', expectValue)}},
	{`{"hello":"world"x`, []scan{{k: Object}, {k: String, n: "hello", v: "world"}, syntaxError('x', expectObjectCommaOrClose)}},
	{`{"hello":"world",x`, []scan{{k: Object}, {k: String, n: "hello", v: "world"}, syntaxError('x', expectObjectKey)}},

	{`}`, []scan{syntaxError('}', expectValue)}},

	{`[ 0.1e2, 1e1, 3.141569, 10000000000000e-10]`,
		[]scan{
			{k: Array},
			{k: Number, v: "0.1e2"},
			{k: Number, v: "1e1"},
			{k: Number, v: "3.141569"},
			{k: Number, v: "10000000000000e-10"},
			{k: End}, eof}},

	{`[0.00011999999999999999, 6E-06, 6E-06, 1E-06, 1E-06]`,
		[]scan{
			{k: Array},
			{k: Number, v: "0.00011999999999999999"},
			{k: Number, v: "6E-06"},
			{k: Number, v: "6E-06"},
			{k: Number, v: "1E-06"},
			{k: Number, v: "1E-06"},
			{k: End}, eof}},

	{`[01]`, []scan{{k: Array}, {k: Number, v: "0"}, syntaxError('1', expectArrayCommaOrClose)}},

	{`[{"foo":123}]`, []scan{{k: Array}, {k: Object}, {k: Number, n: "foo", v: "123"}, {k: End}, {k: End}, eof}},

	{`[ 9223372036854775807, -9223372036854775807 ]`,
		[]scan{
			{k: Array},
			{k: Number, v: "9223372036854775807"},
			{k: Number, v: "-9223372036854775807"},
			{k: End},
			eof}},

	{`[ 1,2,3,4,5,6,7, 123456789 , -123456789, 2147483647, -2147483647 ]`,
		[]scan{
			{k: Array},
			{k: Number, v: "1"},
			{k: Number, v: "2"},
			{k: Number, v: "3"},
			{k: Number, v: "4"},
			{k: Number, v: "5"},
			{k: Number, v: "6"},
			{k: Number, v: "7"},
			{k: Number, v: "123456789"},
			{k: Number, v: "-123456789"},
			{k: Number, v: "2147483647"},
			{k: Number, v: "-2147483647"},
			{k: End},
			eof}},

	{`{ "boolean, true": true, "boolean, false": false, "null": null }`,
		[]scan{
			{k: Object},
			{k: Bool, n: "boolean, true", v: "true"},
			{k: Bool, n: "boolean, false", v: "false"},
			{k: Null, n: "null", v: "null"},
			{k: End},
			eof}},

	{`{ "this": "is", "really": "simple", "json": "right?" }`,
		[]scan{
			{k: Object},
			{k: String, n: "this", v: "is"},
			{k: String, n: "really", v: "simple"},
			{k: String, n: "json", v: "right?"},
			{k: End},
			eof}},

	{``, []scan{eof}},
	{`"`, []scan{scanError(io.ErrUnexpectedEOF)}},

	{`2009-10`, []scan{{k: Number, v: "2009"}, {k: Number, v: "-10"}, eof}},

	{`10.`, []scan{scanError(io.ErrUnexpectedEOF)}},
	{`10e`, []scan{scanError(io.ErrUnexpectedEOF)}},
	{`10e+`, []scan{scanError(io.ErrUnexpectedEOF)}},
	{`10e-`, []scan{scanError(io.ErrUnexpectedEOF)}},

	{`{`, []scan{{k: Object}, scanError(io.ErrUnexpectedEOF)}},
	{`{"hello"`, []scan{{k: Object}, scanError(io.ErrUnexpectedEOF)}},
	{`{"hello":`, []scan{{k: Object}, scanError(io.ErrUnexpectedEOF)}},
	{`{"hello":"world"`, []scan{{k: Object}, {k: String, n: "hello", v: "world"}, scanError(io.ErrUnexpectedEOF)}},
	{`{"hello":"world",`, []scan{{k: Object}, {k: String, n: "hello", v: "world"}, scanError(io.ErrUnexpectedEOF)}},

	{`["hello"`, []scan{{k: Array}, {k: String, v: "hello"}, scanError(io.ErrUnexpectedEOF)}},
	{`["hello",`, []scan{{k: Array}, {k: String, v: "hello"}, scanError(io.ErrUnexpectedEOF)}},

	{`{} {}`, []scan{{k: Object}, {k: End}, {k: Object}, {k: End}, eof}},

	{`[ "foo", "bar"`, []scan{{k: Array}, {k: String, v: "foo"}, {k: String, v: "bar"}, scanError(io.ErrUnexpectedEOF)}},
}

func TestScanner(t *testing.T) {
tests:
	for _, tt := range scannerTests {
		s := NewScanner(strings.NewReader(tt.s))
		s.AllowMultple()
		for i, want := range tt.scans {
			var got scan
			if !s.Scan() {
				got.k = -1
				if err := s.Err(); err != nil {
					got.e = err.Error()
				}
			} else {
				got.k = s.Kind()
				got.n = string(s.Name())
				got.v = string(s.Value())
			}
			if !reflect.DeepEqual(got, want) {
				t.Errorf("%q:%d, got=%s, want=%s", tt.s, i, got, want)
				continue tests
			}
		}
	}
}

func TestScanAtLevel(t *testing.T) {
	s := NewScanner(strings.NewReader(`[[2], 1]`))
	if !s.Scan() {
		t.Errorf("expected s.Scan() = true")
	}
	if s.Kind() != Array {
		t.Errorf("expected [")
	}
	n := s.NestingLevel()
	if !s.Scan() {
		t.Errorf("expected s.Scan() = true")
	}
	if s.Kind() != Array {
		t.Errorf("expected [")
	}
	if !s.ScanAtLevel(n) {
		t.Errorf("expected ss.Scan() = true")
	}
	if s.Kind() != Number || string(s.Value()) != "1" {
		t.Errorf("expected 1")
	}
	if s.ScanAtLevel(n) {
		t.Errorf("expected ss.Scan() = false")
	}
}
