package service

import (
	"encoding/json"
	"testing"

	"github.com/Tencent/WeKnora/internal/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func validWikiGoldFixture(t *testing.T) (*types.WikiGold, *types.EvaluationDataset) {
	t.Helper()
	dataset := &types.EvaluationDataset{ID: "enterprise_rag", SHA256: "dataset-sha"}
	gold := &types.WikiGold{
		SchemaVersion: wikiGoldSchemaVersion,
		DatasetID:     dataset.ID,
		DatasetSHA256: dataset.SHA256,
		Nodes: []types.WikiGoldNode{
			{ID: "entity:acme", Type: types.WikiPageTypeEntity, Name: "Acme", Aliases: []string{"Acme Corp"}},
			{ID: "concept:rag", Type: types.WikiPageTypeConcept, Name: "RAG"},
		},
		Edges: []types.WikiGoldEdge{
			{Source: "entity:acme", Target: "concept:rag"},
			{Source: "concept:rag", Target: "concept:rag"},
		},
	}
	hash, err := wikiGoldContentSHA256(gold)
	require.NoError(t, err)
	gold.ContentSHA256 = hash
	return gold, dataset
}

func TestWikiGoldValidAndHashStable(t *testing.T) {
	gold, dataset := validWikiGoldFixture(t)
	require.NoError(t, validateWikiGold(gold, dataset))

	reordered := *gold
	reordered.Nodes = []types.WikiGoldNode{gold.Nodes[1], gold.Nodes[0]}
	reordered.Edges = []types.WikiGoldEdge{gold.Edges[1], gold.Edges[0]}
	reordered.Nodes[1].Aliases = []string{"Acme Corp"}
	hash, err := wikiGoldContentSHA256(&reordered)
	require.NoError(t, err)
	assert.Equal(t, gold.ContentSHA256, hash)
	assert.Equal(t, 2, wikiGoldSnapshot(gold).NodeCount)
}

func TestWikiGoldStrictDecode(t *testing.T) {
	_, err := decodeWikiGold([]byte(`{"schema_version":"1","unknown":true}`))
	require.ErrorIs(t, err, ErrInvalidWikiGold)
	_, err = decodeWikiGold([]byte(`{} {}`))
	require.ErrorIs(t, err, ErrInvalidWikiGold)
}

func TestWikiGoldValidationFailures(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*types.WikiGold, *types.EvaluationDataset)
	}{
		{"schema", func(g *types.WikiGold, _ *types.EvaluationDataset) { g.SchemaVersion = "2" }},
		{"dataset id", func(g *types.WikiGold, _ *types.EvaluationDataset) { g.DatasetID = "other" }},
		{"dataset hash", func(g *types.WikiGold, _ *types.EvaluationDataset) { g.DatasetSHA256 = "other" }},
		{"duplicate node", func(g *types.WikiGold, _ *types.EvaluationDataset) { g.Nodes = append(g.Nodes, g.Nodes[0]) }},
		{"invalid type", func(g *types.WikiGold, _ *types.EvaluationDataset) { g.Nodes[0].Type = "summary" }},
		{"duplicate alias", func(g *types.WikiGold, _ *types.EvaluationDataset) {
			g.Nodes[0].Aliases = append(g.Nodes[0].Aliases, " acme corp ")
		}},
		{"dangling source", func(g *types.WikiGold, _ *types.EvaluationDataset) { g.Edges[0].Source = "missing" }},
		{"dangling target", func(g *types.WikiGold, _ *types.EvaluationDataset) { g.Edges[0].Target = "missing" }},
		{"duplicate edge", func(g *types.WikiGold, _ *types.EvaluationDataset) { g.Edges = append(g.Edges, g.Edges[0]) }},
		{"content hash", func(g *types.WikiGold, _ *types.EvaluationDataset) { g.ContentSHA256 = "bad" }},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			gold, dataset := validWikiGoldFixture(t)
			tc.mutate(gold, dataset)
			err := validateWikiGold(gold, dataset)
			require.ErrorIs(t, err, ErrInvalidWikiGold)
		})
	}
}

func TestWikiGoldDecodeRoundTrip(t *testing.T) {
	gold, dataset := validWikiGoldFixture(t)
	raw, err := json.Marshal(gold)
	require.NoError(t, err)
	decoded, err := decodeWikiGold(raw)
	require.NoError(t, err)
	require.NoError(t, validateWikiGold(decoded, dataset))
}
