package hot

import "reflect"

// emptyableToPtr returns a pointer copy of value if it's nonzero.
// Otherwise, returns nil pointer.
func emptyableToPtr[T any](x T) *T {
	// Use reflection to check if the value is zero. This is necessary because
	// the zero value of generic types cannot be determined directly.
	isZero := reflect.ValueOf(&x).Elem().IsZero()
	if isZero {
		return nil
	}

	return &x
}

// assertValue panics with the given message if the condition is false.
// This is used for validating configuration parameters.
func assertValue(ok bool, msg string) {
	if !ok {
		panic(msg)
	}
}
