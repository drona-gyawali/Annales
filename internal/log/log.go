package log

type LogEntry struct {
	Operation string `json:"operation"`
	Key       string `json:"key"`
	Value     string `json:"value"`
}

type LogBatch struct {
	SequenceID uint64     `json:"sequence_id"`
	PrevLogKey string     `json:"prev_log_key"`
	Operations []LogEntry `json:"operations"`
}
