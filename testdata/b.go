package testdata

import (
	"github.com/errchecklog/fakefmt"
	"github.com/errchecklog/library"
)

type Handler struct {
	printer fakefmt.Printer
}

func New(printer fakefmt.Printer) *Handler {
	return &Handler{
		printer: printer,
	}
}

func c() {
	var printerObj fakefmt.Printer = &library.FakefmtPrinter{}
	var handler = New(printerObj)

	handler.printer.Print("Hello, world!") // want "call to a provided interface found"
}
