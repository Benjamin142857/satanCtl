package main

type StCtlError struct {
	errMsg string
}

func (e *StCtlError) Error() string {
	return e.errMsg
}
func newStCtlError(errMsg string) *StCtlError {
	return &StCtlError{errMsg: errMsg}
}