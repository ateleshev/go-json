// Copyright 2013 Gary Burd. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package json

import (
	"io"
	"strconv"
	"unicode"
	"unicode/utf16"
	"unicode/utf8"
)

// Kind represents the kind a JSON document element.
type Kind int

const (
	// A Null represents a JSON null element.
	Null Kind = iota

	// A Bool represents a JSON bool element.
	Bool

	// A String represents a JSON string element.
	String

	// A Number represents a JSON number element.
	Number

	// An Array represents the start of a JSON array element.
	Array

	// An Object represents the start of a JSON object element.
	Object

	// End represents the end of an object or array element.
	End
)

// Scanner reads a JSON document from an io.Reader. Successive calls to the
// Scan method step through the elements of the document as follows:
//
//  element = Null | Bool | Number | String | object | array
//  array = Array element* End
//  object = Object element* End
//
// Scanning stops unrecoverably at EOF, the first I/O error, or a syntax error.
// When a scan stops, the reader may have advanced arbitrarily far past the
// last token.
//
// When scanning strings, invalid UTF-8 or invalid UTF-16 surrogate pairs are
// not treated as an error. Instead, they are replaced by the Unicode
// replacement character U+FFFD.
type Scanner struct {
	states []stateFunc

	pos int
	buf []byte

	kind      Kind
	eofOK     bool
	err       error
	cook      bool
	isKey     bool
	boolValue bool

	data [2]struct {
		pos, end int
		cook     bool
	}

	rd io.Reader
}

const (
	nameData = iota
	valueData
)

type stateFunc func(*Scanner, byte) stateFunc

// NewScanner allocates and initializes a new scanner.
func NewScanner(rd io.Reader) *Scanner {
	return &Scanner{
		rd:     rd,
		buf:    make([]byte, 0, 1024),
		states: []stateFunc{(*Scanner).stateStart},
	}
}

// Scan advances the Scanner to the next value, which will then be available
// through the Kind, Value, and BoolValue methods. Scan returns false if there
// are no more tokens in the input or an error is encountered. The Err method
// returns the error if any.
func (s *Scanner) Scan() bool {
	s.kind = -1
	s.data[nameData].pos = -1
	s.data[valueData].pos = -1
	state := s.states[len(s.states)-1]

	for {
		for _, b := range s.buf[s.pos:] {
			state = state(s, b)
			s.pos += 1
			if state == nil {
				return s.kind >= 0
			}
		}
		if s.err == nil {
			s.fill()
			if s.pos < len(s.buf) {
				continue
			}
		}
		if s.err != io.EOF {
			return false
		}
		state = state(s, ' ')
		s.pos = len(s.buf)
		if state == nil && s.kind >= 0 {
			return true
		}
		if !s.eofOK {
			s.err = io.ErrUnexpectedEOF
		}
		return false
	}
}

func (s *Scanner) fill() {
	n := 0
	for i := range s.data {
		if pos := s.data[i].pos; pos >= 0 {
			end := s.data[i].end
			if end < 0 {
				end = s.pos
			}
			n += end - pos
		}
	}

	buf := s.buf[:cap(s.buf)]
	const minRead = 512
	if len(buf)-n < minRead {
		buf = make([]byte, 2*len(buf)+minRead)
	}

	n = 0
	for i := range s.data {
		if pos := s.data[i].pos; pos >= 0 {
			end := s.data[i].end
			if end < 0 {
				end = s.pos
			} else {
				s.data[i].end = n + end - pos
			}
			s.data[i].pos = n
			n += copy(buf[n:], s.buf[pos:end])
		}
	}

	var nn int
	nn, s.err = s.rd.Read(buf[n:])
	s.buf = buf[:n+nn]
	s.pos = n
}

func (s *Scanner) stateStart(b byte) stateFunc {
	s.top((*Scanner).stateEnd)
	return s.stateValue(b)
}

func (s *Scanner) stateEnd(b byte) stateFunc {
	switch {
	case isWhiteSpace(b):
		s.eofOK = true
		return (*Scanner).stateEnd
	default:
		return s.syntaxError(b, expectWhitespace)
	}
}

func (s *Scanner) stateValue(b byte) stateFunc {
	switch {
	case isWhiteSpace(b):
		return (*Scanner).stateValue
	case b == '"':
		s.isKey = false
		s.cook = false
		s.data[valueData].pos = s.pos + 1
		s.data[valueData].end = -1
		return (*Scanner).stateString
	case b == '-':
		s.data[valueData].pos = s.pos
		s.data[valueData].end = -1
		return (*Scanner).stateNumberNeg
	case b == '0':
		s.data[valueData].pos = s.pos
		s.data[valueData].end = -1
		return (*Scanner).stateNumberDotOrExp
	case '1' <= b && b <= '9':
		s.data[valueData].pos = s.pos
		s.data[valueData].end = -1
		return (*Scanner).stateNumberDigits
	case b == 't':
		return (*Scanner).stateTr
	case b == 'f':
		return (*Scanner).stateFa
	case b == 'n':
		return (*Scanner).stateNu
	case b == '[':
		s.push((*Scanner).stateArrayElementOrClose)
		s.kind = Array
		return nil
	case b == '{':
		s.push((*Scanner).stateObjectKeyOrClose)
		s.kind = Object
		return nil
	default:
		return s.syntaxError(b, expectValue)
	}
}

func (s *Scanner) stateArrayElementOrClose(b byte) stateFunc {
	switch {
	case isWhiteSpace(b):
		return (*Scanner).stateArrayElementOrClose
	case b == ']':
		s.pop()
		s.kind = End
		return nil
	default:
		s.top((*Scanner).stateArrayCommaOrClose)
		return s.stateValue(b)
	}
}

func (s *Scanner) stateArrayCommaOrClose(b byte) stateFunc {
	switch {
	case isWhiteSpace(b):
		return (*Scanner).stateArrayCommaOrClose
	case b == ',':
		return (*Scanner).stateValue
	case b == ']':
		s.pop()
		s.kind = End
		return nil
	default:
		return s.syntaxError(b, expectArrayCommaOrClose)
	}
}

func (s *Scanner) stateObjectKeyOrClose(b byte) stateFunc {
	switch {
	case isWhiteSpace(b):
		return (*Scanner).stateObjectKeyOrClose
	case b == '}':
		s.pop()
		s.kind = End
		return nil
	case b == '"':
		s.top((*Scanner).stateObjectCommaOrClose)
		s.cook = false
		s.isKey = true
		s.data[nameData].pos = s.pos + 1
		s.data[nameData].end = -1
		return (*Scanner).stateString
	default:
		return s.syntaxError(b, expectObjectKeyOrClose)
	}
}

func (s *Scanner) stateObjectColon(b byte) stateFunc {
	switch {
	case isWhiteSpace(b):
		return (*Scanner).stateObjectColon
	case b == ':':
		return (*Scanner).stateValue
	default:
		return s.syntaxError(b, expectObjectColon)
	}
}

func (s *Scanner) stateObjectCommaOrClose(b byte) stateFunc {
	switch {
	case isWhiteSpace(b):
		return (*Scanner).stateObjectCommaOrClose
	case b == ',':
		return (*Scanner).stateObjectKey
	case b == '}':
		s.pop()
		s.kind = End
		return nil
	default:
		return s.syntaxError(b, expectObjectCommaOrClose)
	}
}

func (s *Scanner) stateObjectKey(b byte) stateFunc {
	switch {
	case isWhiteSpace(b):
		return (*Scanner).stateObjectKey
	case b == '"':
		s.cook = false
		s.isKey = true
		s.data[nameData].pos = s.pos + 1
		s.data[nameData].end = -1
		return (*Scanner).stateString
	default:
		return s.syntaxError(b, expectObjectKey)
	}
}

func (s *Scanner) stateNu(b byte) stateFunc {
	switch {
	case b == 'u':
		return (*Scanner).stateNul
	default:
		return s.syntaxError(b, expectNu)
	}
}

func (s *Scanner) stateNul(b byte) stateFunc {
	switch {
	case b == 'l':
		return (*Scanner).stateNull
	default:
		return s.syntaxError(b, expectNul)
	}
}

func (s *Scanner) stateNull(b byte) stateFunc {
	switch {
	case b == 'l':
		s.kind = Null
		return nil
	default:
		return s.syntaxError(b, expectNull)
	}
}

func (s *Scanner) stateTr(b byte) stateFunc {
	switch {
	case b == 'r':
		return (*Scanner).stateTru
	default:
		return s.syntaxError(b, expectTr)
	}
}

func (s *Scanner) stateTru(b byte) stateFunc {
	switch {
	case b == 'u':
		return (*Scanner).stateTrue
	default:
		return s.syntaxError(b, expectTru)
	}
}

func (s *Scanner) stateTrue(b byte) stateFunc {
	switch {
	case b == 'e':
		s.kind = Bool
		s.boolValue = true
		return nil
	default:
		return s.syntaxError(b, expectTrue)
	}
}

func (s *Scanner) stateFa(b byte) stateFunc {
	switch {
	case b == 'a':
		return (*Scanner).stateFal
	default:
		return s.syntaxError(b, expectFa)
	}
}

func (s *Scanner) stateFal(b byte) stateFunc {
	switch {
	case b == 'l':
		return (*Scanner).stateFals
	default:
		return s.syntaxError(b, expectFal)
	}
}

func (s *Scanner) stateFals(b byte) stateFunc {
	switch {
	case b == 's':
		return (*Scanner).stateFalse
	default:
		return s.syntaxError(b, expectFals)
	}
}

func (s *Scanner) stateFalse(b byte) stateFunc {
	switch {
	case b == 'e':
		s.kind = Bool
		s.boolValue = false
		return nil
	default:
		return s.syntaxError(b, expectFalse)
	}
}

func (s *Scanner) stateString(b byte) stateFunc {
	switch {
	case b == '"':
		if s.isKey {
			s.data[nameData].end = s.pos
			s.data[nameData].cook = s.cook
			return (*Scanner).stateObjectColon
		}
		s.data[valueData].end = s.pos
		s.data[valueData].cook = s.cook
		s.kind = String
		return nil
	case b == '\\':
		s.cook = true
		return (*Scanner).stateStringEscape
	case b < ' ':
		return s.syntaxError(b, expectStringNotControl)
	case b < utf8.RuneSelf:
		return (*Scanner).stateString
	default:
		s.cook = true
		return (*Scanner).stateString
	}
}

func (s *Scanner) stateStringEscape(b byte) stateFunc {
	switch {
	case b == '"' || b == '\\' || b == 'b' || b == 'f' || b == 'n' || b == 'r' || b == 't' || b == '/':
		return (*Scanner).stateString
	case b == 'u':
		return (*Scanner).stateStringUnicodeEscape1
	default:
		return s.syntaxError(b, expectStringEscape)
	}
}

func (s *Scanner) stateStringUnicodeEscape1(b byte) stateFunc {
	switch {
	case isHexDigit(b):
		return (*Scanner).stateStringUnicodeEscape2
	default:
		return s.syntaxError(b, expectStringUnicodeEscape1)
	}
}

func (s *Scanner) stateStringUnicodeEscape2(b byte) stateFunc {
	switch {
	case isHexDigit(b):
		return (*Scanner).stateStringUnicodeEscape3
	default:
		return s.syntaxError(b, expectStringUnicodeEscape2)
	}
}

func (s *Scanner) stateStringUnicodeEscape3(b byte) stateFunc {
	switch {
	case isHexDigit(b):
		return (*Scanner).stateStringUnicodeEscape4
	default:
		return s.syntaxError(b, expectStringUnicodeEscape3)
	}
}

func (s *Scanner) stateStringUnicodeEscape4(b byte) stateFunc {
	switch {
	case isHexDigit(b):
		return (*Scanner).stateString
	default:
		return s.syntaxError(b, expectStringUnicodeEscape4)
	}
}

func (s *Scanner) stateNumberNeg(b byte) stateFunc {
	switch {
	case b == '0':
		return (*Scanner).stateNumberDotOrExp
	case '1' <= b && b <= '9':
		return (*Scanner).stateNumberDigits
	default:
		return s.syntaxError(b, expectNumberNeg)
	}
}

func (s *Scanner) stateNumberDigits(b byte) stateFunc {
	switch {
	case isDecimalDigit(b):
		return (*Scanner).stateNumberDigits
	default:
		return s.stateNumberDotOrExp(b)
	}
}

func (s *Scanner) stateNumberDotOrExp(b byte) stateFunc {
	switch {
	case b == '.':
		return (*Scanner).stateNumberFrac
	case b == 'e' || b == 'E':
		return (*Scanner).stateNumberExp
	default:
		return s.finishNumber()
	}
}

func (s *Scanner) stateNumberFrac(b byte) stateFunc {
	switch {
	case isDecimalDigit(b):
		return (*Scanner).stateNumberFracDigits
	default:
		return s.syntaxError(b, expectNumberFrac)
	}
}

func (s *Scanner) stateNumberFracDigits(b byte) stateFunc {
	switch {
	case isDecimalDigit(b):
		return (*Scanner).stateNumberFracDigits
	case b == 'e' || b == 'E':
		return (*Scanner).stateNumberExp
	default:
		return s.finishNumber()
	}
}

func (s *Scanner) stateNumberExp(b byte) stateFunc {
	switch {
	case b == '+' || b == '-':
		return (*Scanner).stateNumberExpDigit
	case isDecimalDigit(b):
		return (*Scanner).stateNumberExpDigits
	default:
		return s.syntaxError(b, expectNumberExp)
	}
}

func (s *Scanner) stateNumberExpDigit(b byte) stateFunc {
	switch {
	case isDecimalDigit(b):
		return (*Scanner).stateNumberExpDigits
	default:
		return s.syntaxError(b, expectNumberExpDigit)
	}
}

func (s *Scanner) stateNumberExpDigits(b byte) stateFunc {
	switch {
	case isDecimalDigit(b):
		return (*Scanner).stateNumberExpDigits
	default:
		return s.finishNumber()
	}
}

func (s *Scanner) finishNumber() stateFunc {
	s.kind = Number
	s.data[valueData].end = s.pos
	s.pos -= 1
	return nil
}

func (s *Scanner) top(f stateFunc) {
	s.states[len(s.states)-1] = f
}

func (s *Scanner) push(f stateFunc) {
	s.states = append(s.states, f)
}

func (s *Scanner) pop() {
	s.states = s.states[:len(s.states)-1]
}

// Kind returns the kind of the current value.
func (s *Scanner) Kind() Kind {
	return s.kind
}

// Err returns the first non-EOF error that was encountered by the Scanner.
func (s *Scanner) Err() error {
	err := s.err
	if err == io.EOF {
		return nil
	}
	return err
}

// Name returns the object member name of the current value.  The underlying
// array may point to data that will be overwritten by a subsequent call to
// Scan.
func (s *Scanner) Name() []byte {
	return s.cookedData(nameData)
}

// Value returns the bytes of the current string or number value. The
// underlying array may point to data that will be overwritten by a
// subsequent call to Scan.
func (s *Scanner) Value() []byte {
	return s.cookedData(valueData)
}

func (s *Scanner) cookedData(dataIndex int) []byte {
	data := &s.data[dataIndex]
	if data.pos < 0 {
		return nil
	}
	rbuf := s.buf[data.pos:data.end]
	if !data.cook {
		return rbuf
	}

	r := 0
	w := 0
	wbuf := rbuf
	for r < len(rbuf) {
		switch b := rbuf[r]; {
		case b == '\\':
			r++
			b = rbuf[r]
			if b != 'u' {
				switch b {
				case 'b':
					b = '\b'
				case 'f':
					b = '\f'
				case 'n':
					b = '\n'
				case 'r':
					b = '\r'
				case 't':
					b = '\t'
				}
				wbuf[w] = b
				r++
				w++
			} else {
				c := parseHex(rbuf[r+1 : r+5])
				r += 5
				if utf16.IsSurrogate(c) {
					if r+6 <= len(rbuf) && rbuf[r] == '\\' && rbuf[r+1] == 'u' {
						c = utf16.DecodeRune(c, parseHex(rbuf[r+2:r+6]))
						if c != unicode.ReplacementChar {
							r += 6
						}
					} else {
						c = unicode.ReplacementChar
					}
				}
				w += utf8.EncodeRune(wbuf[w:], c)
			}
		case b < utf8.RuneSelf:
			wbuf[w] = b
			r++
			w++
		default:
			c, n := utf8.DecodeRune(rbuf[r:])
			r += n
			if c == utf8.RuneError && n < 3 &&
				((&wbuf[0] == &rbuf[0] && w+3 >= r) ||
					(w+3 >= len(wbuf))) {
				buf := make([]byte, w+3+(4*(len(rbuf)-r))/3)
				copy(buf, wbuf[:w])
				wbuf = buf
			}
			w += utf8.EncodeRune(wbuf[w:], c)
		}
	}
	return wbuf[:w]
}

// BoolValue returns the  value of the current boolean value.
func (s *Scanner) BoolValue() bool {
	return s.boolValue
}

// Skip scans over the structure of the current value. If the current value is
// an array or object, then Skip scans until the corresponding End token is
// found.  Skip does nothing for the other tokens. Skip recurs if it encounters
// an Array or Object token, so it can be used to skip nested structures.
func (s *Scanner) Skip() {
	n := len(s.states)
	switch s.kind {
	case Object:
		n -= 1
	case Array:
		n -= 1
	}
	for len(s.states) > n && s.Scan() {
	}
}

// ScanToEnd is like Scan, except it returns false if the token is an End
// token.
func (s *Scanner) ScanToEnd() bool {
	if !s.Scan() {
		return false
	}
	if s.Kind() == End {
		return false
	}
	return true
}

func (s *Scanner) syntaxError(b byte, expect string) stateFunc {
	s.err = &SyntaxError{s.pos, b, expect}
	return nil
}

type SyntaxError struct {
	Pos    int
	b      byte
	expect string
}

func (e *SyntaxError) Error() string {
	return "expected " + e.expect + ", found " + strconv.QuoteRune(rune(e.b))
}

const (
	expectWhitespace           = "whitespace"
	expectValue                = "start of JSON value"
	expectArrayCommaOrClose    = "',' or ']' in array"
	expectObjectKeyOrClose     = "key or '}' in object"
	expectObjectColon          = "':' in object member"
	expectObjectCommaOrClose   = "',' or '}' in object"
	expectObjectKey            = "string key in object"
	expectNu                   = "'u' in null"
	expectNul                  = "'l' in null"
	expectNull                 = "'l' in null"
	expectTr                   = "'r' in true"
	expectTru                  = "'u' in true"
	expectTrue                 = "'e' in true"
	expectFa                   = "'a' in false"
	expectFal                  = "'l' in false"
	expectFals                 = "'s' in false"
	expectFalse                = "'e' in false"
	expectStringNotControl     = "non-control byte"
	expectStringEscape         = "string escape"
	expectStringUnicodeEscape1 = "hex digit following \\u"
	expectStringUnicodeEscape2 = "hex digit following \\u"
	expectStringUnicodeEscape3 = "hex digit following \\u"
	expectStringUnicodeEscape4 = "hex digit following \\u"
	expectNumberNeg            = "digit after '-'"
	expectNumberFrac           = "digit after '.'"
	expectNumberExp            = "exponent"
	expectNumberExpDigit       = "exponent digits"
)

func isWhiteSpace(b byte) bool {
	return b == ' ' || b == '\n' || b == '\r' || b == '\t'
}

func isHexDigit(b byte) bool {
	return ('0' <= b && b <= '9') ||
		('a' <= b && b <= 'f') ||
		('A' <= b && b <= 'F')
}

func isDecimalDigit(b byte) bool {
	return '0' <= b && b <= '9'
}

func parseHex(p []byte) rune {
	var r rune
	for _, b := range p {
		switch {
		case '0' <= b && b <= '9':
			r = r<<4 + rune(b-'0')
		case 'a' <= b && b <= 'f':
			r = r<<4 + rune(b-'a'+10)
		case 'A' <= b && b <= 'F':
			r = r<<4 + rune(b-'A'+10)
		}
	}
	return r
}
