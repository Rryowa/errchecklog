package library

type Printer interface {
	Print(string)
}

type FakefmtPrinter struct{}

func (f *FakefmtPrinter) Print(s string) {
	// do nothing
}

type NotAPrinter interface {
	NotAPrint(string)
}

func (f *FakefmtPrinter) NotAPrint(s string) {
	// do nothing
}
