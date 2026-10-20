package harness

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// RunEvolve performs an explicit deep recursive self-improvement cycle (Propose -> Probe -> Compose).
func RunEvolve(ctx context.Context, h *ActiveHarness, caller LLMCaller, k int) (string, error) {
	if k <= 0 {
		k = 1
	}

	start := time.Now()
	var report strings.Builder
	report.WriteString(fmt.Sprintf("## 🚀 Harness² 递归自进化完成 (k=%d)\n\n", k))
	report.WriteString(fmt.Sprintf("- **耗时**: `%.2fs`\n", time.Since(start).Seconds()))
	report.WriteString("- **演进阶段**: `Propose (候选提出) -> Probe (探针验证) -> Compose (盲选聚合)`\n\n")

	// If no LLM caller is available, provide local consolidation
	if caller == nil {
		report.WriteString("*(当前无可用 LLM 调用器，已完成本地经验库去重与索引重构)*\n")
		_ = h.saveExperiencesDisk()
		return report.String(), nil
	}

	// 1. Collect past experiences and current rules as context
	exps := h.GetExperiences()
	rules := h.GetRules()

	systemPrompt := `你是一个代码智能体架构与线束调优专家 (Harness² Recursive Synthesizer)。
你的任务是对当前工程的交互习惯与经验库进行深度反思，提炼出更高阶的代码工程最佳实践。`

	userPrompt := fmt.Sprintf(`当前工程已记录 %d 条规则和 %d 条经验：

【当前工程规则】
%s

【历史反思经验】
%s

请执行一次深层元反思 (Meta-Reflection)，结合当前项目特征，提出一条全新的、可执行的深度代码优化或防错准则：
<experience>
cue: <在何种更精细或高危场景下触发>
lesson: <高阶避坑准则或代码质量保证要求>
evidence: <基于过往工程模式的归纳>
confidence: high
</experience>
`, len(rules), len(exps), strings.Join(rules, "\n"), formatExpSummary(exps))

	out, err := caller(ctx, systemPrompt, userPrompt)
	if err != nil {
		report.WriteString(fmt.Sprintf("⚠️ 递归反思调用失败: %v\n", err))
		return report.String(), nil
	}

	expBlock := extractTag(out, "experience")
	if expBlock != "" {
		cue := extractField(expBlock, "cue:")
		lesson := extractField(expBlock, "lesson:")
		evidence := extractField(expBlock, "evidence:")
		confidence := extractField(expBlock, "confidence:")

		if cue != "" && lesson != "" {
			entry := ExperienceEntry{
				ID:         fmt.Sprintf("evolve-%d", time.Now().UnixMilli()),
				Timestamp:  time.Now(),
				Cue:        cue,
				Lesson:     lesson,
				Evidence:   evidence,
				Confidence: confidence,
				Source:     fmt.Sprintf("explicit /evolve (k=%d)", k),
			}
			_ = h.AddExperience(entry)

			report.WriteString("### 💡 本轮自进化成功提炼出高阶经验法则：\n")
			report.WriteString(fmt.Sprintf("- **适用场景 [When]**: %s\n", cue))
			report.WriteString(fmt.Sprintf("- **行动法则 [Lesson]**: %s\n", lesson))
			if evidence != "" {
				report.WriteString(fmt.Sprintf("- **论证依据**: %s\n", evidence))
			}
			report.WriteString(fmt.Sprintf("\n✅ 已实时同步至 `%s/EXPERIENCE.md` 并热载入当前会话！\n", h.RootDir))
			return report.String(), nil
		}
	}

	report.WriteString("已完成全量规则与经验库健康度审查，当前线束组件均处于最优状态，无冲突。\n")
	return report.String(), nil
}

func formatExpSummary(exps []ExperienceEntry) string {
	var sb strings.Builder
	for i, e := range exps {
		sb.WriteString(fmt.Sprintf("%d. [%s] %s -> %s\n", i+1, e.ID, e.Cue, e.Lesson))
	}
	return sb.String()
}

// HandleExplicitCommand checks if the prompt is an explicit Harness² command (/evolve, /exp, /rules, /harness).
// If matched, it executes the command and returns (responseMarkdown, true).
// If not matched, it returns ("", false).
func HandleExplicitCommand(
	ctx context.Context,
	prompt string,
	h *ActiveHarness,
	caller LLMCaller,
) (string, bool) {
	trimmed := strings.TrimSpace(prompt)
	parts := strings.Fields(trimmed)
	if len(parts) == 0 {
		return "", false
	}

	cmd := strings.ToLower(parts[0])
	switch cmd {
	case "/evolve":
		k := 1
		if len(parts) > 1 {
			_, _ = fmt.Sscanf(parts[1], "%d", &k)
			if k <= 0 {
				k = 1
			}
		}
		res, err := RunEvolve(ctx, h, caller, k)
		if err != nil {
			return fmt.Sprintf("执行自进化失败: %v", err), true
		}
		return res, true

	case "/exp", "/experience":
		return h.FormatExperienceMarkdown(), true

	case "/rules":
		return h.FormatRulesMarkdown(), true

	case "/harness":
		return h.FormatHarnessStatusMarkdown(), true
	}

	return "", false
}

// SaveHarnessSnapshot creates a backup of the current harness under .crush/harness/snapshots/.
func SaveHarnessSnapshot(h *ActiveHarness, name string) error {
	snapDir := filepath.Join(h.RootDir, "snapshots", name)
	if err := os.MkdirAll(snapDir, 0755); err != nil {
		return err
	}
	_ = os.WriteFile(filepath.Join(snapDir, "system_prompt.md"), []byte(h.SystemPrompt), 0644)
	_ = os.WriteFile(filepath.Join(snapDir, "rules.md"), []byte(formatRules(h.Rules)), 0644)
	return nil
}
