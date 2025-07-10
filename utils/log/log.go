package log

import "fmt"

const (
	Reset = "\033[0m"
	Red   = "\033[31m"
	Green = "\033[32m"
)

func Errorf(format string, args ...interface{}) {
	fmt.Printf(Red+format+Reset, args...)
}

func SError(msg string) string {
	return Red + msg + Reset
}

func OKf(format string, args ...interface{}) {
	fmt.Printf(Green+format+Reset, args...)
}

func SOK(msg string) string {
	return Green + msg + Reset
}
