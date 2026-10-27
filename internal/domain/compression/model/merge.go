package model

import "slices"

// Merge fills what the child leaves empty from the parent: every field set on
// the child wins as a whole (the algorithm list is not unioned, it is an
// order of preference).
func Merge(parent, child Compression) Compression {
	result := child

	if result.Backend == "" {
		result.Backend = parent.Backend
	}
	if result.Response == "" {
		result.Response = parent.Response
	}
	if len(result.Algorithms) == 0 {
		result.Algorithms = slices.Clone(parent.Algorithms)
	}
	if result.MinSize == 0 {
		result.MinSize = parent.MinSize
	}
	if len(result.ContentTypes) == 0 {
		result.ContentTypes = slices.Clone(parent.ContentTypes)
	}

	return result
}
