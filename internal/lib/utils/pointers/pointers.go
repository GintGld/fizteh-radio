package ptr

// Pointer casts T type to *T
func Ptr[T any](t T) *T {
	return &t
}
