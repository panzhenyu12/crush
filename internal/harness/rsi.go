package harness

import (
	"context"
	"fmt"
	"log/slog"
	"regexp"
	"strings"
	"time"
)

var (
	// Detection patterns for user corrections, constraints, or preferences
	rxUserCorrection = regexp.MustCompile(`(?i)(不要|不对|写错|不能|禁止|必须|应该|以后|记住|规则|规范|改用|注意|路径在|不要再|prefer|don't|dont|avoid|should|wrong|fix|must|never|always)`)

	// Detection patterns for errors inside tool results
	rxToolError = regexp.MustCompile(`(?i)(error|failed|failure|no such file|command not found|exit status [1-9]|undefined:|syntax error|cannot find)`)
)

// Engine coordinates conversational RSI analysis, distillation, and harness hot-reloading.
type Engine struct {
	harness *ActiveHarness
	caller  LLMCaller
}

// NewEngine creates an RSI engine instance.
func NewEngine(h *ActiveHarness, caller LLMCaller) *Engine {
	return &Engine{
		harness: h,
		caller:  caller,
	}
}

// ShouldReflect evaluates if the turn contains signals warranting an implicit RSI reflection.
func (e *Engine) ShouldReflect(turn Turn) (bool, string) {
	// 1. Tool execution had errors or failures
	for _, t := range turn.Tools {
		if t.Error != "" || rxToolError.MatchString(t.Output) {
			return true, fmt.Sprintf("工具调用触发异常或重试: %s", t.Name)
		}
	}

	// 2. User message contains correction or constraint cues
	if rxUserCorrection.MatchString(turn.UserMessage) {
		return true, "用户输入包含纠偏、限制或工作流指引"
	}

	// 3. Multi-tool sequence (agent performed non-trivial discovery or coding)
	if len(turn.Tools) >= 2 {
		return true, "完成了多步骤工具交互排查"
	}

	return false, ""
}

// AnalyzeAndEvolveAsync processes the turn asynchronously in background (implicit trigger).
func (e *Engine) AnalyzeAndEvolveAsync(
	turn Turn,
	onExpLearned func(ExperienceEntry),
	onRuleLearned func(string),
) {
	should, reason := e.ShouldReflect(turn)
	if !should {
		return
	}

	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel()

		res, err := e.Reflect(ctx, turn, reason)
		if err != nil {
			slog.Debug("Harness² RSI reflection skipped or failed", "error", err)
			return
		}
		if res == nil {
			return
		}

		if res.LearnedExperience != nil {
			if err := e.harness.AddExperience(*res.LearnedExperience); err == nil {
				slog.Info("Harness² RSI (Implicit): learned new experience",
					"cue", res.LearnedExperience.Cue,
					"lesson", res.LearnedExperience.Lesson)
				if onExpLearned != nil {
					onExpLearned(*res.LearnedExperience)
				}
			}
		}

		if res.LearnedRule != "" {
			if err := e.harness.AddRule(res.LearnedRule); err == nil {
				slog.Info("Harness² RSI (Implicit): learned new rule", "rule", res.LearnedRule)
				if onRuleLearned != nil {
					onRuleLearned(res.LearnedRule)
				}
			}
		}
	}()
}

// Reflect conducts an LLM-driven meta-reflection on the conversational trajectory.
func (e *Engine) Reflect(ctx context.Context, turn Turn, triggerReason string) (*ReflectionResult, error) {
	if e.caller == nil {
		return nil, fmt.Errorf("no LLM caller configured for RSI engine")
	}

	var toolLog strings.Builder
	for i, t := range turn.Tools {
		toolLog.WriteString(fmt.Sprintf("[%d] 工具: %s(args: %s)\n", i+1, t.Name, t.Args))
		out := t.Output
		if len(out) > 300 {
			out = out[:300] + "... (截断)"
		}
		if t.Error != "" {
			toolLog.WriteString(fmt.Sprintf("    失败原因: %s\n", t.Error))
		} else {
			toolLog.WriteString(fmt.Sprintf("    输出结果: %s\n", out))
		}
	}

	systemPrompt := "你是一个专业的 AI 智能体自迭代 (Recursive Self-Improvement) 与经验提炼元认知反思器。只输出结构化提炼结果，不要客套。"

	userPrompt := fmt.Sprintf(`观察智能体与用户之间的真实交互记录，自动提炼出可在后续会话中长期生效的【经验法则】或【工程规则】。

【触发原因】: %s

【当前交互记录】
用户输入: %s
智能体回复: %s
工具执行流水:
%s

【反思提炼要求】
1. 如果该交互揭示了特定的代码规范、路径习惯、工具排错教训或用户偏好，提炼为精炼、可执行的经验法则。
2. 格式规范：
<experience>
cue: <在何种场景或条件下触发此规则>
lesson: <明确、具体的行动法则或避坑准则>
evidence: <本次交互中支持该规则的证据>
confidence: high | medium
</experience>

3. 如果属于明确的开发强制规则（如架构限制、语言限制），可额外输出：
<rule>
明确的单条规则陈述
</rule>

4. 若本次交互纯属闲聊或无任何泛化价值的临时信息，请直接输出：
<none></none>
`, triggerReason, turn.UserMessage, turn.AssistantReply, toolLog.String())

	content, err := e.caller(ctx, systemPrompt, userPrompt)
	if err != nil {
		return nil, err
	}

	if strings.Contains(content, "<none>") {
		return nil, nil
	}

	res := &ReflectionResult{
		Reason: triggerReason,
	}

	// Parse <experience>
	exp := extractTag(content, "experience")
	if exp != "" {
		cue := extractField(exp, "cue:")
		lesson := extractField(exp, "lesson:")
		evidence := extractField(exp, "evidence:")
		confidence := extractField(exp, "confidence:")

		if cue != "" && lesson != "" {
			res.LearnedExperience = &ExperienceEntry{
				Cue:        cue,
				Lesson:     lesson,
				Evidence:   evidence,
				Confidence: confidence,
				Source:     triggerReason,
			}
		}
	}

	// Parse <rule>
	rule := extractTag(content, "rule")
	if rule != "" {
		res.LearnedRule = strings.TrimSpace(rule)
	}

	return res, nil
}

func extractTag(s, tag string) string {
	open := "<" + tag + ">"
	close := "</" + tag + ">"
	start := strings.Index(s, open)
	if start == -1 {
		return ""
	}
	start += len(open)
	end := strings.Index(s[start:], close)
	if end == -1 {
		return ""
	}
	return strings.TrimSpace(s[start : start+end])
}

func extractField(block, prefix string) string {
	lines := strings.Split(block, "\n")
	for _, l := range lines {
		l = strings.TrimSpace(l)
		if strings.HasPrefix(strings.ToLower(l), strings.ToLower(prefix)) {
			return strings.TrimSpace(l[len(prefix):])
		}
	}
	return ""
}
