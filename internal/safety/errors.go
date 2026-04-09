package safety

import "fmt"

// SafetyError is a structured, human-readable error returned by the safety
// validation layer. It includes an actionable fix so users know exactly which
// flag resolves the rejection.
type SafetyError struct {
	// Code is a machine-readable slug identifying the kind of rejection.
	Code string
	// Host is the database host that triggered the rejection.
	Host string
	// Reason is a human-readable description of why the connection was refused.
	Reason string
	// Fix is the CLI flag or action the user should take to override.
	Fix string
}

// Error implements the error interface. The message is deliberately verbose so
// it is immediately actionable when printed to a terminal.
func (e *SafetyError) Error() string {
	return fmt.Sprintf(
		"dbseer: refusing to connect to %s\n\n  %s\n  this is a dev tool — it intentionally makes it hard to touch prod\n\nto override (if you're SURE this is a dev db):\n  %s\n\nto see what dbseer discovered without connecting:\n  dbseer --which\n",
		e.Host,
		e.Reason,
		e.Fix,
	)
}

// NewProdError returns a SafetyError for a connection rejected because the host
// matches a production-host pattern.
func NewProdError(host, reason string) *SafetyError {
	return &SafetyError{
		Code:   "prod_host",
		Host:   host,
		Reason: "detected host matches prod pattern: " + reason,
		Fix:    "dbseer --allow-prod",
	}
}

// NewRemoteError returns a SafetyError for a connection rejected because the
// host is not a localhost address.
func NewRemoteError(host string) *SafetyError {
	return &SafetyError{
		Code:   "remote_host",
		Host:   host,
		Reason: "host is not a localhost address",
		Fix:    "dbseer --allow-remote",
	}
}

// NewBindError returns a SafetyError for a bind host that dbseer refuses to
// use in v0.1.
func NewBindError(host string) *SafetyError {
	return &SafetyError{
		Code:   "localhost_bind",
		Host:   host,
		Reason: "bind address is not a loopback interface",
		Fix:    "--host <addr>  (accepted values: 127.0.0.1, localhost, ::1)",
	}
}
