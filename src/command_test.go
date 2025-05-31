package parallel

import (
	"testing"
)

func TestNewCommand(t *testing.T) {
	tests := []struct {
		name     string
		cmd      string
		args     []string
		expected *Command
	}{
		{"valid command", "ls", []string{"-a"}, &Command{Cmd: "ls", Args: []string{"-a"}, Pipe: false}},
		{"empty command", "", nil, &Command{Cmd: "", Args: nil, Pipe: false}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := NewCommand(tt.cmd, tt.args...)
			if result.Cmd != tt.expected.Cmd || len(result.Args) != len(tt.expected.Args) || result.Pipe != tt.expected.Pipe {
				t.Errorf("unexpected result: got %+v, want %+v", result, tt.expected)
			}
		})
	}
}

func TestCommand_SetArguments(t *testing.T) {
	tests := []struct {
		name     string
		initial  *Command
		args     []string
		expected []string
	}{
		{"set args", &Command{}, []string{"arg1", "arg2"}, []string{"arg1", "arg2"}},
		{"replace args", &Command{Args: []string{"old"}}, []string{"new1", "new2"}, []string{"new1", "new2"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.initial.SetArguments(tt.args...)
			if len(tt.initial.Args) != len(tt.expected) {
				t.Errorf("unexpected arg length: got %d, want %d", len(tt.initial.Args), len(tt.expected))
			}
			for i, arg := range tt.initial.Args {
				if arg != tt.expected[i] {
					t.Errorf("unexpected arg: got %s, want %s", arg, tt.expected[i])
				}
			}
		})
	}
}

func TestCommand_SetDir(t *testing.T) {
	tests := []struct {
		name     string
		initial  *Command
		dir      string
		expected string
	}{
		{"set directory", &Command{}, "/path/to/dir", "/path/to/dir"},
		{"overwrite directory", &Command{Dir: "/old/path"}, "/new/path", "/new/path"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.initial.SetDir(tt.dir)
			if tt.initial.Dir != tt.expected {
				t.Errorf("unexpected dir: got %s, want %s", tt.initial.Dir, tt.expected)
			}
		})
	}
}

func TestCommand_UsePipe(t *testing.T) {
	cmd := &Command{}
	cmd.UsePipe()
	if !cmd.Pipe {
		t.Errorf("UsePipe did not set Pipe to true")
	}
}

func TestCommand_SetName(t *testing.T) {
	tests := []struct {
		name     string
		initial  *Command
		newName  string
		expected string
	}{
		{"set name", &Command{}, "TestCommand", "TestCommand"},
		{"overwrite name", &Command{Name: "OldName"}, "NewName", "NewName"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.initial.SetName(tt.newName)
			if tt.initial.Name != tt.expected {
				t.Errorf("unexpected name: got %s, want %s", tt.initial.Name, tt.expected)
			}
		})
	}
}

func TestCommand_GetNamePath(t *testing.T) {
	tests := []struct {
		name     string
		command  *Command
		expected string
	}{
		{"no parent", &Command{Name: "test"}, "test"},
		{"with parent", &Command{Name: "child", parent: &Command{Name: "parent"}}, "parent > child"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.command.GetNamePath()
			if result != tt.expected {
				t.Errorf("unexpected name path: got %s, want %s", result, tt.expected)
			}
		})
	}
}

func TestCommand_Validate(t *testing.T) {
	tests := []struct {
		name      string
		cmd       *Command
		wantError bool
	}{
		{"valid command", &Command{Cmd: "ls"}, false},
		{"empty command", &Command{Cmd: ""}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.cmd.Validate()
			if (err != nil) != tt.wantError {
				t.Errorf("unexpected result for Validate: got err = %v, wantError = %v", err, tt.wantError)
			}
		})
	}
}

func TestCommand_GetChainName(t *testing.T) {
	tests := []struct {
		name     string
		command  *Command
		expected string
	}{
		{"no parent", &Command{Name: "cmd"}, "cmd"},
		{"parent with chain name", &Command{Name: "child", parent: &Command{Name: "parent"}}, "parent"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.command.GetChainName()
			if result != tt.expected {
				t.Errorf("unexpected chain name: got %s, want %s", result, tt.expected)
			}
		})
	}
}

func TestCommand_GetChain(t *testing.T) {
	parentChain := &CommandChain{}
	tests := []struct {
		name     string
		command  *Command
		expected *CommandChain
	}{
		{"no chain", &Command{}, nil},
		{"with chain", &Command{parent: parentChain}, parentChain},
		{"parent with chain", &Command{parent: &Command{parent: parentChain}}, parentChain},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.command.GetChain()
			if result != tt.expected {
				t.Errorf("unexpected chain: got %v, want %v", result, tt.expected)
			}
		})
	}
}
