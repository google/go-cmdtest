// Copyright 2019 The Go Cloud Development Kit Authors
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

package cmdtest

import (
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"sync"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/renameio"
)

var once sync.Once

func setup() {
	// Build echo-stdin, the little program needed to test input redirection.
	if err := exec.Command("go", "build", "testdata/echo-stdin.go").Run(); err != nil {
		log.Fatalf("building echo-stdin: %v", err)
	}
}

// echoStdin contains the same code as the main function of
// testdata/echo-stdin.go, except that it returns the exit code instead of
// calling os.Exit. It is for testing InProcessProgram.
func echoStdin() int {
	fmt.Println("Here is stdin:")
	_, err := io.Copy(os.Stdout, os.Stdin)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed: %v\n", err)
		return 1
	}
	return 0
}

func TestMain(m *testing.M) {
	ret := m.Run()
	// Clean up the echo-stdin binary if we can. (No big deal if we can't.)
	cwd, err := os.Getwd()
	if err == nil {
		name := "echo-stdin"
		if runtime.GOOS == "windows" {
			name += ".exe"
		}
		_ = os.Remove(filepath.Join(cwd, name))
	}
	os.Exit(ret)
}

func TestRead(t *testing.T) {
	got, err := Read("testdata/read")
	if err != nil {
		t.Fatal(err)
	}
	got.Commands = nil
	got.files[0].suite = nil
	want := &TestSuite{
		files: []*testFile{
			{
				filename: filepath.Join("testdata", "read", "read.ct"),
				cases: []*testCase{
					{
						before: []string{
							"# A sample test file.",
							"",
							"#   Prefix stuff.",
							"",
						},
						startLine:  5,
						commands:   []string{"command arg1 arg2", "cmd2"},
						wantOutput: []string{"out1", "out2"},
					},
					{
						before:     []string{"", "# start of the next case"},
						startLine:  11,
						commands:   []string{"c3"},
						wantOutput: nil,
					},
					{
						before:     []string{"", "# start of the third", ""},
						startLine:  15,
						commands:   []string{"c4 --> FAIL"},
						wantOutput: []string{"out3"},
					},
					{
						before:     []string{""},
						startLine:  18,
						commands:   []string{"c5 --> FAIL 2"},
						wantOutput: []string{"out4"},
					},
				},
				suffix: []string{"", "", "# end"},
			},
		},
	}
	if diff := cmp.Diff(want, got, cmp.AllowUnexported(TestSuite{}, testFile{}, testCase{})); diff != "" {
		t.Error(diff)
	}

}

// compareReturningError is similar to compare, but it returns
// errors/differences in an error.
func (ts *TestSuite) compareReturningError(parallel bool) error {
	var ss []string
	for _, tf := range ts.files {
		if s := tf.compare(noopLogger, parallel); s != "" {
			ss = append(ss, s)
		}
	}
	if len(ss) > 0 {
		return errors.New(strings.Join(ss, "\n"))
	}
	return nil
}

func TestCompare(t *testing.T) {
	once.Do(setup)
	ts := mustReadTestSuite(t, "good")
	ts.DisableLogging = true
	ts.Commands["echo-stdin"] = Program("echo-stdin")
	ts.Commands["echoStdin"] = InProcessProgram("echoStdin", echoStdin)
	ts.Run(t, false)

	// Test errors.
	// Since the output of cmp.Diff is unstable, we search for regexps we expect
	// to find there, rather than checking an exact match.
	t.Run("bad", func(t *testing.T) {
		ts = mustReadTestSuite(t, "bad")
		ts.Commands["echo-stdin"] = Program("echo-stdin")
		ts.Commands["code17"] = func([]string, string) ([]byte, error) {
			return nil, fmt.Errorf("wrapping: %w", &ExitCodeErr{Msg: "failed", Code: 17})
		}
		ts.Commands["inprocess99"] = InProcessProgram("inprocess99", func() int { return 99 })

		err := ts.compareReturningError(false)
		if err == nil {
			t.Fatal("got nil, want error")
		}
		got := err.Error()
		wants := []string{
			`testdata.bad.bad-output\.ct:\d: want=-, got=+`,
			`testdata.bad.bad-output\.ct:\d: want=-, got=+`,
			`testdata.bad.bad-fail-1\.ct:\d: "echo" succeeded, but it was expected to fail`,
			`testdata.bad.bad-fail-2\.ct:\d: "cd foo" failed with chdir`,
			`testdata.bad.bad-fail-3\.ct:\d: "cd foo bar" failed with need exactly`,
			`testdata.bad.bad-fail-4\.ct:\d: "cd foo bar" failed without an exit code`,
			`testdata.bad.bad-fail-5\.ct:\d: "cd foo" failed with exit code 2, but 3 was expected`,
			`testdata.bad.bad-fail-6\.ct:\d: "code17" failed with exit code 17, but 4 was expected`,
			`testdata.bad.bad-fail-7\.ct:\d: "inprocess99" failed with exit code 99, but 5 was expected`,
			`testdata.bad.bad-fail-8\.ct:\d: "echo-stdin -exit 1" failed with exit code 1, but 6 was expected`,
		}
		failed := false
		_ = failed
		for _, w := range wants {
			match, err := regexp.MatchString(w, got)
			if err != nil {
				t.Fatal(err)
			}
			if !match {
				t.Errorf(`output does not match "%s"`, w)
				failed = true
			}
		}
		const shouldNotAppear = "should not appear"
		if strings.Contains(got, shouldNotAppear) {
			t.Errorf("saw %q", shouldNotAppear)
			failed = true
		}
		if failed {
			// Log full output to aid debugging.
			t.Logf("output:\n%s", got)
		}
	})
}

func TestExpandVariables(t *testing.T) {
	lookup := func(name string) (string, bool) {
		switch name {
		case "A":
			return "1", true
		case "B_C":
			return "234", true
		default:
			return "", false
		}
	}
	for _, test := range []struct {
		in, want string
	}{
		{"", ""},
		{"${A}", "1"},
		{"${A}${B_C}", "1234"},
		{" x${A}y  ${B_C}z ", " x1y  234z "},
		{" ${A${B_C}", " ${A234"},
	} {
		got, err := expandVariables(test.in, lookup)
		if err != nil {
			t.Errorf("%q: %v", test.in, err)
			continue
		}
		if got != test.want {
			t.Errorf("%q: got %q, want %q", test.in, got, test.want)
		}
	}

	// Unknown variable is an error.
	if _, err := expandVariables("x${C}y", lookup); err == nil {
		t.Error("got nil, want error")
	}
}

func TestUpdateToTemp(t *testing.T) {
	once.Do(setup)
	for _, dir := range []string{"good", "good-without-output"} {
		ts := mustReadTestSuite(t, dir)
		ts.Commands["echo-stdin"] = Program("echo-stdin")
		ts.Commands["echoStdin"] = InProcessProgram("echoStdin", echoStdin)
		f, err := ts.files[0].updateToTemp(false)
		defer f.Cleanup()
		if err != nil {
			t.Fatal(err)
		}
		if diff := diffFiles(t, "testdata/good/good.ct", f.Name()); diff != "" {
			t.Errorf("%s: %s", dir, diff)
		}
	}
}

func TestUpdate(t *testing.T) {
	ct := "testdata/update/update.ct"
	original, err := ioutil.ReadFile(ct)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		// Restore original file content.
		if err := renameio.WriteFile(ct, original, 0644); err != nil {
			t.Fatal(err)
		}
	}()
	ts := mustReadTestSuite(t, "update")
	ts.update(t, false)
	if diff := diffFiles(t, ct, "testdata/update/update.golden"); diff != "" {
		t.Errorf(diff)
	}
}

func TestParseCommand(t *testing.T) {
	for _, test := range []struct {
		cmdline  string
		wantCmd  string
		wantFail bool
		wantCode int
		wantErr  bool
	}{
		{
			cmdline: "ls",
			wantCmd: "ls",
		},
		{
			cmdline:  "a b c --> FAIL   ",
			wantCmd:  "a b c",
			wantFail: true,
		},
		{
			cmdline: "a b c --> fail",
			wantCmd: "a b c --> fail",
		},
		{
			cmdline:  "a b c --> FAIL 23",
			wantCmd:  "a b c",
			wantFail: true,
			wantCode: 23,
		},
		{
			cmdline: "a b c --> FAIL 23a",
			wantErr: true,
		},
		{
			cmdline: "a b c --> FAIL 0",
			wantErr: true,
		},
	} {
		gotCmd, gotFail, gotCode, err := parseCommand(test.cmdline)
		if gotCmd != test.wantCmd || gotFail != test.wantFail || gotCode != test.wantCode || (err != nil) != test.wantErr {
			t.Errorf("%q:\ngot  (%q, %t, %d, %v)\nwant (%q, %t, %d, %t)",
				test.cmdline,
				gotCmd, gotFail, gotCode, err,
				test.wantCmd, test.wantFail, test.wantCode, test.wantErr)
		}
	}
}

func TestParallel(t *testing.T) {
	ts := mustReadTestSuite(t, "parallel")
	ts.RunParallel(t, false)
}

func diffFiles(t *testing.T, gotFile, wantFile string) string {
	got, err := ioutil.ReadFile(gotFile)
	if err != nil {
		t.Fatal(err)
	}
	want, err := ioutil.ReadFile(wantFile)
	if err != nil {
		t.Fatal(err)
	}
	return cmp.Diff(string(want), string(got))
}

func mustReadTestSuite(t *testing.T, dir string) *TestSuite {
	t.Helper()
	ts, err := Read(filepath.Join("testdata", dir))
	if err != nil {
		t.Fatal(err)
	}
	return ts
}
