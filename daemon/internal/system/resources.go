package system

// Resources holds basic memory information for the resources component.
type Resources struct {
	TotalMemoryBytes     uint64
	AvailableMemoryBytes uint64 // 0 when unavailable on this platform.
	MemoryNote           string // human-readable explanation when note=0.
}

// Memory returns installed memory. Available memory is 0 on platforms where
// it is not easily accessible without elevated privileges.
func Memory() (Resources, error) {
	return memory()
}
