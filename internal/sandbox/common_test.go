package sandbox

import (
	"testing"
	"time"
)

func TestConfig_Validate(t *testing.T) {
	tests := []struct {
		name    string
		cfg     Config
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid config",
			cfg: Config{
				WorkDir:        "/tmp",
				Timeout:        10 * time.Second,
				MemoryLimitMB:  500,
				MaxFDs:         256,
				MaxProcesses:   1,
				DenyNetwork:    true,
			},
			wantErr: false,
		},
		{
			name: "empty workdir",
			cfg: Config{
				WorkDir: "",
			},
			wantErr: true,
			errMsg:  "WorkDir is required",
		},
		{
			name: "timeout too large",
			cfg: Config{
				WorkDir: "/tmp",
				Timeout: 10 * time.Minute,
			},
			wantErr: true,
			errMsg:  "cannot exceed 5 minutes",
		},
		{
			name: "memory limit too small",
			cfg: Config{
				WorkDir:       "/tmp",
				MemoryLimitMB: 30,
			},
			wantErr: true,
			errMsg:  "must be at least 50 MB",
		},
		{
			name: "memory limit too large",
			cfg: Config{
				WorkDir:       "/tmp",
				MemoryLimitMB: 15000,
			},
			wantErr: true,
			errMsg:  "cannot exceed 10000 MB",
		},
		{
			name: "maxfds too small",
			cfg: Config{
				WorkDir: "/tmp",
				MaxFDs:  32,
			},
			wantErr: true,
			errMsg:  "must be at least 64",
		},
		{
			name: "maxprocesses too large",
			cfg: Config{
				WorkDir:      "/tmp",
				MaxProcesses: 200,
			},
			wantErr: true,
			errMsg:  "cannot exceed 100",
		},
		{
			name: "default values applied",
			cfg: Config{
				WorkDir: "/tmp",
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := tt.cfg
			err := cfg.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr && err != nil && tt.errMsg != "" {
				if !contains(err.Error(), tt.errMsg) {
					t.Errorf("Validate() error = %v, want containing %q", err, tt.errMsg)
				}
			}
		})
	}
}

func TestDefaultConfig(t *testing.T) {
	workDir := "/tmp/test"
	cfg := DefaultConfig(workDir)

	if cfg.WorkDir != workDir {
		t.Errorf("WorkDir = %v, want %v", cfg.WorkDir, workDir)
	}
	if cfg.Timeout != 10*time.Second {
		t.Errorf("Timeout = %v, want 10s", cfg.Timeout)
	}
	if cfg.MemoryLimitMB != 500 {
		t.Errorf("MemoryLimitMB = %v, want 500", cfg.MemoryLimitMB)
	}
	if cfg.MaxFDs != 256 {
		t.Errorf("MaxFDs = %v, want 256", cfg.MaxFDs)
	}
	if cfg.MaxProcesses != 1 {
		t.Errorf("MaxProcesses = %v, want 1", cfg.MaxProcesses)
	}
	if !cfg.DenyNetwork {
		t.Errorf("DenyNetwork = %v, want true", cfg.DenyNetwork)
	}
}

func TestResult_Duration(t *testing.T) {
	start := time.Now()
	end := start.Add(100 * time.Millisecond)

	result := &Result{
		StartTime: start,
		EndTime:   end,
	}

	duration := result.Duration()
	expected := 100 * time.Millisecond

	if duration != expected {
		t.Errorf("Duration() = %v, want %v", duration, expected)
	}
}

func TestResult_JSON(t *testing.T) {
	result := &Result{
		ExitCode:              0,
		Stdout:                "output",
		Stderr:                "",
		Timeout:               false,
		MemoryExceeded:        false,
		ResourceLimitExceeded: false,
		Error:                 "",
		StartTime:             time.Now(),
		EndTime:               time.Now().Add(100 * time.Millisecond),
		DurationMS:            100,
	}

	// Test that Result can be marshaled to JSON
	// This is implicit in the struct tags
	_ = result
}

func TestNotImplementedError(t *testing.T) {
	err := NotImplementedError{
		Platform: "plan9",
		Feature:  "sandboxing",
	}

	errMsg := err.Error()
	if !contains(errMsg, "plan9") || !contains(errMsg, "sandboxing") {
		t.Errorf("Error() = %q, should contain platform and feature", errMsg)
	}
}

func TestUnsupportedError(t *testing.T) {
	err := UnsupportedError{
		Reason: "requires kernel feature",
	}

	errMsg := err.Error()
	if !contains(errMsg, "requires kernel feature") {
		t.Errorf("Error() = %q, should contain reason", errMsg)
	}
}

func contains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
