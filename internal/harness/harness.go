package harness

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// ActiveHarness manages the agent's editable runtime harness on disk and in memory.
type ActiveHarness struct {
	mu           sync.RWMutex
	RootDir      string
	SystemPrompt string
	Rules        []string
	Experiences  []ExperienceEntry
}

// InitOrLoad initializes or loads the active harness from the given directory.
func InitOrLoad(dir string) (*ActiveHarness, error) {
	if dir == "" {
		dir = filepath.Join(".crush", "harness")
	}

	h := &ActiveHarness{
		RootDir: dir,
	}

	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create harness dir %s: %w", dir, err)
	}

	// 1. system_prompt.md
	sysPath := filepath.Join(dir, "system_prompt.md")
	if data, err := os.ReadFile(sysPath); err == nil && len(data) > 0 {
		h.SystemPrompt = string(data)
	} else {
		h.SystemPrompt = defaultSystemPrompt
		_ = os.WriteFile(sysPath, []byte(defaultSystemPrompt), 0644)
	}

	// 2. rules.md
	rulesPath := filepath.Join(dir, "rules.md")
	if data, err := os.ReadFile(rulesPath); err == nil && len(data) > 0 {
		h.Rules = parseRules(string(data))
	} else {
		h.Rules = []string{
			"优先使用工具阅读和验证代码，避免盲目猜测文件位置或内容",
			"修改代码后及时运行单元测试或语法检查，确保改动正确无误",
			"遵从项目技术栈约定，不引入未授权的异构语言脚本或环境依赖",
		}
		_ = os.WriteFile(rulesPath, []byte(formatRules(h.Rules)), 0644)
	}

	// 3. EXPERIENCE.md & INDEX.md
	expPath := filepath.Join(dir, "EXPERIENCE.md")
	if data, err := os.ReadFile(expPath); err == nil && len(data) > 0 {
		h.Experiences = parseExperiences(string(data))
	} else {
		baseExp := ExperienceEntry{
			ID:         "exp-init",
			Timestamp:  time.Now(),
			Cue:        "在复杂或多子目录工程中定位代码文件时",
			Lesson:     "若直接通过相对路径读取文件报 no such file or directory，先执行文件列表或搜索工具查明项目具体结构，再按完整路径读取",
			Evidence:   "多子工程仓库经常存在同名 cmd 或 scripts 目录",
			Confidence: "high",
			Source:     "init",
		}
		h.Experiences = append(h.Experiences, baseExp)
		_ = h.saveExperiencesDisk()
	}

	return h, nil
}

// AugmentSystemPrompt injects the dynamic rules and continual learning experiences
// into Crush's active system prompt.
func (h *ActiveHarness) AugmentSystemPrompt(base string) string {
	h.mu.RLock()
	defer h.mu.RUnlock()

	var sb strings.Builder
	sb.WriteString(strings.TrimSpace(base))
	sb.WriteString("\n\n")

	// Dynamic Rules Section
	if len(h.Rules) > 0 {
		sb.WriteString("<engineering_rules>\n")
		sb.WriteString("These project rules were established for this codebase. You MUST strictly follow them:\n")
		for i, r := range h.Rules {
			sb.WriteString(fmt.Sprintf("%d. %s\n", i+1, r))
		}
		sb.WriteString("</engineering_rules>\n\n")
	}

	// Dynamic Experience Library Section (RSI Continual Learning)
	if len(h.Experiences) > 0 {
		sb.WriteString("<continual_learning_memory>\n")
		sb.WriteString("The following empirical lessons were learned autonomously from past interactions and tool errors. Do not repeat past mistakes:\n")

		start := 0
		if len(h.Experiences) > 8 {
			start = len(h.Experiences) - 8
		}
		for i := len(h.Experiences) - 1; i >= start; i-- {
			e := h.Experiences[i]
			sb.WriteString(fmt.Sprintf("- When [Cue]: %s\n", e.Cue))
			sb.WriteString(fmt.Sprintf("  Rule [Lesson]: %s\n", e.Lesson))
			if e.Confidence != "" {
				sb.WriteString(fmt.Sprintf("  (Confidence: %s)\n", e.Confidence))
			}
		}
		sb.WriteString("</continual_learning_memory>\n\n")
	}

	return sb.String()
}

// AddExperience appends a newly discovered experience and synchronizes to disk.
func (h *ActiveHarness) AddExperience(exp ExperienceEntry) error {
	h.mu.Lock()
	defer h.mu.Unlock()

	if exp.ID == "" {
		exp.ID = fmt.Sprintf("exp-%d", time.Now().UnixMilli())
	}
	if exp.Timestamp.IsZero() {
		exp.Timestamp = time.Now()
	}

	for _, existing := range h.Experiences {
		if strings.TrimSpace(existing.Lesson) == strings.TrimSpace(exp.Lesson) {
			return nil // Already captured
		}
	}

	h.Experiences = append(h.Experiences, exp)
	return h.saveExperiencesDisk()
}

// AddRule appends an explicit rule and synchronizes to disk.
func (h *ActiveHarness) AddRule(rule string) error {
	h.mu.Lock()
	defer h.mu.Unlock()

	rule = strings.TrimSpace(rule)
	if rule == "" {
		return nil
	}
	for _, r := range h.Rules {
		if r == rule {
			return nil
		}
	}
	h.Rules = append(h.Rules, rule)

	rulesPath := filepath.Join(h.RootDir, "rules.md")
	return os.WriteFile(rulesPath, []byte(formatRules(h.Rules)), 0644)
}

// GetExperiences returns a copy of current experiences.
func (h *ActiveHarness) GetExperiences() []ExperienceEntry {
	h.mu.RLock()
	defer h.mu.RUnlock()
	res := make([]ExperienceEntry, len(h.Experiences))
	copy(res, h.Experiences)
	return res
}

// GetRules returns a copy of current rules.
func (h *ActiveHarness) GetRules() []string {
	h.mu.RLock()
	defer h.mu.RUnlock()
	res := make([]string, len(h.Rules))
	copy(res, h.Rules)
	return res
}

// FormatExperienceMarkdown returns a user-facing Markdown representation of all experiences.
func (h *ActiveHarness) FormatExperienceMarkdown() string {
	exps := h.GetExperiences()
	var sb strings.Builder
	sb.WriteString("## 🧠 Harness² 持续学习经验库 (Continual Learning Memory)\n\n")
	sb.WriteString(fmt.Sprintf("当前项目已累计沉淀 **%d** 条自主迭代经验法则：\n\n", len(exps)))

	for i, e := range exps {
		sb.WriteString(fmt.Sprintf("### %d. `%s` (%s)\n", i+1, e.ID, e.Timestamp.Format("2006-01-02 15:04")))
		sb.WriteString(fmt.Sprintf("- **触发场景 [When]**: %s\n", e.Cue))
		sb.WriteString(fmt.Sprintf("- **经验法则 [Lesson]**: %s\n", e.Lesson))
		if e.Evidence != "" {
			sb.WriteString(fmt.Sprintf("- **支持证据 [Evidence]**: %s\n", e.Evidence))
		}
		if e.Confidence != "" {
			sb.WriteString(fmt.Sprintf("- **置信度**: `%s`\n", e.Confidence))
		}
		if e.Source != "" {
			sb.WriteString(fmt.Sprintf("- **来源**: %s\n", e.Source))
		}
		sb.WriteString("\n")
	}
	return sb.String()
}

// FormatRulesMarkdown returns a user-facing Markdown representation of all rules.
func (h *ActiveHarness) FormatRulesMarkdown() string {
	rules := h.GetRules()
	var sb strings.Builder
	sb.WriteString("## 📋 当前项目工程规范与核心约束 (Active Rules)\n\n")
	for i, r := range rules {
		sb.WriteString(fmt.Sprintf("%d. %s\n", i+1, r))
	}
	sb.WriteString("\n*(可在对话中直接纠偏，例如：'不要使用 Python'，系统将自动录入规则库)*\n")
	return sb.String()
}

// FormatHarnessStatusMarkdown returns a user-facing overview of the active harness.
func (h *ActiveHarness) FormatHarnessStatusMarkdown() string {
	h.mu.RLock()
	defer h.mu.RUnlock()

	var sb strings.Builder
	sb.WriteString("## ⚙️ Harness² 激活线束架构状态\n\n")
	sb.WriteString(fmt.Sprintf("- **线束存储目录**: `%s`\n", h.RootDir))
	sb.WriteString(fmt.Sprintf("- **工程约束规则**: `%d 条` (详见 `rules.md`)\n", len(h.Rules)))
	sb.WriteString(fmt.Sprintf("- **自主迭代经验**: `%d 条` (详见 `EXPERIENCE.md`)\n", len(h.Experiences)))
	sb.WriteString("- **RSI 自迭代模式**: `隐式后台自动触发 (Implicit) + 支持 /evolve 显式演变`\n")
	sb.WriteString("\n### 可用快捷操作\n")
	sb.WriteString("- `/evolve [k]`: 显式触发深度递归自进化流水线 (Propose -> Probe -> Compose)\n")
	sb.WriteString("- `/exp`: 查看所有已提炼经验法则\n")
	sb.WriteString("- `/rules`: 查看当前生效的工程规范\n")
	return sb.String()
}

func (h *ActiveHarness) saveExperiencesDisk() error {
	expPath := filepath.Join(h.RootDir, "EXPERIENCE.md")
	idxPath := filepath.Join(h.RootDir, "INDEX.md")

	var sb strings.Builder
	sb.WriteString("# Continual Learning Experience Library\n\n")
	sb.WriteString("This library stores empirical rules and lessons learned autonomously from daily conversation and tool execution.\n\n")

	for _, e := range h.Experiences {
		sb.WriteString(fmt.Sprintf("### Entry `%s` (%s)\n", e.ID, e.Timestamp.Format("2006-01-02 15:04:05")))
		sb.WriteString(fmt.Sprintf("- **Cue**: %s\n", e.Cue))
		sb.WriteString(fmt.Sprintf("- **Lesson**: %s\n", e.Lesson))
		if e.Evidence != "" {
			sb.WriteString(fmt.Sprintf("- **Evidence**: %s\n", e.Evidence))
		}
		if e.Confidence != "" {
			sb.WriteString(fmt.Sprintf("- **Confidence**: %s\n", e.Confidence))
		}
		if e.Source != "" {
			sb.WriteString(fmt.Sprintf("- **Source**: %s\n", e.Source))
		}
		sb.WriteString("\n---\n\n")
	}

	if err := os.WriteFile(expPath, []byte(sb.String()), 0644); err != nil {
		return err
	}

	var idx strings.Builder
	idx.WriteString(fmt.Sprintf("# Experience Index (%d total lessons)\n\n", len(h.Experiences)))
	for i, e := range h.Experiences {
		idx.WriteString(fmt.Sprintf("%d. [%s] **%s** -> %s\n", i+1, e.ID, e.Cue, e.Lesson))
	}
	return os.WriteFile(idxPath, []byte(idx.String()), 0644)
}

func parseRules(content string) []string {
	var rules []string
	lines := strings.Split(content, "\n")
	for _, l := range lines {
		l = strings.TrimSpace(l)
		if strings.HasPrefix(l, "- ") || strings.HasPrefix(l, "* ") {
			rules = append(rules, strings.TrimSpace(l[2:]))
		} else if len(l) > 2 && l[1] == '.' && l[0] >= '0' && l[0] <= '9' {
			rules = append(rules, strings.TrimSpace(l[2:]))
		}
	}
	return rules
}

func formatRules(rules []string) string {
	var sb strings.Builder
	sb.WriteString("# Codebase and Engineering Rules\n\n")
	for i, r := range rules {
		sb.WriteString(fmt.Sprintf("%d. %s\n", i+1, r))
	}
	return sb.String()
}

func parseExperiences(content string) []ExperienceEntry {
	var exps []ExperienceEntry
	entries := strings.Split(content, "### Entry ")
	for _, entry := range entries {
		entry = strings.TrimSpace(entry)
		if entry == "" || strings.HasPrefix(entry, "# Continual Learning") {
			continue
		}
		lines := strings.Split(entry, "\n")
		var e ExperienceEntry
		if len(lines) > 0 {
			fields := strings.Fields(lines[0])
			if len(fields) > 0 {
				e.ID = strings.Trim(fields[0], "`")
			}
		}
		for _, l := range lines {
			l = strings.TrimSpace(l)
			if strings.HasPrefix(l, "- **Cue**:") {
				e.Cue = strings.TrimSpace(strings.TrimPrefix(l, "- **Cue**:"))
			} else if strings.HasPrefix(l, "- **Lesson**:") {
				e.Lesson = strings.TrimSpace(strings.TrimPrefix(l, "- **Lesson**:"))
			} else if strings.HasPrefix(l, "- **Evidence**:") {
				e.Evidence = strings.TrimSpace(strings.TrimPrefix(l, "- **Evidence**:"))
			} else if strings.HasPrefix(l, "- **Confidence**:") {
				e.Confidence = strings.TrimSpace(strings.TrimPrefix(l, "- **Confidence**:"))
			} else if strings.HasPrefix(l, "- **Source**:") {
				e.Source = strings.TrimSpace(strings.TrimPrefix(l, "- **Source**:"))
			}
		}
		if e.Lesson != "" {
			exps = append(exps, e)
		}
	}
	return exps
}

const defaultSystemPrompt = `You are Crush equipped with Harness² Recursive Self-Improvement capabilities.
Operate autonomously, verify changes with tests, and continuously integrate learned experience.`
