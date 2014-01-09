// Copyright 2013 Gary Burd. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package json

import (
	"bytes"
	"io"
	"testing"
)

var writerTests = []struct {
	fn func(w *Writer)
	s  string
}{
	{func(w *Writer) { w.Int(-1) }, "-1"},
	{func(w *Writer) { w.Uint(1) }, "1"},
	{func(w *Writer) { w.QuotedInt(-1) }, `"-1"`},
	{func(w *Writer) { w.QuotedUint(1) }, `"1"`},
	{func(w *Writer) { w.Float(1.23) }, "1.23"},
	{func(w *Writer) { w.Bool(true) }, "true"},
	{func(w *Writer) { w.String("hello") }, `"hello"`},
	{func(w *Writer) { w.StringBytes([]byte("hello")) }, `"hello"`},
	{func(w *Writer) { w.StartObject(); w.Name("hello"); w.String("world"); w.EndObject() }, `{"hello":"world"}`},
	{func(w *Writer) {
		w.StartObject()
		w.Name("a")
		w.String("b")
		w.Name("c")
		w.String("d")
		w.EndObject()
	}, `{"a":"b","c":"d"}`},
	{func(w *Writer) { w.StartArray(); w.String("hello"); w.EndArray() }, `["hello"]`},
	{func(w *Writer) { w.StartArray(); w.String("a"); w.String("b"); w.EndArray() }, `["a","b"]`},
}

func TestWrite(t *testing.T) {
	for _, tt := range writerTests {
		var buf bytes.Buffer
		w := NewWriter(&buf)
		tt.fn(w)
		s := buf.String()
		if tt.s != s {
			t.Errorf("want %s, got %s", tt.s, s)
		}
	}
}

type writerOnly struct {
	io.Writer
}

func TestWriteWriterOnly(t *testing.T) {
	for _, tt := range writerTests {
		var buf bytes.Buffer
		w := NewWriter(writerOnly{&buf})
		tt.fn(w)
		s := buf.String()
		if tt.s != s {
			t.Errorf("want %s, got %s", tt.s, s)
		}
	}
}

func TestWriteArray(t *testing.T) {
	for _, tt := range writerTests {
		var buf bytes.Buffer
		w := NewWriter(&buf)
		w.StartArray()
		tt.fn(w)
		tt.fn(w)
		w.EndArray()
		got := buf.String()
		want := "[" + tt.s + "," + tt.s + "]"
		if want != got {
			t.Errorf("want %s, got %s", want, got)
		}
	}
}

func TestWriteObject(t *testing.T) {
	for _, tt := range writerTests {
		var buf bytes.Buffer
		w := NewWriter(&buf)
		w.StartObject()
		w.Name("a")
		tt.fn(w)
		w.Name("b")
		tt.fn(w)
		w.EndObject()
		got := buf.String()
		want := `{"a":` + tt.s + `,"b":` + tt.s + "}"
		if want != got {
			t.Errorf("want %s, got %s", want, got)
		}
	}
}
