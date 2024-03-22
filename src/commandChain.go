package parallel

import "github.com/efureev/reggol"

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
