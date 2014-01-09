// Copyright 2013 Gary Burd. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package json_test

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/garyburd/json"
)

const jsonText = `
[
    {
        "name": "redigo",
        "keywords": ["database", "redis"],
        "imports": 10
    },
    {
        "name": "mgo",
        "keywords": ["database", "mongodb"],
        "imports": 22
    }
]
`

func decodeValue(s *json.Scanner) (interface{}, error) {
	switch s.Kind() {
	case json.Number:
		return strconv.ParseFloat(string(s.Value()), 64)
	case json.String:
		return string(s.Value()), nil
	case json.Array:
		v := []interface{}{}
		n := s.NestingLevel()
		for s.ScanAtLevel(n) {
			subv, err := decodeValue(s)
			if err != nil {
				return v, err
			}
			v = append(v, subv)
		}
		return v, s.Err()
	case json.Object:
		v := make(map[string]interface{})
		n := s.NestingLevel()
		for s.ScanAtLevel(n) {
			name := string(s.Name())
			subv, err := decodeValue(s)
			if err != nil {
				return v, err
			}
			v[name] = subv
		}
		return v, s.Err()
	case json.Bool:
		return s.Value()[0] == 't', nil
	case json.Null:
		return nil, nil
	default:
		return nil, fmt.Errorf("unexpected %v", s.Kind())
	}
}

// This example shows how to decode a JSON value to a tree of maps and slices.
func ExampleScanner() {

	s := json.NewScanner(strings.NewReader(jsonText))

	if !s.Scan() {
		fmt.Printf("error %v\n", s.Err())
		return
	}

	v, err := decodeValue(s)
	if err != nil {
		fmt.Printf("error %v\n", err)
		return
	}

	s.Scan()
	if s.Err() != nil {
		fmt.Printf("error %v\n", s.Err())
		return
	}

	fmt.Println(v)

	// Output:
	// [map[name:redigo keywords:[database redis] imports:10] map[name:mgo keywords:[database mongodb] imports:22]]
}
