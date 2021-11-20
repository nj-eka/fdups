package errs

import (
	cou "github.com/nj-eka/fdups/contexts"
	"time"
)

type Error interface {
	error
	Severity() Severity
	TimeStamp() time.Time
	Kind() Kind
	OperationPath() cou.Operations
	StackTrace() []Frame
	Unwrap() error
}
