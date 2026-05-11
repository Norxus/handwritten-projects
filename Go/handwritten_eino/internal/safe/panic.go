package safe

import "fmt"

type panicErr struct {
	info  any
	stack []byte
}

func (p *panicErr) Error() string {
	return fmt.Sprintf("panic error: %v, \nstack: %s", p.info, string(p.stack))
}

func NewPanicErr(info any, stack []byte) error {
	return &panicErr{
		info:  info,
		stack: stack,
	}
}
