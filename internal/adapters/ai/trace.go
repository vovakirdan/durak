package ai

import (
	"sync"
	"time"

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
	Client      ClientInfo
	StartedAt   time.Time
	Duration    time.Duration
	RawCommand  string
	Usage       TokenUsage
	CommandKind textcmd.Kind
	Decision    app.PlayerDecision
	Err         string
}

// CombineTraceSinks fans out traces to every non-nil sink.
func CombineTraceSinks(sinks ...RawCommandTraceSink) RawCommandTraceSink {
	filtered := make([]RawCommandTraceSink, 0, len(sinks))
	for _, sink := range sinks {
		if sink != nil {
			filtered = append(filtered, sink)
		}
	}
	if len(filtered) == 0 {
		return nil
	}
	if len(filtered) == 1 {
		return filtered[0]
	}
	return multiTraceSink(filtered)
}

type multiTraceSink []RawCommandTraceSink

// RecordRawCommandTrace forwards one trace to every configured sink.
func (s multiTraceSink) RecordRawCommandTrace(trace *RawCommandTrace) {
	for _, sink := range s {
		sink.RecordRawCommandTrace(trace)
	}
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
	s.traces = append(s.traces, cloneRawCommandTrace(trace))
}

// Traces returns a copy of recorded traces.
func (s *MemoryTraceSink) Traces() []RawCommandTrace {
	if s == nil {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	traces := make([]RawCommandTrace, 0, len(s.traces))
	for index := range s.traces {
		traces = append(traces, cloneRawCommandTrace(&s.traces[index]))
	}
	return traces
}

func cloneRawCommandTrace(trace *RawCommandTrace) RawCommandTrace {
	if trace == nil {
		return RawCommandTrace{}
	}
	cloned := *trace
	cloned.Prompt = cloneTurnPrompt(&trace.Prompt)
	return cloned
}

func clientInfo(client Client) ClientInfo {
	provider, ok := client.(ClientInfoProvider)
	if !ok {
		return ClientInfo{}
	}
	return provider.ClientInfo()
}
