package service

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/Tencent/WeKnora/internal/types"
)

const wikiGoldSchemaVersion = "1"

var (
	ErrWikiGoldNotFound = errors.New("wiki evaluation gold not found")
	ErrInvalidWikiGold  = errors.New("invalid wiki evaluation gold")
)

type wikiGoldLoader struct{}

func NewWikiGoldLoader() *wikiGoldLoader {
	return &wikiGoldLoader{}
}

func (l *wikiGoldLoader) Load(_ context.Context, dataset *types.EvaluationDataset) (*types.WikiGold, error) {
	if dataset == nil || strings.TrimSpace(dataset.ID) == "" {
		return nil, fmt.Errorf("%w: dataset is required", ErrInvalidWikiGold)
	}
	path := filepath.Join(datasetDir(dataset.ID), "wiki_gold.json")
	raw, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("%w: %s", ErrWikiGoldNotFound, dataset.ID)
		}
		return nil, fmt.Errorf("wiki gold: read %s: %w", path, err)
	}
	gold, err := decodeWikiGold(raw)
	if err != nil {
		return nil, err
	}
	if err := validateWikiGold(gold, dataset); err != nil {
		return nil, err
	}
	return gold, nil
}

func decodeWikiGold(raw []byte) (*types.WikiGold, error) {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	var gold types.WikiGold
	if err := decoder.Decode(&gold); err != nil {
		return nil, fmt.Errorf("%w: decode JSON: %v", ErrInvalidWikiGold, err)
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		if err == nil {
			return nil, fmt.Errorf("%w: trailing JSON value", ErrInvalidWikiGold)
		}
		return nil, fmt.Errorf("%w: trailing JSON: %v", ErrInvalidWikiGold, err)
	}
	return &gold, nil
}

func validateWikiGold(gold *types.WikiGold, dataset *types.EvaluationDataset) error {
	if gold == nil {
		return fmt.Errorf("%w: empty document", ErrInvalidWikiGold)
	}
	if gold.SchemaVersion != wikiGoldSchemaVersion {
		return fmt.Errorf("%w: schema_version %q, want %q", ErrInvalidWikiGold, gold.SchemaVersion, wikiGoldSchemaVersion)
	}
	if dataset == nil || gold.DatasetID != dataset.ID {
		return fmt.Errorf("%w: dataset_id %q does not match %q", ErrInvalidWikiGold, gold.DatasetID, datasetIDOf(dataset))
	}
	if gold.DatasetSHA256 == "" || gold.DatasetSHA256 != dataset.SHA256 {
		return fmt.Errorf("%w: dataset_sha256 %q does not match %q", ErrInvalidWikiGold, gold.DatasetSHA256, dataset.SHA256)
	}
	if len(gold.Nodes) == 0 {
		return fmt.Errorf("%w: nodes must not be empty", ErrInvalidWikiGold)
	}
	nodeIDs := make(map[string]struct{}, len(gold.Nodes))
	for i, node := range gold.Nodes {
		if strings.TrimSpace(node.ID) == "" {
			return fmt.Errorf("%w: nodes[%d].id is empty", ErrInvalidWikiGold, i)
		}
		if _, exists := nodeIDs[node.ID]; exists {
			return fmt.Errorf("%w: duplicate node id %q", ErrInvalidWikiGold, node.ID)
		}
		nodeIDs[node.ID] = struct{}{}
		if node.Type != types.WikiPageTypeEntity && node.Type != types.WikiPageTypeConcept {
			return fmt.Errorf("%w: node %q has invalid type %q", ErrInvalidWikiGold, node.ID, node.Type)
		}
		if strings.TrimSpace(node.Name) == "" {
			return fmt.Errorf("%w: node %q has empty name", ErrInvalidWikiGold, node.ID)
		}
		seenLabels := map[string]struct{}{canonicalGoldLabel(node.Name): {}}
		for _, alias := range node.Aliases {
			label := canonicalGoldLabel(alias)
			if label == "" {
				return fmt.Errorf("%w: node %q has empty alias", ErrInvalidWikiGold, node.ID)
			}
			if _, exists := seenLabels[label]; exists {
				return fmt.Errorf("%w: node %q has duplicate name or alias %q", ErrInvalidWikiGold, node.ID, alias)
			}
			seenLabels[label] = struct{}{}
		}
	}
	seenEdges := make(map[string]struct{}, len(gold.Edges))
	for i, edge := range gold.Edges {
		if _, ok := nodeIDs[edge.Source]; !ok {
			return fmt.Errorf("%w: edges[%d] source %q does not exist", ErrInvalidWikiGold, i, edge.Source)
		}
		if _, ok := nodeIDs[edge.Target]; !ok {
			return fmt.Errorf("%w: edges[%d] target %q does not exist", ErrInvalidWikiGold, i, edge.Target)
		}
		key := edge.Source + "\x00" + edge.Target
		if _, exists := seenEdges[key]; exists {
			return fmt.Errorf("%w: duplicate edge %q -> %q", ErrInvalidWikiGold, edge.Source, edge.Target)
		}
		seenEdges[key] = struct{}{}
	}
	want, err := wikiGoldContentSHA256(gold)
	if err != nil {
		return fmt.Errorf("%w: compute content_sha256: %v", ErrInvalidWikiGold, err)
	}
	if gold.ContentSHA256 == "" || gold.ContentSHA256 != want {
		return fmt.Errorf("%w: content_sha256 %q does not match %q", ErrInvalidWikiGold, gold.ContentSHA256, want)
	}
	return nil
}

func wikiGoldContentSHA256(gold *types.WikiGold) (string, error) {
	if gold == nil {
		return "", errors.New("nil gold")
	}
	canonical := *gold
	canonical.ContentSHA256 = ""
	canonical.Nodes = append([]types.WikiGoldNode(nil), gold.Nodes...)
	for i := range canonical.Nodes {
		canonical.Nodes[i].Aliases = append([]string(nil), canonical.Nodes[i].Aliases...)
		sort.Strings(canonical.Nodes[i].Aliases)
	}
	sort.Slice(canonical.Nodes, func(i, j int) bool { return canonical.Nodes[i].ID < canonical.Nodes[j].ID })
	canonical.Edges = append([]types.WikiGoldEdge(nil), gold.Edges...)
	sort.Slice(canonical.Edges, func(i, j int) bool {
		if canonical.Edges[i].Source != canonical.Edges[j].Source {
			return canonical.Edges[i].Source < canonical.Edges[j].Source
		}
		return canonical.Edges[i].Target < canonical.Edges[j].Target
	})
	encoded, err := json.Marshal(canonical)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(encoded)
	return hex.EncodeToString(sum[:]), nil
}

func wikiGoldSnapshot(gold *types.WikiGold) types.WikiGoldSnapshot {
	if gold == nil {
		return types.WikiGoldSnapshot{}
	}
	return types.WikiGoldSnapshot{
		SchemaVersion: gold.SchemaVersion,
		DatasetSHA256: gold.DatasetSHA256,
		ContentSHA256: gold.ContentSHA256,
		NodeCount:     len(gold.Nodes),
		EdgeCount:     len(gold.Edges),
	}
}

func canonicalGoldLabel(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}

func datasetIDOf(dataset *types.EvaluationDataset) string {
	if dataset == nil {
		return ""
	}
	return dataset.ID
}
