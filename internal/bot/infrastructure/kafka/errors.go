package consumer

import (
	"fmt"
)

type ErrAvroMsg struct {
	Op  string
	Err error
}

func NewErrAvroMsg(op string, err error) error {
	return ErrAvroMsg{Op: op, Err: err}
}

func (e ErrAvroMsg) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("%s: %v", e.Op, e.Err)
	}

	return e.Op
}

func (e ErrAvroMsg) Unwrap() error {
	return e.Err
}
