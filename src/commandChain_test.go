package parallel

import (
	"errors"
	"testing"
)

const testChainName = "TestChain"

func TestFlow_AddChain(t *testing.T) {
	t.Run("Add single chain", func(t *testing.T) {
		f := &Flow{}
		chain := CommandChain{Name: testChainName}
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
				chain := CommandChain{Name: "ValidChain"}
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
			expectedErr: errors.New("flow must contain at least one chain"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := tt.setupFlow()
			err := f.Validate()

			if (err != nil) != tt.expectErr {
				t.Errorf("expected error: %v, got: %v", tt.expectErr, err)
			}
			if err != nil && err.Error() != tt.expectedErr.Error() {
				t.Errorf("expected error message: %v, got: %v", tt.expectedErr, err)
			}
		})
	}
}

func TestCommandChain_GetNamePath(t *testing.T) {
	t.Run("Get name path of chain", func(t *testing.T) {
		cc := CommandChain{Name: testChainName}
		if cc.GetNamePath() != testChainName {
			t.Errorf("expected name path '%s', got %s", testChainName, cc.GetNamePath())
		}
	})
}

func TestCommandChain_GetChainName(t *testing.T) {
	t.Run("Get chain name", func(t *testing.T) {
		cc := CommandChain{Name: testChainName}
		if cc.GetChainName() != testChainName {
			t.Errorf("expected chain name '%s', got %s", testChainName, cc.GetChainName())
		}
	})
}

func TestCommandChain_Add(t *testing.T) {
	t.Run("Add single command to chain", func(t *testing.T) {
		cc := &CommandChain{Name: "TestChain"}
		cmd := Command{Name: "TestCommand"}
		cc.Add(cmd)

		if len(cc.commands) != 1 {
			t.Errorf("expected 1 command, got %d", len(cc.commands))
		}
		if cc.commands[0].Name != "TestCommand" {
			t.Errorf("expected command name 'TestCommand', got %s", cc.commands[0].Name)
		}
		if cc.commands[0].parent != cc {
			t.Errorf("expected command parent to be the chain")
		}
	})
}
