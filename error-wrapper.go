package utils

import "fmt"

type errorWrapper struct {
	msg  string
	prev error
}

func WrapError(msg string, prev error) error {
	if prev == nil {
		return nil
	}
	return &errorWrapper{
		msg:  msg,
		prev: prev,
	}
}

func (wErr *errorWrapper) Error() (errStr string) {
	errStr = wErr.msg

	prev := wErr.prev
	for prev != nil {
		if cur, ok := prev.(*errorWrapper); ok {
			errStr = fmt.Sprintf("%s <<= %s", errStr, cur.msg)
			prev = cur.prev
		} else {
			errStr = fmt.Sprintf("%s <<= %s", errStr, prev.Error())
			prev = nil
		}
	}
	return
}
