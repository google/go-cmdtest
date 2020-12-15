// Copyright 2020 The Go Cloud Development Kit Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     https://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Either windows or windows_test
// +build windows windows_test

// Test this code on any machine with
//   go test -tags windows_test

package cmdtest

import (
	"io/ioutil"
	"os"
	"path/filepath"
)

func createTempFile(filename string) (tempFile, error) {
	f, err := ioutil.TempFile("", filepath.Base(filename))
	if err != nil {
		return nil, err
	}
	return &simpleTempFile{File: f, path: filename}, nil
}

type simpleTempFile struct {
	*os.File
	path   string // rename to this
	closed bool   // Close was called
	done   bool   // Close and Rename succeeded
}

// Code is taken from github.com/google/renameio@v0.1.0/tempfile.go.
// Although rename can't properly be done atomically on Windows,
// this is the best we have, and it's better than nothing.

func (t *simpleTempFile) Cleanup() error {
	if t.done {
		return nil
	}
	var closeErr error
	if !t.closed {
		closeErr = t.Close()

	}
	if err := os.Remove(t.Name()); err != nil {
		return err
	}
	return closeErr
}

func (t *simpleTempFile) CloseAtomicallyReplace() error {
	if err := t.Sync(); err != nil {
		return err
	}
	t.closed = true
	if err := t.Close(); err != nil {
		return err
	}
	if err := os.Rename(t.Name(), t.path); err != nil {
		return err
	}
	t.done = true
	return nil
}
