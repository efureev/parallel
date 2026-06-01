package parallel

import "testing"

// TestFlowBuilder_CommandParentPointsToStoredChain гарантирует, что после сборки Flow
// поле parent каждой команды указывает ровно на тот объект цепочки, который хранится
// в Flow.Chains (а не на «протёкшую» копию). Это защищает от латентного бага,
// при котором мутация цепочки в слайсе расходилась бы с тем, что видят команды.
func TestFlowBuilder_CommandParentPointsToStoredChain(t *testing.T) {
	b := NewFlowBuilder(Logger())

	data := ConfigData{
		Chains: []ChainConfig{
			{
				Name: "first",
				Commands: []NamedCommand{
					{Name: "a", Spec: command{Cmd: []string{"echo", "a"}}},
					{Name: "b", Spec: command{Cmd: []string{"echo", "b"}}},
				},
			},
			{
				Name: "second",
				Commands: []NamedCommand{
					{Name: "c", Spec: command{Cmd: []string{"echo", "c"}}},
				},
			},
		},
	}

	flow := b.Build(data)

	if len(flow.Chains) != 2 {
		t.Fatalf("expected 2 chains, got %d", len(flow.Chains))
	}

	for _, chain := range flow.Chains {
		for i := range chain.commands {
			got := chain.commands[i].GetChain()
			if got != chain {
				t.Fatalf("command %q parent points to %p, want stored chain %p",
					chain.commands[i].Name, got, chain)
			}

			if got.Name != chain.Name || got.Color != chain.Color {
				t.Fatalf("command %q parent chain mismatch: got %+v", chain.commands[i].Name, got)
			}
		}
	}
}
