// Copyright 2014 Gary Burd. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package json

import (
	"fmt"
	"strconv"
)

// A NumberValue represents a JSON number literal.
type NumberValue string

// String returns the literal text of the number.
func (n NumberValue) String() string { return string(n) }

// Float64 returns the number as a float64.
func (n NumberValue) Float64() (float64, error) {
	return strconv.ParseFloat(string(n), 64)
}

// Int64 returns the number as an int64.
func (n NumberValue) Int64() (int64, error) {
	return strconv.ParseInt(string(n), 10, 64)
}

// Uint64 returns the number as an uint64.
func (n NumberValue) Uint64() (uint64, error) {
	return strconv.ParseUint(string(n), 10, 64)
}

// Int returns the number as an int.
func (n NumberValue) Int() (int, error) {
	i, err := strconv.ParseInt(string(n), 10, 0)
	return int(i), err
}

// Uint returns the number as an uint.
func (n NumberValue) Uint() (uint, error) {
	i, err := strconv.ParseUint(string(n), 10, 0)
	return uint(i), err
}

var emptySlice = make([]interface{}, 0, 0)

// DecodeValue decodes the current scanner value to to Go types as follows:
//
//   JSON   Go
//   null   nil
//   object map[string]interface{}
//   array  []interface{}
//   string string
//   bolean bool
//   number NumberValue
func DecodeValue(s *Scanner) (interface{}, error) {
	switch s.Kind() {
	case Number:
		return NumberValue(s.Value()), nil
	case String:
		return string(s.Value()), nil
	case Array:
		v := emptySlice
		n := s.NestingLevel()
		for s.ScanAtLevel(n) {
			subv, err := DecodeValue(s)
			if err != nil {
				return v, err
			}
			v = append(v, subv)
		}
		return v, s.Err()
	case Object:
		v := make(map[string]interface{})
		n := s.NestingLevel()
		for s.ScanAtLevel(n) {
			name := string(s.Name())
			subv, err := DecodeValue(s)
			if err != nil {
				return v, err
			}
			v[name] = subv
		}
		return v, s.Err()
	case Bool:
		return s.Value()[0] == 't', nil
	case Null:
		return nil, nil
	default:
		return nil, fmt.Errorf("unexpected %v", s.Kind())
	}
}
