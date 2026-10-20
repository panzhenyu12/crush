package harness

import (
	"context"
	"time"
)

// ExperienceEntry represents an empirical rule or lesson learned via Harness² RSI.
type ExperienceEntry struct {
	ID         string    `json:"id"`
	Timestamp  time.Time `json:"timestamp"`
	Cue        string    `json:"cue"`        // When this situation applies
	Lesson     string    `json:"lesson"`     // Actionable rule or behavior
	Evidence   string    `json:"evidence"`   // What interaction proved this
	Confidence string    `json:"confidence"` // high | medium | low
	Source     string    `json:"source"`     // Trigger reason / event
}

// ToolRecord captures a single tool call in a conversational turn.
type ToolRecord struct {
	Name   string `json:"name"`
	Args   string `json:"args"`
	Output string `json:"output"`
	Error  string `json:"error"`
}

// Turn captures the full context of a user turn and the assistant's actions.
type Turn struct {
	UserMessage    string       `json:"user_message"`
	AssistantReply string       `json:"assistant_reply"`
	Tools          []ToolRecord `json:"tools"`
}

// ReflectionResult holds the outcome of an autonomous RSI iteration.
type ReflectionResult struct {
	LearnedExperience *ExperienceEntry `json:"learned_experience,omitempty"`
	LearnedRule       string           `json:"learned_rule,omitempty"`
	Reason            string           `json:"reason"`
}

// LLMCaller is a generic function to call an LLM with system and user prompts.
type LLMCaller func(ctx context.Context, systemPrompt, userPrompt string) (string, error)
