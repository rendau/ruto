package gateway

type Heartbeat struct {
	GatewayID        string
	HostName         string
	SnapshotVersion  string
	LastApplyAtUnix  int64
	StartedAtUnix    int64
	LastError        string
	MemoryAllocBytes uint64
	GoroutinesCount  uint32
}

type Item struct {
	GatewayID        string `json:"gateway_id"`
	HostName         string `json:"host_name"`
	SnapshotVersion  string `json:"snapshot_version"`
	LastApplyAtUnix  int64  `json:"last_apply_at_unix"`
	StartedAtUnix    int64  `json:"started_at_unix"`
	LastError        string `json:"last_error"`
	LastSeenAtUnix   int64  `json:"last_seen_at_unix"`
	Status           string `json:"status"`
	MemoryAllocBytes uint64 `json:"memory_alloc_bytes"`
	GoroutinesCount  uint32 `json:"goroutines_count"`
}
