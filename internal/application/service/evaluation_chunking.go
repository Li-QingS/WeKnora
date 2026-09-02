package service

import (
	"fmt"
	"strings"

	"github.com/Tencent/WeKnora/internal/infrastructure/chunker"
	"github.com/Tencent/WeKnora/internal/types"
)

const evaluationChunkStrategyPassthrough = "passthrough"

var allowedEvaluationChunkStrategies = map[string]bool{
	evaluationChunkStrategyPassthrough: true,
	chunker.StrategyAuto:               true,
	chunker.StrategyHeading:            true,
	chunker.StrategyHeuristic:          true,
	chunker.StrategyRecursive:          true,
	chunker.StrategyLegacy:             true,
}

// normalizeEvaluationChunking resolves defaults so the same config always
// produces the same recorded snapshot and splitter behavior.
func normalizeEvaluationChunking(cfg *types.EvaluationChunkingConfig) (*types.EvaluationChunkingConfig, error) {
	if cfg == nil {
		return nil, nil
	}
	normalized := *cfg
	normalized.Strategy = strings.ToLower(strings.TrimSpace(normalized.Strategy))
	if normalized.Strategy == "" {
		normalized.Strategy = chunker.StrategyRecursive
	}
	if !allowedEvaluationChunkStrategies[normalized.Strategy] {
		return nil, fmt.Errorf("%w: unsupported chunking strategy %q", ErrInvalidEvaluationParams, cfg.Strategy)
	}
	if normalized.Strategy == evaluationChunkStrategyPassthrough {
		return &normalized, nil
	}

	if normalized.ChunkSize < 0 {
		return nil, fmt.Errorf("%w: chunk_size must be positive", ErrInvalidEvaluationParams)
	}
	if normalized.ChunkOverlap < 0 {
		return nil, fmt.Errorf("%w: chunk_overlap must be non-negative", ErrInvalidEvaluationParams)
	}
	if normalized.ChunkSize == 0 {
		normalized.ChunkSize = chunker.DefaultChunkSize
	}
	if normalized.ChunkOverlap == 0 {
		normalized.ChunkOverlap = min(chunker.DefaultChunkOverlap, normalized.ChunkSize/2)
	}
	if normalized.ChunkOverlap >= normalized.ChunkSize {
		return nil, fmt.Errorf("%w: chunk_overlap must be less than chunk_size", ErrInvalidEvaluationParams)
	}
	if normalized.TokenLimit < 0 {
		return nil, fmt.Errorf("%w: token_limit must be positive", ErrInvalidEvaluationParams)
	}
	return &normalized, nil
}

func evaluationChunkingSnapshot(cfg *types.EvaluationChunkingConfig) *types.ChunkingSnapshot {
	if cfg == nil {
		return nil
	}
	snapshot := &types.ChunkingSnapshot{
		Strategy:     cfg.Strategy,
		ChunkSize:    cfg.ChunkSize,
		ChunkOverlap: cfg.ChunkOverlap,
		TokenLimit:   cfg.TokenLimit,
		Languages:    cfg.Languages,
	}
	if cfg.Strategy == evaluationChunkStrategyPassthrough {
		snapshot.ChunkSize = 0
		snapshot.ChunkOverlap = 0
		snapshot.Languages = nil
	}
	return snapshot
}

// chunkEvaluationPassages splits each corpus document into chunks before it
// enters the temporary knowledge base. Chunks stay contiguous substrings of
// their source passage so the existing content-based metric mapping still
// recognizes which dataset passage a retrieved chunk belongs to.
func chunkEvaluationPassages(passages []string, chunking *types.EvaluationChunkingConfig) []string {
	if chunking == nil || chunking.Strategy == evaluationChunkStrategyPassthrough {
		return passages
	}
	out := make([]string, 0, len(passages))
	splitter := chunker.SplitterConfig{
		ChunkSize:    chunking.ChunkSize,
		ChunkOverlap: chunking.ChunkOverlap,
		Strategy:     chunking.Strategy,
		TokenLimit:   chunking.TokenLimit,
		Languages:    chunking.Languages,
	}
	for _, passage := range passages {
		if strings.TrimSpace(passage) == "" {
			continue
		}
		for _, chunk := range chunker.Split(chunker.NormalizeLineEndings(passage), splitter) {
			if strings.TrimSpace(chunk.Content) != "" {
				out = append(out, chunk.Content)
			}
		}
	}
	return out
}
