package parallel

import (
	"errors"
	"fmt"

	"github.com/efureev/reggol"
)

type CommandParent interface {
	GetNamePath() string
	GetChainName() string
}

var (
	ErrNoChains       = errors.New("flow must contain at least one chain")
	ErrEmptyChainName = errors.New("chain name cannot be empty")
	ErrEmptyChain     = errors.New("chain must contain at least one command")
)

type Flow struct {
	Chains []*CommandChain
}

func (f *Flow) AddChain(chain *CommandChain) {
	f.Chains = append(f.Chains, chain)
}

// Validate validates all chains and commands in the flow.
func (f *Flow) Validate() error {
	if len(f.Chains) == 0 {
		return ErrNoChains
	}

	for _, chain := range f.Chains {
		if chain.Name == "" {
			return ErrEmptyChainName
		}

		if len(chain.commands) == 0 {
			return fmt.Errorf("chain %q: %w", chain.Name, ErrEmptyChain)
		}

		for _, cmd := range chain.commands {
			if err := cmd.Validate(); err != nil {
				return fmt.Errorf("invalid command in chain %q: %w", chain.Name, err)
			}
		}
	}

	return nil
}

type CommandChain struct {
	Name     string
	commands []Command
	Color    reggol.TextStyle
}

func (cc *CommandChain) GetNamePath() string {
	return cc.Name
}

func (cc *CommandChain) GetChainName() string {
	return cc.Name
}

func (cc *CommandChain) Add(cmd Command) {
	cmd.parent = cc
	cc.commands = append(cc.commands, cmd)
}
