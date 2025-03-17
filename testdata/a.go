package testdata

import (
	"github.com/errchecklog/fakefmt"
	"github.com/errchecklog/library"
)

func a() {
	var p fakefmt.Printer = &library.FakefmtPrinter{}
	p.Print("Hello, world!") // want "call to a provided interface found"
}

func b() {
	var p fakefmt.NotAPrinter = &library.FakefmtPrinter{}
	p.NotAPrint("Hello, world!")
}
