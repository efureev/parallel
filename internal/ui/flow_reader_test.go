package ui

import (
	"testing"

	"github.com/efureev/parallel/internal/flow"
)

func TestFlowReader_Out(t *testing.T) {
	reader := NewFlowReader(NewDiscardLogger())

	// nil и пустой Flow не должны приводить к панике.
	reader.Out(nil)
	reader.Out(&flow.Flow{})

	chain := &flow.CommandChain{Name: "c1"}
	chain.Add(flow.Command{
		Name:   "a",
		Cmd:    "echo",
		Args:   []string{"a"},
		Dir:    "/tmp",
		Pipe:   true,
		Format: flow.Format{CmdName: "%CMD_NAME%"},
	})
	chain.Add(flow.Command{Name: "b", Cmd: "echo", Args: []string{"b"}, Disable: true})

	result := &flow.Flow{}
	result.AddChain(chain)
	result.AddChain(&flow.CommandChain{Name: "empty"})

	reader.Out(result)
}
