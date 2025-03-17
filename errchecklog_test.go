package errchecklog

import (
	"testing"

	"golang.org/x/tools/go/analysis/analysistest"
)

func TestErrCheckLog(t *testing.T) {
	testdata := analysistest.TestData()
	analysistest.Run(t, testdata, NewAnalyzer("fakefmt", "Printer"))
}
