package parallel

import (
	"fmt"
	"github.com/efureev/reggol"
)

type CommandParent interface {
	GetNamePath() string
	GetChainName() string
}

type Flow struct {
	Chains []CommandChain
}

func (f *Flow) AddChain(chain CommandChain) {
	f.Chains = append(f.Chains, chain)
}

// Validate validates all chains in the flow
func (f *Flow) Validate() error {
	if len(f.Chains) == 0 {
		return fmt.Errorf("flow must contain at least one chain")
	}
	return nil
}

type CommandChains map[string]CommandChains

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
