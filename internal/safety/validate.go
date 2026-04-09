package safety

// Options controls which safety checks are bypassed.
type Options struct {
	// AllowRemote, when true, permits connections to non-localhost hosts.
	AllowRemote bool
	// AllowProd, when true, permits connections to hosts matching production patterns.
	AllowProd bool
	// Readonly indicates the session should be opened in read-only mode.
	// This field is informational within the safety package; enforcement is done
	// by the database layer via "default_transaction_read_only=on".
	Readonly bool
}

// ValidateURL checks whether the given URLInfo is safe to connect to given the
// provided options.
//
// Call order invariant (pinned per plan C2):
//  1. If IsLocalhost is false and AllowRemote is false → reject with NewRemoteError.
//  2. If IsProdHost matches and AllowProd is false → reject with NewProdError.
//  3. Otherwise return nil.
func ValidateURL(info URLInfo, opts Options) error {
	if !info.IsLocalhost && !opts.AllowRemote {
		return NewRemoteError(info.Host)
	}

	if isProd, reason := IsProdHost(info.Host); isProd && !opts.AllowProd {
		return NewProdError(info.Host, reason)
	}

	return nil
}

// ValidateBind checks whether host is an acceptable bind address for the
// dbseer HTTP server. In v0.1, only loopback addresses are accepted.
// Any other value is rejected with a clear error explaining the restriction.
func ValidateBind(host string) error {
	switch host {
	case "127.0.0.1", "localhost", "::1":
		return nil
	}
	return &SafetyError{
		Code:   "localhost_bind",
		Host:   host,
		Reason: "dbseer binds to localhost only — this is a dev tool",
		Fix:    "accepted values: 127.0.0.1, localhost, ::1  (no flag exists to change this in v0.1)",
	}
}
