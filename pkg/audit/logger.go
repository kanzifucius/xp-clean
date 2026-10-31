package audit

import (
	"encoding/json"
	"io"
	"os"
	"time"
)

// ObjectRef is the object identity recorded in an audit entry.
type ObjectRef struct {
	APIVersion string `json:"apiVersion"`
	Kind       string `json:"kind"`
	Namespace  string `json:"namespace,omitempty"`
	Name       string `json:"name"`
	UID        string `json:"uid,omitempty"`
}

// Entry is one structured audit event (stdout JSON for MVP).
type Entry struct {
	TS        time.Time `json:"ts"`
	Policy    string    `json:"policy"`
	TraceID   string    `json:"traceId"`
	Action    string    `json:"action"`
	DryRun    bool      `json:"dryRun"`
	Object    ObjectRef `json:"object"`
	Owner     string    `json:"owner,omitempty"`
	Reason    string    `json:"reason,omitempty"`
	ExpiresAt string    `json:"expiresAt,omitempty"`
}

// Logger writes JSON audit lines.
type Logger struct {
	out io.Writer
}

// NewLogger returns a logger writing to w (defaults to stdout).
func NewLogger(w io.Writer) *Logger {
	if w == nil {
		w = os.Stdout
	}
	return &Logger{out: w}
}

// Log writes one audit entry with ts filled if zero.
func (l *Logger) Log(e Entry) error {
	if e.TS.IsZero() {
		e.TS = time.Now().UTC()
	}
	enc := json.NewEncoder(l.out)
	return enc.Encode(e)
}

// LogAll writes every entry.
func (l *Logger) LogAll(entries []Entry) error {
	for _, e := range entries {
		if err := l.Log(e); err != nil {
			return err
		}
	}
	return nil
}
