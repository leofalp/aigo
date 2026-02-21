package utils

// Ptr returns a pointer to v. It is a generic convenience helper that avoids
// the need for a temporary variable when the address of a literal or computed
// value must be passed where a pointer is expected.
//
// Example:
//
//	timeout := utils.Ptr(30 * time.Second)
func Ptr[T any](v T) *T {
	return &v
}
