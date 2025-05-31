package parallel

import (
	"os"
	"testing"
)

func TestParseFlags(t *testing.T) {
	defaultConfigPath := ".parallelrc.yaml"

	tests := []struct {
		name          string
		args          []string
		options       []Option
		expectedPath  string
		expectedError string
	}{
		{
			name:          "valid config flag",
			args:          []string{"-f", "/path/to/config.yaml"},
			expectedPath:  "/path/to/config.yaml",
			expectedError: "",
		},
		{
			name:          "missing config flag",
			args:          []string{},
			expectedPath:  defaultConfigPath,
			expectedError: "",
		},
		{
			name:          "empty config flag value",
			args:          []string{"-f", ""},
			expectedPath:  "",
			expectedError: "config file path cannot be empty",
		},
		{
			name:          "invalid flag",
			args:          []string{"-unknown"},
			expectedPath:  "",
			expectedError: "parsing flags: flag provided but not defined: -unknown",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			originalArgs := os.Args
			defer func() { os.Args = originalArgs }()

			// Simulate command-line arguments
			os.Args = append([]string{"cmd"}, tt.args...)

			cfg, err := ParseFlags(tt.options...)

			if (err != nil) != (tt.expectedError != "") {
				t.Errorf("unexpected error state: got %v, want error: %v", err, tt.expectedError)
			} else if err != nil && tt.expectedError != "" && err.Error() != tt.expectedError {
				t.Errorf("unexpected error: got %v, want %v", err, tt.expectedError)
			}

			if cfg != nil && cfg.ConfigFilePath != tt.expectedPath {
				t.Errorf("unexpected config path: got %v, want %v", cfg.ConfigFilePath, tt.expectedPath)
			}
		})
	}
}
