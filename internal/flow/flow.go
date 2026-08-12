// Package flow содержит доменную модель: цепочки команд, сами команды и их валидацию.
package flow

import (
	"errors"
	"fmt"

	"github.com/efureev/reggol"
)

var (
	ErrNoChains       = errors.New("flow must contain at least one chain")
	ErrEmptyChainName = errors.New("chain name cannot be empty")
	ErrEmptyChain     = errors.New("chain must contain at least one command")
)

// Flow — разобранная конфигурация: набор цепочек, выполняемых параллельно.
type Flow struct {
	Chains []*CommandChain
}

func (f *Flow) AddChain(chain *CommandChain) {
	f.Chains = append(f.Chains, chain)
}

// Validate проверяет все цепочки и команды.
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

// CommandChain — именованная последовательность команд.
//
// TODO(2.3): поле Color — тип библиотеки логирования в доменной структуре.
// Оно уезжает в internal/ui вместе с палитрой, после чего пакет flow перестанет
// зависеть от reggol. См. docs/UPGRADE-SPEC.md, задача 2.3.
type CommandChain struct {
	Name     string
	commands []Command
	Color    reggol.TextStyle
}

func (cc *CommandChain) GetChainName() string {
	return cc.Name
}

// Commands возвращает команды цепочки в порядке их объявления в конфигурации.
func (cc *CommandChain) Commands() []Command {
	return cc.commands
}

func (cc *CommandChain) Add(cmd Command) {
	cc.commands = append(cc.commands, cmd)
}
