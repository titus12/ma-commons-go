package errors

import "fmt"

type ServerError struct {
	err string
	sid int32
}

func NewServerError(err string, id int32) error {
	return &ServerError{err: err, sid: id}
}

func (e *ServerError) Error() string {
	return fmt.Sprintf("%v,%v", e.sid, e.err)
}

func (e *ServerError) GetSid() int32 {
	return e.sid
}
