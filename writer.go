// Copyright 2014 Gary Burd. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package json

import (
	"bufio"
	"errors"
	"io"
	"math"
	"strconv"
)

type stringWriter interface {
	Write(p []byte) (nn int, err error)
	WriteByte(c byte) error
	WriteString(s string) (int, error)
}

type Writer struct {
	bw      *bufio.Writer
	sw      stringWriter
	scratch [64]byte
	comma   bool
	depth   int
	err     error
}

func NewWriter(w io.Writer) *Writer {
	writer := &Writer{}
	if sw, ok := w.(stringWriter); ok {
		writer.sw = sw
	} else {
		writer.bw = bufio.NewWriter(w)
		writer.sw = writer.bw
	}
	return writer
}

func (w *Writer) Err() error {
	return w.err
}

func (w *Writer) end(err error) error {
	if w.depth != 0 {
		w.comma = true
		return err
	}

	w.comma = false
	if w.bw != nil {
		if e := w.bw.Flush(); e != nil && err == nil {
			err = e
		}
	}
	return err
}

func (w *Writer) StartArray() error {
	if w.comma {
		w.sw.WriteByte(',')
	}
	w.comma = false
	w.depth += 1
	return w.sw.WriteByte('[')
}

func (w *Writer) EndArray() error {
	w.depth -= 1
	return w.end(w.sw.WriteByte(']'))
}

func (w *Writer) StartObject() error {
	if w.comma {
		w.sw.WriteByte(',')
	}
	w.comma = false
	w.depth += 1
	return w.sw.WriteByte('{')
}

func (w *Writer) EndObject() error {
	w.depth -= 1
	return w.end(w.sw.WriteByte('}'))
}

func (w *Writer) Name(name string) error {
	if w.comma {
		w.sw.WriteByte(',')
	}
	w.comma = false
	writeString(w.sw, name)
	return w.sw.WriteByte(':')
}

func (w *Writer) write(p []byte) error {
	if w.comma {
		w.sw.WriteByte(',')
	}
	_, err := w.sw.Write(p)
	return w.end(err)
}

func (w *Writer) writeQuoted(p []byte) error {
	if w.comma {
		w.sw.WriteByte(',')
	}
	w.sw.WriteByte('"')
	w.sw.Write(p)
	return w.end(w.sw.WriteByte('"'))
}

func (w *Writer) Uint(u uint64) error {
	return w.write(strconv.AppendUint(w.scratch[:0], u, 10))
}

func (w *Writer) Int(i int64) error {
	return w.write(strconv.AppendInt(w.scratch[:0], i, 10))
}

func (w *Writer) QuotedUint(u uint64) error {
	return w.writeQuoted(strconv.AppendUint(w.scratch[:0], u, 10))
}

func (w *Writer) QuotedInt(i int64) error {
	return w.writeQuoted(strconv.AppendInt(w.scratch[:0], i, 10))
}

func (w *Writer) Float(f float64) error {
	if math.IsInf(f, 0) || math.IsNaN(f) {
		w.write([]byte("0"))
		return errors.New("unsupported value (inf, nan)")
	}
	return w.write(strconv.AppendFloat(w.scratch[:0], f, 'g', -1, 64))
}

func (w *Writer) Bool(b bool) error {
	if w.comma {
		w.sw.WriteByte(',')
	}
	_, err := w.sw.WriteString(strconv.FormatBool(b))
	return w.end(err)
}

func (w *Writer) String(s string) error {
	if w.comma {
		w.sw.WriteByte(',')
	}
	return w.end(writeString(w.sw, s))
}

func (w *Writer) StringBytes(p []byte) error {
	if w.comma {
		w.sw.WriteByte(',')
	}
	return w.end(writeStringBytes(w.sw, p))
}
