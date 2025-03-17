package testdata

import (
	"github.com/errchecklog/fakefmt"
	"github.com/errchecklog/library"
)

type Handler struct {
	printer     fakefmt.Printer
	notAPrinter fakefmt.NotAPrinter
}

func New(printer fakefmt.Printer, aPrinter fakefmt.NotAPrinter) *Handler {
	return &Handler{
		printer: printer,
	}
}

func c() {
	var printerObj fakefmt.Printer = &library.FakefmtPrinter{}
	var notAPrinter fakefmt.NotAPrinter = &library.FakefmtPrinter{}
	var handler = New(printerObj, notAPrinter)

	handler.printer.Print("Hello, world!") // want "call to a provided interface found"
	handler.notAPrinter.NotAPrint("Hello, world!")
}
