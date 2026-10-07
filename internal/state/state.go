package state

import "github.com/drona-gyawali/logStore/internal/log"

type State struct {
	data map[string]string
}

func New() *State {
	return &State{
		data: make(map[string]string),
	}
}

func (s *State) Apply(entry log.LogEntry) {
	switch entry.Operation {
	case "SET":
		s.data[entry.Key] = entry.Value

	case "DELETE":
		delete(s.data, entry.Key)
	}
}

func (s *State) ApplyBatch(batch log.LogBatch) {
	for _, entry := range batch.Operations {
		s.Apply(entry)
	}
}

func (s *State) Get(key string) (string, bool) {
	value, ok := s.data[key]
	return value, ok
}

func (s *State) Load(data map[string]string) {
	s.data = data
}
