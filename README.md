[![Build Status](https://travis-ci.org/google/go-cmdtest.svg?branch=master)](https://travis-ci.org/google/go-cmdtest)
[![godoc](https://godoc.org/github.com/google/go-cmdtest?status.svg)](https://godoc.org/github.com/google/go-cmdtest)
[![Go Report Card](https://goreportcard.com/badge/github.com/google/go-cmdtest)](https://goreportcard.com/report/github.com/google/go-cmdtest)

# Testing your CLI

The cmdtest package simplifies testing of command-line interfaces. It provides a
simple, cross-platform, shell-like language to express command execution. It can
compare actual output with the expected output, and can also update a file with
new "golden" output that is deemed correct.

## Test files

Start using cmdtest by writing a test file with the extension `.ct`. The test
file will consist of commands prefixed by `$` and expected output following each
command. Lines starting with `#` are comments. Example:

```
# Testing for my-cli.

# The "help" command.
$ my-cli help
my-cli is a CLI, and this is its help.

# Verify that an invalid command fails and prints a useful error.
$ my-cli invalidcmd --> FAIL
Error: unknown command "invalidcmd".
```

You can leave the expected output out and let `cmdtest` fill it in for you using
`update` mode (see below).

More details on test file format:

*   Before the first line starting with a `$`, empty lines and lines beginning
    with "#" are ignored.
*   A sequence of consecutive lines starting with `$` begin a test case. These
    lines are commands to execute. See below for the valid commands.
*   Lines following the `$` lines are command output (merged stdout and stderr).
    Output is always treated literally.
*   After the command output there should be a blank line. Between that blank
    line and the next `$` line, empty lines and lines beginning with `#` are
    ignored. (Because of these rules, cmdtest cannot distinguish trailing blank
    lines in the output.)
*   Syntax of a line beginning with `$`:
    *   A sequence of space-separated words (no quoting is supported). The first
        word is the command, the rest are its args. If the next-to-last word is
        `<`, the last word is interpreted as a file and becomes the standard
        input to the command. None of the built-in commands (see below) support
        input redirection, but commands defined with Program do.
*   By default, commands are expected to succeed, and the test will fail
    otherwise. However, commands that are expected to fail can be marked with a
    `--> FAIL` suffix.

All test files in the same directory make up a test suite. See the TestSuite
documentation for the syntax of test files, and the `testdata/` directory for
examples.

## Commands

`cmdtest` comes with the following built-in commands:

*   cd DIR
*   cat FILE
*   mkdir DIR
*   setenv VAR VALUE
*   echo ARG1 ARG2 ...
*   fecho FILE ARG1 ARG2 ...

These all have their usual Unix shell meaning, except for `fecho`, which writes
its arguments to a file (output redirection is not supported). All file and
directory arguments must refer to the current directory; that is, they cannot
contain slashes.

You can add your own custom commands by adding them to the `TestSuite.Commands`
map; keep reading for an example.

## Variable substitution

`cmdtest` does its own environment variable substitution, using the syntax
`${VAR}`. Test execution inherits the full environment of the test binary caller
(typically, your shell). The environment variable ROOTDIR is set to the
temporary directory created to run the test file.

## Writing the test

To test, first read the suite:

```go
ts, err := cmdtest.Read("testdata")
```

Next, configure the resulting `TestSuite` by adding a `Setup` function and/or
adding commands to the `Commands` map. In particular, you will want to add a
command for your CLI. `cmdtest` can't handle `os.Exit` being called during a
test, so refactor your `main` function like this:

```go
func main() {
        os.Exit(run())
}

func run() int {
    // Your previous main here, returning 0 for success.
}
```

Then, add the command for your CLI to the `TestSuite`:

```go
ts.Commands["my-cli"] = cmdtest.InProcessProgram("my-cli", run)
```

Finally, call `TestSuite.Run` with `false` to compare the expected output to the
actual output, `true` to update the expected output. Typically, this boolean
will be the value of a flag. So, your final test code will look something like:

```go
var update = flag.Bool("update", false, "update test files with results")

func TestCLI(t *testing.T) {
    ts, err := cmdtest.Read("testdata")
    if err != nil {
        t.Fatal(err)
    }
    ts.Commands["my-cli"] = cmdtest.InProcessProgram("my-cli", run)
    err := ts.Run(t, *update)
    if err != nil {
        t.Fatal(err)
    }
}
```
