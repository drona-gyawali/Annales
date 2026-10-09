package snapshot

type Snapshot struct {
	SequenceID uint64            `json:"sequence_id"`
	LastLogKey string            `json:"last_log_key"`
	Data       map[string]string `json:"data"`
}
