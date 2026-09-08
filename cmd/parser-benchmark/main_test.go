package main

import (
	"encoding/json"
	"math"
	"strings"
	"testing"
)

func TestScore(t *testing.T) {
	gold := goldFile{
		ContentPhrases: []string{"Alpha One", "Beta Two"},
		Structure: []structureCheck{
			{Name: "heading", Pattern: `(?m)^# Alpha One$`},
			{Name: "table", Pattern: `(?m)^\| A \| B \|$`},
		},
	}
	contentHits, contentTotal, structureHits, structureTotal, content, structure, quality := score("# Alpha One\n\n| A | B |", gold)
	if contentHits != 1 || contentTotal != 2 || structureHits != 2 || structureTotal != 2 {
		t.Fatalf("unexpected counts: content=%d/%d structure=%d/%d", contentHits, contentTotal, structureHits, structureTotal)
	}
	if *content != 0.5 || *structure != 1 || math.Abs(*quality-0.6) > 1e-12 {
		t.Fatalf("unexpected scores: content=%v structure=%v quality=%v", *content, *structure, *quality)
	}
}

func TestUnavailableScoreSerializesNull(t *testing.T) {
	data, err := json.Marshal(engineResult{Engine: "cloud", Status: "unavailable"})
	if err != nil {
		t.Fatal(err)
	}
	if string(data) == "" || !containsAll(string(data), `"content_recall":null`, `"structure_recall":null`, `"quality_score":null`) {
		t.Fatalf("null score contract missing: %s", data)
	}
}

func containsAll(value string, parts ...string) bool {
	for _, part := range parts {
		if !strings.Contains(value, part) {
			return false
		}
	}
	return true
}
