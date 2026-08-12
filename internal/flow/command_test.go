package flow

import (
	"errors"
	"testing"
)

func TestCommand_Validate(t *testing.T) {
	tests := []struct {
		name      string
		cmd       Command
		wantError bool
	}{
		{"valid command", Command{Cmd: "ls"}, false},
		{"empty command", Command{Cmd: ""}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.cmd.Validate()
			if (err != nil) != tt.wantError {
				t.Errorf("unexpected result for Validate: got err = %v, wantError = %v", err, tt.wantError)
			}

			if tt.wantError && !errors.Is(err, ErrEmptyCommand) {
				t.Errorf("expected ErrEmptyCommand, got %v", err)
			}
		})
	}
}

func TestCommand_DisplayName(t *testing.T) {
	tests := []struct {
		name     string
		cmd      Command
		expected string
	}{
		{"explicit name wins", Command{Name: "worker", Cmd: "php"}, "worker"},
		{"falls back to executable", Command{Cmd: "php"}, "php"},
		{"empty both", Command{}, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.cmd.DisplayName(); got != tt.expected {
				t.Errorf("DisplayName() = %q, want %q", got, tt.expected)
			}
		})
	}
}
