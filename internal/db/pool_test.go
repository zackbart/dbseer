package db

import (
	"testing"
	"time"
)

func TestSetDurationRuntimeParam(t *testing.T) {
	params := map[string]string{}
	setDurationRuntimeParam(params, "statement_timeout", 1500*time.Millisecond)
	if params["statement_timeout"] != "1500ms" {
		t.Fatalf("statement_timeout = %q, want 1500ms", params["statement_timeout"])
	}
}

func TestSetRuntimeParamIfMissing(t *testing.T) {
	params := map[string]string{"application_name": "custom"}
	setRuntimeParamIfMissing(params, "application_name", "dbseer")
	if params["application_name"] != "custom" {
		t.Fatalf("application_name was overwritten: %q", params["application_name"])
	}
}
