package harness

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestHarnessLifecycle(t *testing.T) {
	tempDir := filepath.Join(os.TempDir(), "test_crush_harness")
	_ = os.RemoveAll(tempDir)
	defer os.RemoveAll(tempDir)

	h, err := InitOrLoad(tempDir)
	if err != nil {
		t.Fatalf("InitOrLoad failed: %v", err)
	}

	if len(h.Rules) == 0 {
		t.Errorf("expected default rules")
	}
	if len(h.Experiences) == 0 {
		t.Errorf("expected default experience")
	}

	// Test augment prompt
	base := "You are Crush coder."
	augmented := h.AugmentSystemPrompt(base)
	if !strings.Contains(augmented, "You are Crush coder.") {
		t.Errorf("expected base prompt preserved")
	}
	if !strings.Contains(augmented, "<engineering_rules>") {
		t.Errorf("expected engineering_rules block")
	}
	if !strings.Contains(augmented, "<continual_learning_memory>") {
		t.Errorf("expected continual_learning_memory block")
	}

	// Add an experience
	err = h.AddExperience(ExperienceEntry{
		Cue:        "遇到编译错误时",
		Lesson:     "检查缺失的包导入并再次运行 go test",
		Evidence:   "go build failed with undefined: foo",
		Confidence: "high",
	})
	if err != nil {
		t.Fatalf("AddExperience failed: %v", err)
	}

	if len(h.Experiences) != 2 {
		t.Errorf("expected 2 experiences, got %d", len(h.Experiences))
	}

	// Verify persistence
	data, err := os.ReadFile(filepath.Join(tempDir, "EXPERIENCE.md"))
	if err != nil || !strings.Contains(string(data), "遇到编译错误时") {
		t.Errorf("EXPERIENCE.md missing newly added lesson")
	}

	// Test reload
	h2, err := InitOrLoad(tempDir)
	if err != nil {
		t.Fatalf("InitOrLoad reload failed: %v", err)
	}
	if len(h2.Experiences) != 2 {
		t.Errorf("expected 2 experiences after reload, got %d", len(h2.Experiences))
	}
}

func TestShouldReflect(t *testing.T) {
	e := &Engine{}

	// Case 1: Casual talk -> false
	turn1 := Turn{
		UserMessage:    "你好，今天天气不错",
		AssistantReply: "是的，祝你有美好的一天！",
	}
	should, _ := e.ShouldReflect(turn1)
	if should {
		t.Errorf("expected ShouldReflect=false")
	}

	// Case 2: User correction -> true
	turn2 := Turn{
		UserMessage:    "不对，不要用 python，必须用纯 go",
		AssistantReply: "好的，已改用 Go 实现。",
	}
	should, reason := e.ShouldReflect(turn2)
	if !should || !strings.Contains(reason, "纠偏") {
		t.Errorf("expected ShouldReflect=true for user correction, got reason: %s", reason)
	}

	// Case 3: Tool error -> true
	turn3 := Turn{
		UserMessage: "查看配置",
		Tools: []ToolRecord{
			{Name: "read_file", Args: "config.yaml", Output: "error: file not found", Error: "not found"},
		},
	}
	should, reason = e.ShouldReflect(turn3)
	if !should || !strings.Contains(reason, "工具调用") {
		t.Errorf("expected ShouldReflect=true for tool error, got reason: %s", reason)
	}
}

func TestHandleExplicitCommand(t *testing.T) {
	tempDir := filepath.Join(os.TempDir(), "test_crush_harness_cmd")
	_ = os.RemoveAll(tempDir)
	defer os.RemoveAll(tempDir)

	h, _ := InitOrLoad(tempDir)

	// Test /exp
	res, matched := HandleExplicitCommand(context.Background(), "/exp", h, nil)
	if !matched || !strings.Contains(res, "Harness² 持续学习经验库") {
		t.Errorf("expected /exp matched with formatted markdown")
	}

	// Test /rules
	res, matched = HandleExplicitCommand(context.Background(), "/rules", h, nil)
	if !matched || !strings.Contains(res, "工程规范与核心约束") {
		t.Errorf("expected /rules matched with formatted markdown")
	}

	// Test /harness
	res, matched = HandleExplicitCommand(context.Background(), "/harness", h, nil)
	if !matched || !strings.Contains(res, "Harness² 激活线束架构状态") {
		t.Errorf("expected /harness matched with status markdown")
	}

	// Test normal message -> not matched
	res, matched = HandleExplicitCommand(context.Background(), "帮我写个 hello world", h, nil)
	if matched {
		t.Errorf("expected normal prompt to not match")
	}
}
