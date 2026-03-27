// Copyright Contributors to the KubeOpenCode project

package handlers

import (
	"testing"
)

func TestValidateServerURL(t *testing.T) {
	tests := []struct {
		name    string
		url     string
		wantErr bool
	}{
		{
			name:    "valid cluster-local URL",
			url:     "http://my-agent.default.svc.cluster.local:4096",
			wantErr: false,
		},
		{
			name:    "valid URL with custom port",
			url:     "http://agent.kubeopencode-system.svc.cluster.local:8080",
			wantErr: false,
		},
		{
			name:    "https scheme rejected",
			url:     "https://my-agent.default.svc.cluster.local:4096",
			wantErr: true,
		},
		{
			name:    "ftp scheme rejected",
			url:     "ftp://my-agent.default.svc.cluster.local:4096",
			wantErr: true,
		},
		{
			name:    "file scheme rejected",
			url:     "file:///etc/passwd",
			wantErr: true,
		},
		{
			name:    "external host rejected",
			url:     "http://evil.example.com:4096",
			wantErr: true,
		},
		{
			name:    "localhost rejected",
			url:     "http://localhost:4096",
			wantErr: true,
		},
		{
			name:    "IP address rejected",
			url:     "http://10.0.0.1:4096",
			wantErr: true,
		},
		{
			name:    "metadata endpoint rejected",
			url:     "http://169.254.169.254",
			wantErr: true,
		},
		{
			name:    "userinfo rejected",
			url:     "http://user:pass@my-agent.default.svc.cluster.local:4096",
			wantErr: true,
		},
		{
			name:    "empty URL rejected",
			url:     "",
			wantErr: true,
		},
		{
			name:    "partial cluster.local suffix rejected",
			url:     "http://evil.cluster.local:4096",
			wantErr: true,
		},
		{
			name:    "svc.cluster.local without service name",
			url:     "http://svc.cluster.local:4096",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateServerURL(tt.url)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateServerURL(%q) error = %v, wantErr %v", tt.url, err, tt.wantErr)
			}
		})
	}
}
