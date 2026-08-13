// Package flow содержит доменную модель: цепочки команд, сами команды и их валидацию.
//
// Пакет не зависит ни от одного другого слоя приложения и ни от одной внешней
// библиотеки — это проверяется линтером depguard. Всё, что относится к показу
// (цвета, форматирование, палитра), живёт в internal/ui.
//
// Семантика значений: Flow владеет цепочками по указателю, потому что цепочка —
// изменяемая сущность с накапливаемым списком команд. Сама Command передаётся
// и хранится по значению: она неизменяема после сборки, и копия безопасна.
// Обратных ссылок «команда → цепочка» нет намеренно — контекст цепочки
// передаётся вызывающим явным параметром.
package flow

import (
	"errors"
	"fmt"
)

var (
	ErrNoChains       = errors.New("flow must contain at least one chain")
	ErrEmptyChainName = errors.New("chain name cannot be empty")
	ErrEmptyChain     = errors.New("chain must contain at least one command")

	// ErrUnknownRestartPolicy — значение поля restart вне перечисления.
	ErrUnknownRestartPolicy = errors.New("unknown restart policy")
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
// ColorIdx — порядковый номер цепочки, по которому слой представления выбирает
// цвет из своей палитры. Домен намеренно не знает, что это за цвет и есть ли он
// вообще: при выводе в файл раскраски не будет, а номер останется прежним.
type CommandChain struct {
	Name     string
	commands []Command
	ColorIdx int
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
