package types

import (
	"strings"
	"testing"
)

func TestRenderSystemPromptPlaceholdersSuppressesDynamicTime(t *testing.T) {
	tmpl := "You are WeKnora.\nCurrent time: {{current_time}} {{current_week}}\nLanguage: {{language}}"
	got := RenderSystemPromptPlaceholders(tmpl, PlaceholderValues{
		"language": "Chinese (Simplified)",
	})

	if strings.Contains(got, "{{current_time}}") || strings.Contains(got, "{{current_week}}") {
		t.Fatalf("system prompt still contains dynamic time placeholders: %q", got)
	}
	if !strings.Contains(got, "You are WeKnora.") || !strings.Contains(got, "Chinese (Simplified)") {
		t.Fatalf("system prompt lost static content: %q", got)
	}
}

func TestRenderPromptPlaceholdersStillFillsUserSideTime(t *testing.T) {
	tmpl := "Current time: {{current_time}}"
	got := RenderPromptPlaceholders(tmpl, nil)

	if strings.Contains(got, "{{current_time}}") {
		t.Fatalf("user-side placeholder was not filled: %q", got)
	}
	if !strings.ContainsAny(got, "0123456789") {
		t.Fatalf("user-side time rendering looks empty: %q", got)
	}
}
