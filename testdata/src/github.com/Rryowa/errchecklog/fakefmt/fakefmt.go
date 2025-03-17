package fakefmt

// Printer is an interface with a Print method.
type Printer interface {
	Print(string)
}

// Printer is an interface with a Print method.
type NotAPrinter interface {
	NotAPrint(string)
}
