package flow

import (
	"errors"
	"testing"
)

const testChainName = "TestChain"

func TestFlow_AddChain(t *testing.T) {
	t.Run("Add single chain", func(t *testing.T) {
		f := &Flow{}
		chain := &CommandChain{Name: testChainName}
		f.AddChain(chain)

		if len(f.Chains) != 1 {
			t.Errorf("expected 1 chain, got %d", len(f.Chains))
		}
		if f.Chains[0].Name != testChainName {
			t.Errorf("expected chain name '%s', got %s", testChainName, f.Chains[0].Name)
		}
	})
}

func TestFlow_Validate(t *testing.T) {
	tests := []struct {
		name        string
		setupFlow   func() *Flow
		expectErr   bool
		expectedErr error
	}{
		{
			name: "Valid flow with chains",
			setupFlow: func() *Flow {
				f := &Flow{}
				chain := &CommandChain{Name: "ValidChain"}
				chain.Add(Command{Cmd: "echo"})
				f.AddChain(chain)

				return f
			},
			expectErr:   false,
			expectedErr: nil,
		},
		{
			name: "Invalid flow without chains",
			setupFlow: func() *Flow {
				return &Flow{}
			},
			expectErr:   true,
			expectedErr: ErrNoChains,
		},
		{
			name: "Chain without name",
			setupFlow: func() *Flow {
				f := &Flow{}
				chain := &CommandChain{}
				chain.Add(Command{Cmd: "echo"})
				f.AddChain(chain)

				return f
			},
			expectErr:   true,
			expectedErr: ErrEmptyChainName,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := tt.setupFlow()
			err := f.Validate()

			if (err != nil) != tt.expectErr {
				t.Errorf("expected error: %v, got: %v", tt.expectErr, err)
			}
			if err != nil && !errors.Is(err, tt.expectedErr) {
				t.Errorf("expected error: %v, got: %v", tt.expectedErr, err)
			}
		})
	}
}

func TestFlow_ValidateEmptyChain(t *testing.T) {
	f := &Flow{Chains: []*CommandChain{{Name: "empty"}}}

	err := f.Validate()
	if !errors.Is(err, ErrEmptyChain) {
		t.Fatalf("expected ErrEmptyChain, got %v", err)
	}
}

func TestFlow_ValidateInvalidCommand(t *testing.T) {
	chain := &CommandChain{Name: "c"}
	chain.Add(Command{Cmd: ""})

	f := &Flow{Chains: []*CommandChain{chain}}

	if err := f.Validate(); !errors.Is(err, ErrEmptyCommand) {
		t.Fatalf("expected ErrEmptyCommand, got %v", err)
	}
}

func TestCommandChain_GetChainName(t *testing.T) {
	cc := CommandChain{Name: testChainName}
	if cc.GetChainName() != testChainName {
		t.Errorf("expected chain name '%s', got %s", testChainName, cc.GetChainName())
	}
}

func TestCommandChain_Add(t *testing.T) {
	cc := &CommandChain{Name: testChainName}
	cc.Add(Command{Name: "TestCommand"})

	commands := cc.Commands()
	if len(commands) != 1 {
		t.Fatalf("expected 1 command, got %d", len(commands))
	}

	if commands[0].Name != "TestCommand" {
		t.Errorf("expected command name 'TestCommand', got %s", commands[0].Name)
	}
}

// TestCommandChain_CommandsPreservesOrder фиксирует, что порядок команд в цепочке
// повторяет порядок добавления: на нём держится детерминированность вывода.
func TestCommandChain_CommandsPreservesOrder(t *testing.T) {
	cc := &CommandChain{Name: testChainName}
	want := []string{"first", "second", "third"}

	for _, name := range want {
		cc.Add(Command{Name: name, Cmd: "echo"})
	}

	got := cc.Commands()
	if len(got) != len(want) {
		t.Fatalf("expected %d commands, got %d", len(want), len(got))
	}

	for i, name := range want {
		if got[i].Name != name {
			t.Errorf("command[%d] = %q, want %q", i, got[i].Name, name)
		}
	}
}
