package safety

import (
	"errors"
	"testing"
)

func makeInfo(host string) URLInfo {
	return URLInfo{
		Raw:         "postgres://" + host + ":5432/dev",
		Host:        host,
		Port:        "5432",
		Database:    "dev",
		User:        "user",
		IsLocalhost: isLocalhost(host),
	}
}

func TestValidateURL(t *testing.T) {
	cases := []struct {
		name        string
		info        URLInfo
		opts        Options
		wantErr     bool
		wantCode    string
	}{
		{
			name:    "localhost always passes",
			info:    makeInfo("localhost"),
			opts:    Options{},
			wantErr: false,
		},
		{
			name:    "127.0.0.1 always passes",
			info:    makeInfo("127.0.0.1"),
			opts:    Options{},
			wantErr: false,
		},
		{
			name:     "remote without AllowRemote -> error",
			info:     makeInfo("192.168.1.100"),
			opts:     Options{},
			wantErr:  true,
			wantCode: "remote_host",
		},
		{
			name:    "remote with AllowRemote -> passes if not prod",
			info:    makeInfo("192.168.1.100"),
			opts:    Options{AllowRemote: true},
			wantErr: false,
		},
		{
			name:     "prod-pattern remote without AllowRemote -> remote_host error (remote check first)",
			info:     makeInfo("mydb.us-east-1.rds.amazonaws.com"),
			opts:     Options{},
			wantErr:  true,
			wantCode: "remote_host",
		},
		{
			name:     "prod-pattern remote with AllowRemote but without AllowProd -> prod_host error",
			info:     makeInfo("mydb.us-east-1.rds.amazonaws.com"),
			opts:     Options{AllowRemote: true},
			wantErr:  true,
			wantCode: "prod_host",
		},
		{
			name:    "prod-pattern with both AllowRemote and AllowProd -> passes",
			info:    makeInfo("mydb.us-east-1.rds.amazonaws.com"),
			opts:    Options{AllowRemote: true, AllowProd: true},
			wantErr: false,
		},
		{
			name:    "readonly flag does not affect validation",
			info:    makeInfo("localhost"),
			opts:    Options{Readonly: true},
			wantErr: false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := ValidateURL(tc.info, tc.opts)
			if tc.wantErr && err == nil {
				t.Fatal("expected error, got nil")
			}
			if !tc.wantErr && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if tc.wantErr && tc.wantCode != "" {
				var se *SafetyError
				if !errors.As(err, &se) {
					t.Fatalf("expected *SafetyError, got %T: %v", err, err)
				}
				if se.Code != tc.wantCode {
					t.Errorf("SafetyError.Code = %q, want %q", se.Code, tc.wantCode)
				}
			}
		})
	}
}

func TestValidateBind(t *testing.T) {
	cases := []struct {
		host    string
		wantErr bool
	}{
		{"127.0.0.1", false},
		{"localhost", false},
		{"::1", false},
		{"0.0.0.0", true},
		{"192.168.1.1", true},
		{"", true},
	}

	for _, tc := range cases {
		err := ValidateBind(tc.host)
		if tc.wantErr && err == nil {
			t.Errorf("ValidateBind(%q): expected error, got nil", tc.host)
		}
		if !tc.wantErr && err != nil {
			t.Errorf("ValidateBind(%q): unexpected error: %v", tc.host, err)
		}
	}
}
