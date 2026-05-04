package agentsdk

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

// Session is the persistence surface for conversation history. The Agent
// loop reads via Messages, appends via Append, and resets via Reset (used
// when a caller wants a fresh conversation under the same id).
type Session interface {
	ID() string
	Append(message Message) error
	Messages() []Message
	Reset() error
}

// MemorySession keeps history in process memory; it is the right choice
// for one-shot tasks where persistence between processes is not needed.
type MemorySession struct {
	id       string
	mu       sync.Mutex
	messages []Message
}

// NewMemorySession returns an empty session under id. An empty id is fine —
// the runtime treats unidentified sessions as ephemeral.
func NewMemorySession(id string) *MemorySession {
	return &MemorySession{id: id}
}

// ID implements Session.
func (s *MemorySession) ID() string { return s.id }

// Append implements Session.
func (s *MemorySession) Append(message Message) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.messages = append(s.messages, message)
	return nil
}

// Messages implements Session and returns a defensive copy.
func (s *MemorySession) Messages() []Message {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]Message, len(s.messages))
	copy(out, s.messages)
	return out
}

// Reset clears the history.
func (s *MemorySession) Reset() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.messages = nil
	return nil
}

// FileSession persists messages as JSON-Lines so a future agentctl session
// resume can replay the transcript.
//
// Append acquires writeMu across the disk write and the memory update so
// on-disk and in-memory order stay aligned. Messages reads acquire only
// memMu (an RWMutex), so concurrent readers don't queue behind a slow
// fsync. Reset acquires both — it truncates disk and clears memory.
type FileSession struct {
	id      string
	path    string
	writeMu sync.Mutex
	memMu   sync.RWMutex
	mem     []Message
}

// NewFileSession opens (or creates) the session file at path and returns a
// session bound to it. Existing content is parsed at construction so the
// agent loop sees prior history immediately.
func NewFileSession(id string, path string) (*FileSession, error) {
	if path == "" {
		return nil, errors.New("file session path is required")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, err
	}
	session := &FileSession{id: id, path: path}
	if err := session.load(); err != nil {
		return nil, err
	}
	return session, nil
}

// ID implements Session.
func (s *FileSession) ID() string { return s.id }

// Append implements Session.
func (s *FileSession) Append(message Message) error {
	encoded, err := json.Marshal(message)
	if err != nil {
		return fmt.Errorf("encode message: %w", err)
	}
	encoded = append(encoded, '\n')

	s.writeMu.Lock()
	defer s.writeMu.Unlock()

	file, err := os.OpenFile(s.path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	if _, writeErr := file.Write(encoded); writeErr != nil {
		_ = file.Close()
		return writeErr
	}
	if closeErr := file.Close(); closeErr != nil {
		return closeErr
	}

	s.memMu.Lock()
	s.mem = append(s.mem, message)
	s.memMu.Unlock()
	return nil
}

// Messages implements Session. Read-only callers do not block on Append.
func (s *FileSession) Messages() []Message {
	s.memMu.RLock()
	defer s.memMu.RUnlock()
	out := make([]Message, len(s.mem))
	copy(out, s.mem)
	return out
}

// Reset clears history both on disk and in memory.
func (s *FileSession) Reset() error {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	if err := os.WriteFile(s.path, nil, 0o600); err != nil {
		return err
	}
	s.memMu.Lock()
	s.mem = nil
	s.memMu.Unlock()
	return nil
}

func (s *FileSession) load() error {
	file, err := os.Open(s.path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 4096), 4*1024*1024)
	for scanner.Scan() {
		raw := scanner.Bytes()
		if len(raw) == 0 {
			continue
		}
		var message Message
		if err := json.Unmarshal(raw, &message); err != nil {
			return fmt.Errorf("decode session line: %w", err)
		}
		s.mem = append(s.mem, message)
	}
	return scanner.Err()
}
