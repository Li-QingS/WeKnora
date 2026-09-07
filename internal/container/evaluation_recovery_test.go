package container

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/Tencent/WeKnora/internal/types"
)

func TestRecoveredWikiTerminalStatus(t *testing.T) {
	tests := []struct {
		name        string
		typeValue   types.EvaluationType
		failure     types.EvaluationStage
		result      json.RawMessage
		want        types.EvaluationStatue
		wantPromote bool
	}{
		{"failed cleanup", types.EvaluationTypeWiki, types.EvaluationStageGenerating, nil, types.EvaluationStatueFailed, true},
		{"saved result", types.EvaluationTypeWiki, "", json.RawMessage(`{"metric":{}}`), types.EvaluationStatueSuccess, true},
		{"crashed mid-run", types.EvaluationTypeWiki, "", nil, types.EvaluationStatueInterrupted, false},
		{"empty result", types.EvaluationTypeWiki, "", json.RawMessage(`{}`), types.EvaluationStatueInterrupted, false},
		{"rag stays interrupted", types.EvaluationTypeRAG, types.EvaluationStageGenerating, nil, types.EvaluationStatueInterrupted, false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, promote := recoveredWikiTerminalStatus(test.typeValue, test.failure, test.result)
			assert.Equal(t, test.want, got)
			assert.Equal(t, test.wantPromote, promote)
		})
	}
}
