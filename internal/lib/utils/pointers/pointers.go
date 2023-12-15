package pointers

// Pointer casts T type to *T
func Pointer[T any](t T) *T {
	return &t
}
