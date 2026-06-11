package ai

import (
	"sync"

	"github.com/vovakirdan/durak/internal/adapters/textcmd"
	"github.com/vovakirdan/durak/internal/app"
)

// RawCommandTraceSink receives raw AI command attempts.
type RawCommandTraceSink interface {
	RecordRawCommandTrace(*RawCommandTrace)
}

// RawCommandTrace records one raw AI command attempt.
type RawCommandTrace struct {
	Prompt      TurnPrompt
	RawCommand  string
	CommandKind textcmd.Kind
	Decision    app.PlayerDecision
	Err         string
}

// MemoryTraceSink stores raw AI command traces in memory.
type MemoryTraceSink struct {
	mu     sync.Mutex
	traces []RawCommandTrace
}

// NewMemoryTraceSink creates an empty trace sink.
func NewMemoryTraceSink() *MemoryTraceSink {
	return &MemoryTraceSink{}
}

// RecordRawCommandTrace appends one trace.
func (s *MemoryTraceSink) RecordRawCommandTrace(trace *RawCommandTrace) {
	if s == nil || trace == nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.traces = append(s.traces, *trace)
}

// Traces returns a copy of recorded traces.
func (s *MemoryTraceSink) Traces() []RawCommandTrace {
	if s == nil {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]RawCommandTrace(nil), s.traces...)
}
