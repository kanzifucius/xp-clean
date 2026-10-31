package exit

import "errors"

// Process exit codes for xp-clean (documented in the architecture plan §8).
const (
	Success = 0
	Runtime = 1
	Usage   = 2
	// Blocked is returned when --fail-on-blocked is set and the run completed with blocked plans.
	Blocked = 3
)

// Code is an error that carries a preferred process exit status.
type Code struct {
	Err  error
	Code int
}

func (e *Code) Error() string {
	if e.Err == nil {
		return ""
	}
	return e.Err.Error()
}

func (e *Code) Unwrap() error { return e.Err }

// UsageErr marks a validation / argv error (exit 2).
func UsageErr(err error) error {
	if err == nil {
		return nil
	}
	return &Code{Err: err, Code: Usage}
}

// RuntimeErr marks a runtime failure (exit 1).
func RuntimeErr(err error) error {
	if err == nil {
		return nil
	}
	return &Code{Err: err, Code: Runtime}
}

// BlockedErr marks a completed run that had blocked plans (exit 3).
func BlockedErr(err error) error {
	if err == nil {
		return nil
	}
	return &Code{Err: err, Code: Blocked}
}

// Status extracts an exit code from err (default Runtime).
func Status(err error) int {
	if err == nil {
		return Success
	}
	var c *Code
	if errors.As(err, &c) {
		return c.Code
	}
	return Runtime
}
