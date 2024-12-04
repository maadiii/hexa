package helper

func FromPointer[T any](value *T) T {
	if value != nil {
		return *value
	}

	var zero T

	return zero
}

func ToPointer[T any](value T) *T {
	return &value
}
