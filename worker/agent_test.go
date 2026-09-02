package worker

import (
	"agent-api/llm"
	"context"
	"encoding/json"
	"testing"
)

// scriptedChatter 按预设顺序返回模型回复，并记下是否收到 role=tool 的回填消息。
type scriptedChatter struct {
	turns         []*llm.AssistantTurn
	i             int
	gotToolResult bool
}

func (s *scriptedChatter) ChatWithTools(ctx context.Context, msgs []llm.Message, tools []llm.Tool) (*llm.AssistantTurn, error) {
	for _, m := range msgs {
		if m.Role == "tool" {
			s.gotToolResult = true
		}
	}
	if s.i >= len(s.turns) {
		return &llm.AssistantTurn{Content: "done"}, nil
	}
	t := s.turns[s.i]
	s.i++
	return t, nil
}

func TestRunAgent_ExecutesToolAndFeedsBack(t *testing.T) {
	specs, handlers := defaultTools()

	sc := &scriptedChatter{turns: []*llm.AssistantTurn{
		{ToolCalls: []llm.ToolCall{{
			ID:       "1",
			Type:     "function",
			Function: llm.ToolCallFn{Name: "calculate", Arguments: `{"expression":"2*(3+4)"}`},
		}}},
		{Content: "the answer is 14"},
	}}

	var executed string
	exec := func(tc llm.ToolCall) string {
		h := handlers[tc.Function.Name]
		out, _ := h(json.RawMessage(tc.Function.Arguments))
		executed = out
		return out
	}

	res, err := runAgent(context.Background(), sc, "what is 2*(3+4)?", specs, exec)
	if err != nil {
		t.Fatal(err)
	}
	if res != "the answer is 14" {
		t.Errorf("res = %q, want %q", res, "the answer is 14")
	}
	if executed != "14" {
		t.Errorf("tool executed result = %q, want %q", executed, "14")
	}
	if !sc.gotToolResult {
		t.Errorf("tool result was not fed back into the conversation")
	}
}

func TestEvalArithmetic(t *testing.T) {
	cases := map[string]float64{
		"2*(3+4)": 14,
		"1+2*3":   7,
		"(1+2)*3": 9,
		"10/4":    2.5,
		"-3+5":    2,
		"2.5*2":   5,
	}
	for expr, want := range cases {
		got, err := evalArithmetic(expr)
		if err != nil {
			t.Fatalf("%q: %v", expr, err)
		}
		if got != want {
			t.Errorf("%q = %v, want %v", expr, got, want)
		}
	}

	if _, err := evalArithmetic("1/0"); err == nil {
		t.Errorf("expected division-by-zero error")
	}
	if _, err := evalArithmetic("1+"); err == nil {
		t.Errorf("expected parse error for incomplete expression")
	}
}
