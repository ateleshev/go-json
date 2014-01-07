// Copyright 2013 Gary Burd. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package json_test

import (
	"bytes"
	"compress/gzip"
	sjson "encoding/json"
	"go/build"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	"github.com/garyburd/json"
)

var codeJSON []byte

func codeInit() {
	p, err := build.Default.Import("encoding/json", "", build.FindOnly)
	if err != nil {
		panic(err)
	}
	f, err := os.Open(filepath.Join(p.Dir, "testdata", "code.json.gz"))
	if err != nil {
		panic(err)
	}
	defer f.Close()
	gz, err := gzip.NewReader(f)
	if err != nil {
		panic(err)
	}
	data, err := ioutil.ReadAll(gz)
	if err != nil {
		panic(err)
	}
	codeJSON = data
}

func BenchmarkScanner(b *testing.B) {
	b.StopTimer()
	if codeJSON == nil {
		codeInit()
	}
	b.StartTimer()
	for i := 0; i < b.N; i++ {
		s := json.NewScanner(bytes.NewReader(codeJSON))
		// Check for errors.
		for s.Scan() {
		}
		if s.Err() != nil {
			b.Fatal(s.Err())
		}

		// Decode.
		var err error
		s = json.NewScanner(bytes.NewReader(codeJSON))
		for s.Scan() {
			_, err = decodeValue(s)
		}

		if s.Err() != nil {
			b.Fatal(s.Err())
		}
		if err != nil {
			b.Fatal(err)
		}
	}
	b.SetBytes(int64(len(codeJSON)))
}

func BenchmarkScannerOnly(b *testing.B) {
	b.StopTimer()
	if codeJSON == nil {
		codeInit()
	}
	b.StartTimer()
	for i := 0; i < b.N; i++ {
		s := json.NewScanner(bytes.NewReader(codeJSON))
		for s.Scan() {
		}
		if s.Err() != nil {
			b.Fatal(s.Err())
		}
	}
	b.SetBytes(int64(len(codeJSON)))
}

func BenchmarkStdUnmarshal(b *testing.B) {
	b.StopTimer()
	if codeJSON == nil {
		codeInit()
	}
	b.StartTimer()
	for i := 0; i < b.N; i++ {
		m := make(map[string]interface{})
		err := sjson.Unmarshal(codeJSON, &m)
		if err != nil {
			b.Fatal(err.Error())
		}
	}
	b.SetBytes(int64(len(codeJSON)))
}

func BenchmarkStdDecode(b *testing.B) {
	b.StopTimer()
	if codeJSON == nil {
		codeInit()
	}
	b.StartTimer()
	for i := 0; i < b.N; i++ {
		m := make(map[string]interface{})
		err := sjson.NewDecoder(bytes.NewReader(codeJSON)).Decode(&m)
		if err != nil {
			b.Fatal(err.Error())
		}
	}
	b.SetBytes(int64(len(codeJSON)))
}
