package manifest

type IndexManifest struct {
	CurrentSeq         uint64 `json:"current_seq"`
	ETag               string `json:"etag"`
	CurrentSnapshotSeq uint64 `json:"current_snapshot_seq"`
}
