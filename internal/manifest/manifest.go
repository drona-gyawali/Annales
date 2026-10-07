package manifest

type IndexManifest struct {
	CurrentSeq         uint64 `json:"current_seq"`
	CurrentLogKey      string `json:"current_log_key"`
	CurrentSnapshotSeq uint64 `json:"current_snapshot_seq"`
}
