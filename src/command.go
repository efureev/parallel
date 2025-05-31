package parallel

import "fmt"

type Format struct {
	CmdName string
}

type Command struct {
	Name   string
	Cmd    string
	Args   []string
	Dir    string
	Pipe   bool
	Format Format
	parent CommandParent
}

func NewCommand(cmd string, args ...string) *Command {
	return &Command{Cmd: cmd, Args: args, Pipe: false}
}

func (cmd *Command) SetArguments(args ...string) *Command {
	cmd.Args = args

	return cmd
}

func (cmd *Command) GetNamePath() string {
	if cmd.parent == nil {
		return cmd.Name
	}

	return cmd.parent.GetNamePath() + ` > ` + cmd.Name
}

func (cmd *Command) GetChainName() string {
	if cmd.parent == nil {
		return cmd.Name
	}

	return cmd.parent.GetChainName()
}

func (cmd *Command) GetChain() *CommandChain {
	if chain, ok := cmd.parent.(*CommandChain); ok {
		return chain
	}

	if c, ok := cmd.parent.(*Command); ok {
		return c.GetChain()
	}

	return nil
}

func (cmd *Command) SetDir(dir string) *Command {
	cmd.Dir = dir

	return cmd
}

func (cmd *Command) UsePipe() *Command {
	cmd.Pipe = true

	return cmd
}

func (cmd *Command) SetName(name string) *Command {
	cmd.Name = name

	return cmd
}

func (cmd *Command) getName() string {
	if cmd.Name != `` {
		return cmd.Name
	}

	return cmd.Cmd
}

// Validate checks if the command is properly configured
func (cmd *Command) Validate() error {
	if cmd.Cmd == "" {
		return fmt.Errorf("command cannot be empty")
	}
	return nil
}
