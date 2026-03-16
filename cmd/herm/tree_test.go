package main

import (
	"encoding/json"
	"strings"
	"testing"

	"langdag.com/langdag/types"
)

func TestRenderTree_LinearConversation(t *testing.T) {
	a := &App{
		models: []ModelDef{
			{ID: "claude-sonnet-4-20250514", Provider: ProviderAnthropic, PromptPrice: 3, CompletionPrice: 15},
		},
	}

	nodes := []*types.Node{
		{ID: "1", NodeType: types.NodeTypeUser, Content: "Hello"},
		{ID: "2", ParentID: "1", NodeType: types.NodeTypeAssistant, Content: "Hi there!", Model: "claude-sonnet-4-20250514", TokensIn: 100, TokensOut: 50},
		{ID: "3", ParentID: "2", NodeType: types.NodeTypeUser, Content: "Help me fix a bug"},
		{ID: "4", ParentID: "3", NodeType: types.NodeTypeAssistant, Content: "Sure, let me look.", Model: "claude-sonnet-4-20250514", TokensIn: 500, TokensOut: 200},
	}

	result := a.renderTree(nodes)

	// All lines should start at column 0 (no indentation).
	for _, line := range strings.Split(strings.TrimRight(result, "\n"), "\n") {
		if line == "" || strings.HasPrefix(line, "Total:") {
			continue
		}
		if strings.HasPrefix(line, " ") || strings.HasPrefix(line, "│") || strings.HasPrefix(line, "├") || strings.HasPrefix(line, "└") {
			t.Errorf("linear conversation should be flat, got indented line: %q", line)
		}
	}
	if !strings.Contains(result, "Hello") {
		t.Errorf("expected 'Hello', got:\n%s", result)
	}
	if !strings.Contains(result, "Help me fix a bug") {
		t.Errorf("expected 'Help me fix a bug', got:\n%s", result)
	}
	if !strings.Contains(result, "Total:") {
		t.Errorf("expected total cost, got:\n%s", result)
	}
}

func TestRenderTree_WithToolCalls(t *testing.T) {
	a := &App{}

	nodes := []*types.Node{
		{ID: "1", NodeType: types.NodeTypeUser, Content: "Run ls"},
		{ID: "2", ParentID: "1", NodeType: types.NodeTypeAssistant, Content: "Let me run that."},
		{ID: "3", ParentID: "2", NodeType: types.NodeTypeToolCall, Content: `{"name":"bash","input":{"command":"ls"}}`},
		{ID: "4", ParentID: "3", NodeType: types.NodeTypeToolResult, Content: `file1.go\nfile2.go`},
		{ID: "5", ParentID: "4", NodeType: types.NodeTypeAssistant, Content: "Here are the files."},
	}

	result := a.renderTree(nodes)
	lines := strings.Split(strings.TrimRight(result, "\n"), "\n")

	// Tool call and result lines should be indented.
	foundIndentedTool := false
	for _, line := range lines {
		if strings.Contains(line, "bash") && strings.HasPrefix(line, "  ") {
			foundIndentedTool = true
		}
	}
	if !foundIndentedTool {
		t.Errorf("expected tool call line to be indented, got:\n%s", result)
	}

	// User/Assistant lines should NOT be indented.
	for _, line := range lines {
		if strings.Contains(line, "Run ls") && strings.HasPrefix(line, " ") {
			t.Errorf("user line should not be indented: %q", line)
		}
		if strings.Contains(line, "Here are the files") && strings.HasPrefix(line, " ") {
			t.Errorf("continuation assistant line should not be indented: %q", line)
		}
	}
}

func TestRenderTree_ToolResultAsUserNode(t *testing.T) {
	a := &App{}

	// Simulates the real langdag storage where tool results are stored as
	// user nodes via PromptFrom, and assistant tool_use is stored as JSON.
	nodes := []*types.Node{
		{ID: "1", NodeType: types.NodeTypeUser, Content: "Run hello world"},
		{ID: "2", ParentID: "1", NodeType: types.NodeTypeAssistant,
			Content: `[{"type":"tool_use","id":"call_1","name":"bash","input":{"command":"go run main.go"}},{"type":"text","text":"Let me run that."}]`},
		{ID: "3", ParentID: "2", NodeType: types.NodeTypeUser,
			Content: `[{"type":"tool_result","tool_use_id":"call_1","content":"Hello, World!"}]`},
		{ID: "4", ParentID: "3", NodeType: types.NodeTypeAssistant, Content: "Done!"},
	}

	result := a.renderTree(nodes)

	// Tool result user node should NOT show as "You:".
	if strings.Contains(result, "You: [{") {
		t.Errorf("tool_result user node should not display raw JSON, got:\n%s", result)
	}

	// Should show the tool name from the assistant node.
	if !strings.Contains(result, "bash") {
		t.Errorf("expected tool name 'bash' in tree, got:\n%s", result)
	}

	// Tool result should show as "✓ bash" (tool name, not output), indented.
	foundToolResult := false
	for _, line := range strings.Split(strings.TrimRight(result, "\n"), "\n") {
		if strings.Contains(line, "✓") && strings.Contains(line, "bash") {
			foundToolResult = true
			if !strings.HasPrefix(line, "  ") {
				t.Errorf("tool result line should be indented: %q", line)
			}
		}
	}
	if !foundToolResult {
		t.Errorf("expected '✓ bash' tool result line, got:\n%s", result)
	}
	// Should NOT show tool output content in tree.
	if strings.Contains(result, "Hello, World!") {
		t.Errorf("tree should not show tool output, got:\n%s", result)
	}

	// The actual user message should still show as "You:" at column 0.
	for _, line := range strings.Split(strings.TrimRight(result, "\n"), "\n") {
		if strings.Contains(line, "You:") && strings.HasPrefix(line, " ") {
			t.Errorf("user line should not be indented: %q", line)
		}
	}

	// Assistant lines should be at column 0.
	for _, line := range strings.Split(strings.TrimRight(result, "\n"), "\n") {
		if strings.Contains(line, "Assistant") && strings.HasPrefix(line, " ") {
			t.Errorf("assistant line should not be indented: %q", line)
		}
		if strings.Contains(line, "Done!") && strings.HasPrefix(line, " ") {
			t.Errorf("assistant line should not be indented: %q", line)
		}
	}
}

func TestRenderTree_ToolResultTokenEstimate(t *testing.T) {
	a := &App{}

	// Create a tool result with known content size.
	// 400 chars / 4 chars-per-token = ~100 tokens.
	resultContent := strings.Repeat("x", 400)
	nodes := []*types.Node{
		{ID: "1", NodeType: types.NodeTypeUser, Content: "Do something"},
		{ID: "2", ParentID: "1", NodeType: types.NodeTypeAssistant,
			Content: `[{"type":"tool_use","id":"call_1","name":"bash","input":{}}]`},
		{ID: "3", ParentID: "2", NodeType: types.NodeTypeUser,
			Content: `[{"type":"tool_result","tool_use_id":"call_1","content":"` + resultContent + `"}]`},
		{ID: "4", ParentID: "3", NodeType: types.NodeTypeAssistant, Content: "Done."},
	}

	result := a.renderTree(nodes)

	// Should include a token estimate annotation.
	if !strings.Contains(result, "tokens") {
		t.Errorf("expected token estimate in tree, got:\n%s", result)
	}
}

func TestRenderTree_ToolResultError(t *testing.T) {
	a := &App{}

	nodes := []*types.Node{
		{ID: "1", NodeType: types.NodeTypeUser, Content: "Do something"},
		{ID: "2", ParentID: "1", NodeType: types.NodeTypeAssistant,
			Content: `[{"type":"tool_use","id":"call_1","name":"bash","input":{}}]`},
		{ID: "3", ParentID: "2", NodeType: types.NodeTypeUser,
			Content: `[{"type":"tool_result","tool_use_id":"call_1","content":"command not found","is_error":true}]`},
		{ID: "4", ParentID: "3", NodeType: types.NodeTypeAssistant, Content: "That failed."},
	}

	result := a.renderTree(nodes)

	// Should show error marker (✗) not success marker (✓).
	if !strings.Contains(result, "✗") {
		t.Errorf("expected error marker ✗ for failed tool result, got:\n%s", result)
	}
}

func TestExtractToolName(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{`{"name":"bash","input":{"command":"ls"}}`, "bash"},
		{`{"name":"git","input":{"args":"status"}}`, "git"},
		{`no json here`, ""},
		{`{}`, ""},
	}
	for _, tt := range tests {
		got := extractToolName(tt.input)
		if got != tt.want {
			t.Errorf("extractToolName(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestShortModel(t *testing.T) {
	tests := []struct {
		input, want string
	}{
		{"claude-sonnet-4-20250514", "claude-sonnet-4-20250514"},
		{"anthropic/claude-sonnet-4-20250514", "claude-sonnet-4-20250514"},
		{"openai/gpt-4o", "gpt-4o"},
	}
	for _, tt := range tests {
		got := shortModel(tt.input)
		if got != tt.want {
			t.Errorf("shortModel(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestRebuildChatMessages_IncludesToolCalls(t *testing.T) {
	a := &App{}

	nodes := []*types.Node{
		{ID: "1", NodeType: types.NodeTypeUser, Content: "Run ls"},
		{ID: "2", ParentID: "1", NodeType: types.NodeTypeAssistant,
			Content: `[{"type":"text","text":"Let me run that."},{"type":"tool_use","id":"call_1","name":"bash","input":{"command":"ls -la"}}]`},
		{ID: "3", ParentID: "2", NodeType: types.NodeTypeUser,
			Content: `[{"type":"tool_result","tool_use_id":"call_1","content":"file1.go\nfile2.go"}]`},
		{ID: "4", ParentID: "3", NodeType: types.NodeTypeAssistant, Content: "Here are the files."},
	}

	msgs := a.rebuildChatMessages(nodes)

	// Should have: user, assistant text, toolCall, toolResult, assistant text
	var kinds []chatMsgKind
	for _, m := range msgs {
		kinds = append(kinds, m.kind)
	}
	want := []chatMsgKind{msgUser, msgAssistant, msgToolCall, msgToolResult, msgAssistant}
	if len(kinds) != len(want) {
		t.Fatalf("got %d messages %v, want %d %v", len(kinds), kinds, len(want), want)
	}
	for i := range want {
		if kinds[i] != want[i] {
			t.Errorf("msg[%d].kind = %v, want %v", i, kinds[i], want[i])
		}
	}

	// Tool call should contain the bash command summary.
	for _, m := range msgs {
		if m.kind == msgToolCall {
			if !strings.Contains(m.content, "ls -la") {
				t.Errorf("tool call content = %q, want it to contain 'ls -la'", m.content)
			}
		}
	}

	// Tool result should contain the collapsed output.
	for _, m := range msgs {
		if m.kind == msgToolResult {
			if !strings.Contains(m.content, "file1.go") {
				t.Errorf("tool result content = %q, want it to contain 'file1.go'", m.content)
			}
		}
	}
}

func TestRebuildChatMessages_ToolResultError(t *testing.T) {
	a := &App{}

	nodes := []*types.Node{
		{ID: "1", NodeType: types.NodeTypeUser, Content: "Do something"},
		{ID: "2", ParentID: "1", NodeType: types.NodeTypeAssistant,
			Content: `[{"type":"tool_use","id":"call_1","name":"bash","input":{"command":"bad-cmd"}}]`},
		{ID: "3", ParentID: "2", NodeType: types.NodeTypeUser,
			Content: `[{"type":"tool_result","tool_use_id":"call_1","content":"command not found","is_error":true}]`},
		{ID: "4", ParentID: "3", NodeType: types.NodeTypeAssistant, Content: "That failed."},
	}

	msgs := a.rebuildChatMessages(nodes)

	for _, m := range msgs {
		if m.kind == msgToolResult {
			if !m.isError {
				t.Errorf("expected tool result to be marked as error")
			}
			return
		}
	}
	t.Errorf("no tool result found in messages")
}

func TestRebuildChatMessages_OldFormatToolNodes(t *testing.T) {
	a := &App{}

	nodes := []*types.Node{
		{ID: "1", NodeType: types.NodeTypeUser, Content: "Run ls"},
		{ID: "2", ParentID: "1", NodeType: types.NodeTypeAssistant, Content: "Let me run that."},
		{ID: "3", ParentID: "2", NodeType: types.NodeTypeToolCall, Content: `{"name":"bash","input":{"command":"ls"}}`},
		{ID: "4", ParentID: "3", NodeType: types.NodeTypeToolResult, Content: "file1.go\nfile2.go"},
		{ID: "5", ParentID: "4", NodeType: types.NodeTypeAssistant, Content: "Here are the files."},
	}

	msgs := a.rebuildChatMessages(nodes)

	var kinds []chatMsgKind
	for _, m := range msgs {
		kinds = append(kinds, m.kind)
	}
	want := []chatMsgKind{msgUser, msgAssistant, msgToolCall, msgToolResult, msgAssistant}
	if len(kinds) != len(want) {
		t.Fatalf("got %d messages %v, want %d %v", len(kinds), kinds, len(want), want)
	}
	for i := range want {
		if kinds[i] != want[i] {
			t.Errorf("msg[%d].kind = %v, want %v", i, kinds[i], want[i])
		}
	}
}

func TestTruncate(t *testing.T) {
	if got := truncate("short", 10); got != "short" {
		t.Errorf("truncate short = %q", got)
	}
	got := truncate("this is a very long string", 10)
	// Should be 9 chars of original + "…"
	if got != "this is a…" {
		t.Errorf("truncate long = %q, want %q", got, "this is a…")
	}
}

// ─── 3a: extractAssistantText ───

func TestExtractAssistantText_PlainText(t *testing.T) {
	got := extractAssistantText("Hello world")
	if got != "Hello world" {
		t.Errorf("plain text: got %q, want %q", got, "Hello world")
	}
}

func TestExtractAssistantText_Empty(t *testing.T) {
	if got := extractAssistantText(""); got != "" {
		t.Errorf("empty: got %q", got)
	}
	if got := extractAssistantText("   "); got != "" {
		t.Errorf("whitespace: got %q", got)
	}
}

func TestExtractAssistantText_JSONTextBlocks(t *testing.T) {
	blocks, _ := json.Marshal([]types.ContentBlock{
		{Type: "text", Text: "First paragraph."},
		{Type: "text", Text: "Second paragraph."},
	})
	got := extractAssistantText(string(blocks))
	if got != "First paragraph.\nSecond paragraph." {
		t.Errorf("got %q, want joined text blocks", got)
	}
}

func TestExtractAssistantText_ToolUseOnly(t *testing.T) {
	blocks, _ := json.Marshal([]types.ContentBlock{
		{Type: "tool_use", ID: "call_1", Name: "bash", Input: json.RawMessage(`{"command":"ls"}`)},
	})
	got := extractAssistantText(string(blocks))
	if got != "" {
		t.Errorf("tool_use only: got %q, want empty", got)
	}
}

func TestExtractAssistantText_MixedContent(t *testing.T) {
	blocks, _ := json.Marshal([]types.ContentBlock{
		{Type: "text", Text: "Let me check."},
		{Type: "tool_use", ID: "call_1", Name: "bash", Input: json.RawMessage(`{}`)},
		{Type: "text", Text: "Done now."},
	})
	got := extractAssistantText(string(blocks))
	if got != "Let me check.\nDone now." {
		t.Errorf("mixed: got %q", got)
	}
}

func TestExtractAssistantText_InvalidJSON(t *testing.T) {
	// Starts with '[' but isn't valid JSON → should return "".
	got := extractAssistantText("[not valid json")
	if got != "" {
		t.Errorf("invalid JSON: got %q, want empty", got)
	}
}

func TestExtractAssistantText_EmptyTextBlocks(t *testing.T) {
	blocks, _ := json.Marshal([]types.ContentBlock{
		{Type: "text", Text: ""},
		{Type: "text", Text: "Real content"},
		{Type: "text", Text: ""},
	})
	got := extractAssistantText(string(blocks))
	if got != "Real content" {
		t.Errorf("empty text blocks: got %q, want %q", got, "Real content")
	}
}

// ─── 3b: parseAssistantContent ───

func TestParseAssistantContent_PlainText(t *testing.T) {
	preview, tools := parseAssistantContent("Hello, how can I help?")
	if preview != "Hello, how can I help?" {
		t.Errorf("preview = %q", preview)
	}
	if len(tools) != 0 {
		t.Errorf("tools = %v, want empty", tools)
	}
}

func TestParseAssistantContent_MultilinePlainText(t *testing.T) {
	preview, _ := parseAssistantContent("Line one\nLine two\nLine three")
	if preview != "Line one" {
		t.Errorf("preview = %q, want first line only", preview)
	}
}

func TestParseAssistantContent_Empty(t *testing.T) {
	preview, tools := parseAssistantContent("")
	if preview != "" || tools != nil {
		t.Errorf("empty: preview=%q tools=%v", preview, tools)
	}
}

func TestParseAssistantContent_TextAndToolUse(t *testing.T) {
	blocks, _ := json.Marshal([]types.ContentBlock{
		{Type: "text", Text: "Let me run that command."},
		{Type: "tool_use", ID: "call_abc", Name: "bash", Input: json.RawMessage(`{"command":"ls"}`)},
	})
	preview, tools := parseAssistantContent(string(blocks))
	if preview != "Let me run that command." {
		t.Errorf("preview = %q", preview)
	}
	if len(tools) != 1 {
		t.Fatalf("tools count = %d, want 1", len(tools))
	}
	if tools[0].id != "call_abc" || tools[0].name != "bash" {
		t.Errorf("tool = %+v", tools[0])
	}
}

func TestParseAssistantContent_MultipleToolUses(t *testing.T) {
	blocks, _ := json.Marshal([]types.ContentBlock{
		{Type: "tool_use", ID: "c1", Name: "bash", Input: json.RawMessage(`{}`)},
		{Type: "tool_use", ID: "c2", Name: "git", Input: json.RawMessage(`{}`)},
	})
	_, tools := parseAssistantContent(string(blocks))
	if len(tools) != 2 {
		t.Fatalf("tools count = %d, want 2", len(tools))
	}
	if tools[0].name != "bash" || tools[1].name != "git" {
		t.Errorf("tools = %+v %+v", tools[0], tools[1])
	}
}

func TestParseAssistantContent_ToolUseNoName(t *testing.T) {
	blocks, _ := json.Marshal([]types.ContentBlock{
		{Type: "tool_use", ID: "c1", Input: json.RawMessage(`{}`)},
	})
	_, tools := parseAssistantContent(string(blocks))
	if len(tools) != 1 || tools[0].name != "tool" {
		t.Errorf("unnamed tool_use should default to 'tool', got %+v", tools)
	}
}

func TestParseAssistantContent_InvalidJSON(t *testing.T) {
	preview, tools := parseAssistantContent("[{broken")
	if preview != "" || tools != nil {
		t.Errorf("invalid JSON: preview=%q tools=%v", preview, tools)
	}
}

// ─── 3c: parseToolResults and isToolResultContent ───

func TestIsToolResultContent_True(t *testing.T) {
	content := `[{"type":"tool_result","tool_use_id":"c1","content":"ok"}]`
	if !isToolResultContent(content) {
		t.Error("should detect tool_result content")
	}
}

func TestIsToolResultContent_WithWhitespace(t *testing.T) {
	content := `  [{"type":"tool_result","tool_use_id":"c1","content":"ok"}]  `
	if !isToolResultContent(content) {
		t.Error("should handle leading/trailing whitespace")
	}
}

func TestIsToolResultContent_False_PlainText(t *testing.T) {
	if isToolResultContent("Hello world") {
		t.Error("plain text should not be tool result content")
	}
}

func TestIsToolResultContent_False_OtherJSON(t *testing.T) {
	content := `[{"type":"text","text":"hello"}]`
	if isToolResultContent(content) {
		t.Error("text blocks should not be tool result content")
	}
}

func TestIsToolResultContent_False_Empty(t *testing.T) {
	if isToolResultContent("") {
		t.Error("empty should return false")
	}
	if isToolResultContent("   ") {
		t.Error("whitespace should return false")
	}
}

func TestParseToolResults_Single(t *testing.T) {
	content := toolResultContent("call_1", "file1.go")
	results := parseToolResults(content)
	if len(results) != 1 {
		t.Fatalf("got %d results, want 1", len(results))
	}
	if results[0].toolUseID != "call_1" {
		t.Errorf("toolUseID = %q", results[0].toolUseID)
	}
	if results[0].isError {
		t.Error("should not be error")
	}
}

func TestParseToolResults_Multiple(t *testing.T) {
	blocks, _ := json.Marshal([]types.ContentBlock{
		{Type: "tool_result", ToolUseID: "c1", Content: "ok"},
		{Type: "tool_result", ToolUseID: "c2", Content: "fail", IsError: true},
	})
	results := parseToolResults(string(blocks))
	if len(results) != 2 {
		t.Fatalf("got %d results, want 2", len(results))
	}
	if results[0].toolUseID != "c1" || results[0].isError {
		t.Errorf("result[0] = %+v", results[0])
	}
	if results[1].toolUseID != "c2" || !results[1].isError {
		t.Errorf("result[1] = %+v", results[1])
	}
}

func TestParseToolResults_MixedBlockTypes(t *testing.T) {
	blocks, _ := json.Marshal([]types.ContentBlock{
		{Type: "text", Text: "some text"},
		{Type: "tool_result", ToolUseID: "c1", Content: "ok"},
	})
	results := parseToolResults(string(blocks))
	if len(results) != 1 {
		t.Fatalf("should only return tool_result blocks, got %d", len(results))
	}
}

func TestParseToolResults_InvalidJSON(t *testing.T) {
	results := parseToolResults("not json")
	if results != nil {
		t.Errorf("invalid JSON should return nil, got %v", results)
	}
}

func TestParseToolResults_EmptyArray(t *testing.T) {
	results := parseToolResults("[]")
	if len(results) != 0 {
		t.Errorf("empty array should return empty, got %d", len(results))
	}
}
