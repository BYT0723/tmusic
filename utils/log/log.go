package log

import "fmt"

const (
	Reset = "\033[0m"
	Red   = "\033[31m"
	Green = "\033[32m"
)

func Errorf(format string, args ...any) {
	fmt.Printf(Red+format+Reset, args...)
}

func SError(msg string) string {
	return Red + msg + Reset
}

func Infof(format string, args ...any) {
	fmt.Printf(Green+format+Reset, args...)
}

func SInfo(msg string) string {
	return Green + msg + Reset
}
