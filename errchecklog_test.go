package main

import (
	"testing"

	"golang.org/x/tools/go/analysis/analysistest"
)

func TestCallcheck(t *testing.T) {
	testdata := analysistest.TestData()
	//analysistest.Run(t, testdata, NewAnalyzer(strings.Join([]string{testdata, "src/github.com/errchecklog/fakefmt"}, "/")))
	analysistest.Run(t, testdata, NewAnalyzer("fakefmt"))
}
