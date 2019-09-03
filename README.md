[![Build Status](https://travis-ci.org/google/cmdtest.svg?branch=master)](https://travis-ci.org/google/cmdtest)
[![godoc](https://godoc.org/github.com/google/cmdtest?status.svg)](https://godoc.org/github.com/google/cmdtest)
[![Go Report Card](https://goreportcard.com/badge/github.com/google/cmdtest)](https://goreportcard.com/report/github.com/google/cmdtest)

# Testing your CLI

The cmdtest package simplifies testing of command-line interfaces. It
provides a simple, cross-platform, shell-like language to express command
execution. It can compare actual output with the expected output, and can
also update a file with new "golden" output that is deemed correct.

Start using cmdtest by writing a test file with commands and expected output,
giving it the extension ".ct". All test files in the same directory make up a
test suite. See the TestSuite documentation for the syntax of test files.

To test, first read the suite:

```go
ts, err := cmdtest.Read("testdata")
```

Then configure the resulting `TestSuite` by adding commands or enabling
debugging features. Lastly, call `TestSuite.Run` with `false` to compare
or `true` to update. Typically, this boolean will be the value of a flag:

```go
var update = flag.Bool("update", false, "update test files with results")
...
err := ts.Run(*update)
```

TODO(rvangent): Fill in more.