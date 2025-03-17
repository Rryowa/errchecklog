package testdata

import (
	"github.com/errchecklog/fakefmt"
	"github.com/errchecklog/library"
)

func f() {
	var p fakefmt.Printer = &library.FakefmtPrinter{}
	p.Print("Hello, world!") // want "call to a provided interface found"
}
