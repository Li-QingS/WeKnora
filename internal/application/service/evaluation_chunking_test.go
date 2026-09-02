package service

import (
	"strings"
	"testing"

	"github.com/Tencent/WeKnora/internal/infrastructure/chunker"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/stretchr/testify/require"
)

func TestNormalizeEvaluationChunkingDefaults(t *testing.T) {
	cfg, err := normalizeEvaluationChunking(&types.EvaluationChunkingConfig{
		Strategy: chunker.StrategyRecursive,
	})
	require.NoError(t, err)
	require.NotNil(t, cfg)
	require.Equal(t, chunker.DefaultChunkSize, cfg.ChunkSize)
	require.Equal(t, chunker.DefaultChunkOverlap, cfg.ChunkOverlap)

	cfg, err = normalizeEvaluationChunking(&types.EvaluationChunkingConfig{
		Strategy: evaluationChunkStrategyPassthrough,
	})
	require.NoError(t, err)
	require.Equal(t, evaluationChunkStrategyPassthrough, cfg.Strategy)

	_, err = normalizeEvaluationChunking(&types.EvaluationChunkingConfig{
		Strategy:     "nope",
		ChunkSize:    512,
		ChunkOverlap: 80,
	})
	require.ErrorIs(t, err, ErrInvalidEvaluationParams)

	_, err = normalizeEvaluationChunking(&types.EvaluationChunkingConfig{
		Strategy:     "recursive",
		ChunkSize:    512,
		ChunkOverlap: 600,
	})
	require.ErrorIs(t, err, ErrInvalidEvaluationParams)
}

func TestChunkEvaluationPassagesSplitsDocuments(t *testing.T) {
	passage := strings.Repeat("Enterprise retrieval needs robust chunk boundaries.\n\n", 30)
	cfg, err := normalizeEvaluationChunking(&types.EvaluationChunkingConfig{
		Strategy:     "recursive",
		ChunkSize:    128,
		ChunkOverlap: 16,
	})
	require.NoError(t, err)

	chunked := chunkEvaluationPassages([]string{passage}, cfg)
	require.Greater(t, len(chunked), 1)
	for _, chunk := range chunked {
		require.True(t, strings.Contains(passage, chunk), "chunk must stay a substring of its passage")
	}
}

func TestChunkEvaluationPassagesPassthrough(t *testing.T) {
	cfg, err := normalizeEvaluationChunking(&types.EvaluationChunkingConfig{
		Strategy: evaluationChunkStrategyPassthrough,
	})
	require.NoError(t, err)
	out := chunkEvaluationPassages([]string{"one", "two"}, cfg)
	require.Equal(t, []string{"one", "two"}, out)

	require.Equal(t, []string{"one", "two"}, chunkEvaluationPassages([]string{"one", "two"}, nil))
}

func TestEvaluationChunkingSnapshotPassthrough(t *testing.T) {
	cfg, err := normalizeEvaluationChunking(&types.EvaluationChunkingConfig{
		Strategy: evaluationChunkStrategyPassthrough,
	})
	require.NoError(t, err)
	snapshot := evaluationChunkingSnapshot(cfg)
	require.NotNil(t, snapshot)
	require.Equal(t, evaluationChunkStrategyPassthrough, snapshot.Strategy)
	require.Zero(t, snapshot.ChunkSize)
	require.Zero(t, snapshot.ChunkOverlap)
	require.Nil(t, evaluationChunkingSnapshot(nil))
}
