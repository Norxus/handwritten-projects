package compose

import (
	"errors"
	"fmt"
	"reflect"
)

var ErrExceedMaxSteps = errors.New("exceed max steps")

func newUnexpectedInputTypeErr(expected reflect.Type, got reflect.Type) error {
	return fmt.Errorf("unexpected input type, expected: %v, got %v", expected, got)
}

func newStreamReadError(err error) error {
	return fmt.Errorf("failed to read from stream. error: %w", err)
}
