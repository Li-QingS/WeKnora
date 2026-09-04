package agent

import (
	"strings"
	"testing"

	"github.com/Tencent/WeKnora/internal/agent/skills"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFormatSkillsMetadataIncludesShellGuidanceOnlyWhenEnabled(t *testing.T) {
	metadata := []*skills.SkillMetadata{{Name: "demo", Description: "demo skill"}}

	enabled := formatSkillsMetadata(metadata, true)
	require.Contains(t, enabled, "shell_exec")
	for _, command := range []string{"find", "file", "cat", "head", "tail", "sed", "grep", "awk"} {
		assert.Contains(t, enabled, command)
	}
	assert.Contains(t, enabled, "Freely execute shell commands")
	assert.Contains(t, enabled, "Binary output is suppressed")

	disabled := formatSkillsMetadata(metadata, false)
	assert.NotContains(t, disabled, "shell_exec")
}

func TestFormatSkillsMetadataSortsByName(t *testing.T) {
	metadata := []*skills.SkillMetadata{
		{Name: "zeta", Description: "z skill"},
		{Name: "alpha", Description: "a skill"},
		{Name: "mid", Description: "m skill"},
	}

	out := formatSkillsMetadata(metadata, false)
	alpha := strings.Index(out, "1. **alpha**")
	mid := strings.Index(out, "2. **mid**")
	zeta := strings.Index(out, "3. **zeta**")
	if alpha < 0 || mid < 0 || zeta < 0 {
		t.Fatalf("missing sorted skill entries in: %s", out)
	}
	if !(alpha < mid && mid < zeta) {
		t.Fatalf("skills are not rendered in deterministic name order: %s", out)
	}
}

func TestFormatKnowledgeBaseListSortsByIDAndCapabilities(t *testing.T) {
	kbs := []*KnowledgeBaseInfo{
		{ID: "kb-z", Name: "Z", Capabilities: []string{"chunks", "wiki"}},
		{ID: "kb-a", Name: "A", Capabilities: []string{"wiki", "chunks"}},
	}

	out := formatKnowledgeBaseList(kbs)
	a := strings.Index(out, `id="kb-a"`)
	z := strings.Index(out, `id="kb-z"`)
	if a < 0 || z < 0 || a > z {
		t.Fatalf("knowledge bases are not deterministic by ID: %s", out)
	}
	if !strings.Contains(out, `capabilities="chunks,wiki"`) {
		t.Fatalf("capabilities are not deterministic: %s", out)
	}
}
