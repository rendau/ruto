package gateway

type Heartbeat struct {
	GatewayID       string
	PodName         string
	HostName        string
	SnapshotVersion string
	LastApplyAtUnix int64
	StartedAtUnix   int64
	LastError       string
}

type Item struct {
	GatewayID       string `json:"gateway_id"`
	PodName         string `json:"pod_name"`
	HostName        string `json:"host_name"`
	SnapshotVersion string `json:"snapshot_version"`
	LastApplyAtUnix int64  `json:"last_apply_at_unix"`
	StartedAtUnix   int64  `json:"started_at_unix"`
	LastError       string `json:"last_error"`
	LastSeenAtUnix  int64  `json:"last_seen_at_unix"`
	Status          string `json:"status"`
}
