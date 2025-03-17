package library

type Printer interface {
	Print(string)
}

type FakefmtPrinter struct{}

func (f *FakefmtPrinter) Print(s string) {
	// do nothing
}
