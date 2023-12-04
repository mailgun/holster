package slice

// Remove removes the given index range from the given slice, preserving element order.
// It is the safer version of slices.Delete() as it will
// not panic on invalid slice ranges. Nil and empty slices are also safe.
// Remove modifies the contents of the given slice and performs no allocations
// or copies.
func Remove[T comparable](slice []T, i, j int) []T {
	// Nothing to do for emtpy slices or starting indecies outside range
	if len(slice) == 0 || i > len(slice) {
		return slice
	}
	// Prevent invalid slice indexing
	if i < 0 || j > len(slice) {
		return slice
	}
	// Removing the last element is a simple re-slice
	if i == len(slice)-1 && j >= len(slice) {
		return slice[:i]
	}
	// Note: this modifies the slice
	return append(slice[:i], slice[j:]...)
}
