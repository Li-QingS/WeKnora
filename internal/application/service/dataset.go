package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"

	"github.com/Tencent/WeKnora/internal/logger"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
	"github.com/parquet-go/parquet-go"
)

// Dataset errors returned by DatasetService.
var (
	ErrDatasetNotFound = errors.New("dataset not found")
	ErrInvalidDataset  = errors.New("invalid dataset")
)

var datasetIDPattern = regexp.MustCompile(`^[A-Za-z0-9_-]+$`)

// datasetRoot is the directory that contains dataset directories. Kept as a
// variable so tests can point it at the repository root.
var datasetRoot = "./dataset"

// DatasetService provides operations for working with datasets
type DatasetService struct{}

// NewDatasetService creates a new DatasetService instance
func NewDatasetService() interfaces.DatasetService {
	return &DatasetService{}
}

// TextInfo represents text data with ID in parquet format
type TextInfo struct {
	ID   int64  `parquet:"id"`   // Unique identifier
	Text string `parquet:"text"` // Text content
}

// RelsInfo represents question-passage relations in parquet format
type RelsInfo struct {
	QID int64 `parquet:"qid"` // Question ID
	PID int64 `parquet:"pid"` // Passage ID
}

// QaInfo represents question-answer relations in parquet format
type QaInfo struct {
	QID int64 `parquet:"qid"` // Question ID
	AID int64 `parquet:"aid"` // Answer ID
}

// GetDatasetByID loads, validates and hashes the named dataset.
func (d *DatasetService) GetDatasetByID(ctx context.Context, datasetID string) (*types.EvaluationDataset, error) {
	logger.Info(ctx, "Start getting dataset by ID")
	logger.Infof(ctx, "Getting dataset with ID: %s", datasetID)

	if datasetID == "" {
		datasetID = "default"
	}
	if !validDatasetID(datasetID) {
		return nil, fmt.Errorf("%w: unsafe dataset id %q", ErrInvalidDataset, datasetID)
	}

	dir := datasetDir(datasetID)
	if _, err := os.Stat(dir); err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("%w: %s", ErrDatasetNotFound, datasetID)
		}
		return nil, fmt.Errorf("dataset: stat %s: %w", dir, err)
	}

	loaded, err := loadDatasetDir(dir)
	if err != nil {
		return nil, err
	}
	loaded.ID = datasetID
	logger.Infof(ctx, "Retrieved %d QA pairs from dataset", len(loaded.Pairs))
	return loaded, nil
}

// ListAvailableDatasets scans the dataset root and returns metadata for every
// valid dataset directory. The built-in "samples" directory is exposed as the
// "default" alias used by the evaluation API.
func (d *DatasetService) ListAvailableDatasets(ctx context.Context) ([]*types.EvaluationDatasetMeta, error) {
	logger.Info(ctx, "Listing available evaluation datasets")
	entries, err := os.ReadDir(datasetRoot)
	if err != nil {
		return nil, fmt.Errorf("dataset: read %s: %w", datasetRoot, err)
	}

	seen := make(map[string]bool)
	datasets := make([]*types.EvaluationDatasetMeta, 0, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() || entry.Name() == "" {
			continue
		}
		id := entry.Name()
		if !validDatasetID(id) {
			continue
		}
		if id == "samples" {
			id = "default"
		}
		if seen[id] {
			continue
		}
		seen[id] = true
		loaded, loadErr := loadDatasetDir(filepath.Join(datasetRoot, entry.Name()))
		if loadErr != nil {
			logger.Debugf(ctx, "Skipping invalid dataset directory %s: %v", entry.Name(), loadErr)
			continue
		}
		datasets = append(datasets, &types.EvaluationDatasetMeta{
			ID:          id,
			SHA256:      loaded.SHA256,
			SampleCount: loaded.SampleCount,
		})
	}
	sort.Slice(datasets, func(i, j int) bool { return datasets[i].ID < datasets[j].ID })
	logger.Infof(ctx, "Found %d evaluation datasets", len(datasets))
	return datasets, nil
}

func validDatasetID(id string) bool {
	if id == "" {
		return true
	}
	return datasetIDPattern.MatchString(id)
}

func datasetDir(id string) string {
	if id == "" || id == "default" {
		return filepath.Join(datasetRoot, "samples")
	}
	return filepath.Join(datasetRoot, id)
}

func loadDatasetDir(dir string) (*types.EvaluationDataset, error) {
	queries, err := loadParquet[TextInfo](filepath.Join(dir, "queries.parquet"))
	if err != nil {
		return nil, fmt.Errorf("%w: load queries.parquet: %v", ErrInvalidDataset, err)
	}
	corpus, err := loadParquet[TextInfo](filepath.Join(dir, "corpus.parquet"))
	if err != nil {
		return nil, fmt.Errorf("%w: load corpus.parquet: %v", ErrInvalidDataset, err)
	}
	answers, err := loadParquet[TextInfo](filepath.Join(dir, "answers.parquet"))
	if err != nil {
		return nil, fmt.Errorf("%w: load answers.parquet: %v", ErrInvalidDataset, err)
	}
	qrels, err := loadParquet[RelsInfo](filepath.Join(dir, "qrels.parquet"))
	if err != nil {
		return nil, fmt.Errorf("%w: load qrels.parquet: %v", ErrInvalidDataset, err)
	}
	qas, err := loadParquet[QaInfo](filepath.Join(dir, "qas.parquet"))
	if err != nil {
		return nil, fmt.Errorf("%w: load qas.parquet: %v", ErrInvalidDataset, err)
	}

	ds := buildDataset(queries, corpus, answers, qrels, qas)
	if err := validateDataset(ds); err != nil {
		return nil, err
	}
	hash, err := canonicalDatasetHash(queries, corpus, answers, qrels, qas)
	if err != nil {
		return nil, fmt.Errorf("%w: compute dataset hash: %v", ErrInvalidDataset, err)
	}
	pairs := ds.Iterate()
	return &types.EvaluationDataset{
		SHA256:      hash,
		SampleCount: len(pairs),
		Pairs:       pairs,
	}, nil
}

func buildDataset(
	queries []TextInfo,
	corpus []TextInfo,
	answers []TextInfo,
	qrels []RelsInfo,
	qas []QaInfo,
) dataset {
	res := dataset{
		queries: make(map[int64]string),  // qid -> question text
		corpus:  make(map[int64]string),  // pid -> passage text
		answers: make(map[int64]string),  // aid -> answer text
		qrels:   make(map[int64][]int64), // qid -> list of pid
		qas:     make(map[int64]int64),   // qid -> aid
	}
	for _, qi := range queries {
		res.queries[qi.ID] = qi.Text
	}
	for _, ci := range corpus {
		res.corpus[ci.ID] = ci.Text
	}
	for _, ai := range answers {
		res.answers[ai.ID] = ai.Text
	}
	for _, ri := range qrels {
		res.qrels[ri.QID] = append(res.qrels[ri.QID], ri.PID)
	}
	for _, qi := range qas {
		res.qas[qi.QID] = qi.AID
	}
	return res
}

func validateDataset(ds dataset) error {
	if len(ds.queries) == 0 {
		return fmt.Errorf("%w: dataset has no queries", ErrInvalidDataset)
	}
	for qid := range ds.queries {
		if _, ok := ds.qas[qid]; !ok {
			return fmt.Errorf("%w: query %d has no answer", ErrInvalidDataset, qid)
		}
		pids, ok := ds.qrels[qid]
		if !ok || len(pids) == 0 {
			return fmt.Errorf("%w: query %d has no related passages", ErrInvalidDataset, qid)
		}
	}
	for qid, aid := range ds.qas {
		if _, ok := ds.queries[qid]; !ok {
			return fmt.Errorf("%w: answer maps unknown query %d", ErrInvalidDataset, qid)
		}
		if _, ok := ds.answers[aid]; !ok {
			return fmt.Errorf("%w: query %d references unknown answer %d", ErrInvalidDataset, qid, aid)
		}
	}
	for qid, pids := range ds.qrels {
		if _, ok := ds.queries[qid]; !ok {
			return fmt.Errorf("%w: qrel maps unknown query %d", ErrInvalidDataset, qid)
		}
		for _, pid := range pids {
			if _, ok := ds.corpus[pid]; !ok {
				return fmt.Errorf("%w: query %d references unknown passage %d", ErrInvalidDataset, qid, pid)
			}
		}
	}
	return nil
}

func canonicalDatasetHash(
	queries []TextInfo,
	corpus []TextInfo,
	answers []TextInfo,
	qrels []RelsInfo,
	qas []QaInfo,
) (string, error) {
	sort.Slice(queries, func(i, j int) bool { return queries[i].ID < queries[j].ID })
	sort.Slice(corpus, func(i, j int) bool { return corpus[i].ID < corpus[j].ID })
	sort.Slice(answers, func(i, j int) bool { return answers[i].ID < answers[j].ID })
	sort.Slice(qrels, func(i, j int) bool {
		if qrels[i].QID != qrels[j].QID {
			return qrels[i].QID < qrels[j].QID
		}
		return qrels[i].PID < qrels[j].PID
	})
	sort.Slice(qas, func(i, j int) bool {
		if qas[i].QID != qas[j].QID {
			return qas[i].QID < qas[j].QID
		}
		return qas[i].AID < qas[j].AID
	})

	h := sha256.New()
	for _, rows := range []any{queries, corpus, answers, qrels, qas} {
		encoded, err := json.Marshal(rows)
		if err != nil {
			return "", err
		}
		h.Write(encoded)
		h.Write([]byte{'\n'})
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// dataset represents the in-memory dataset structure
type dataset struct {
	queries map[int64]string  // qid -> question text
	corpus  map[int64]string  // pid -> passage text
	answers map[int64]string  // aid -> answer text
	qrels   map[int64][]int64 // qid -> list of related pids
	qas     map[int64]int64   // qid -> aid
}

// Iterate generates QA pairs from the dataset
func (d *dataset) Iterate() []*types.QAPair {
	var pairs []*types.QAPair

	for qid, question := range d.queries {
		// Get answer info
		aid, hasAnswer := d.qas[qid]
		answer := ""
		if hasAnswer {
			answer = d.answers[aid]
		}

		// Get related passages
		pids := d.qrels[qid]
		var pidStr []int
		for _, pid := range pids {
			pidStr = append(pidStr, int(pid))
		}
		var passages []string
		for _, pid := range pids {
			passages = append(passages, d.corpus[pid])
		}

		pairs = append(pairs, &types.QAPair{
			QID:      int(qid),
			Question: question,
			PIDs:     pidStr,
			Passages: passages,
			AID:      int(aid),
			Answer:   answer,
		})
	}

	sort.Slice(pairs, func(i, j int) bool { return pairs[i].QID < pairs[j].QID })
	return pairs
}

// GetContextForQID retrieves context passages for a given question ID
func (d *dataset) GetContextForQID(qid int64) ([]string, error) {
	pids, ok := d.qrels[qid]
	if !ok {
		return nil, errors.New("question ID not found")
	}

	var contextParts []string
	for _, pid := range pids {
		if text, exists := d.corpus[pid]; exists {
			contextParts = append(contextParts, text)
		}
	}

	return contextParts, nil
}

// PrintStats prints dataset statistics to the logger
func (d *dataset) PrintStats(ctx context.Context) {
	logger.Infof(ctx, "QA System Statistics:")
	logger.Infof(ctx, "- Total queries: %d", len(d.queries))
	logger.Infof(ctx, "- Total corpus passages: %d", len(d.corpus))
	logger.Infof(ctx, "- Total answers: %d", len(d.answers))

	// Calculate average passages per query
	totalRelations := 0
	for _, pids := range d.qrels {
		totalRelations += len(pids)
	}
	avgPassages := float64(totalRelations) / float64(len(d.qrels))
	logger.Infof(ctx, "- Average passages per query: %.2f", avgPassages)

	// Calculate coverage
	coveredQueries := len(d.qas)
	coverage := float64(coveredQueries) / float64(len(d.queries)) * 100
	logger.Infof(ctx, "- Answer coverage: %.2f%% (%d/%d)", coverage, coveredQueries, len(d.queries))
}

// PrintRandomQA prints a random question with its related passages and answer
func (d *dataset) PrintRandomQA() error {
	// Get a random qid
	var qid int64
	for k := range d.qas {
		qid = k
		break
	}
	if qid == 0 {
		return errors.New("no questions available")
	}

	// Get question text
	question, ok := d.queries[qid]
	if !ok {
		return fmt.Errorf("question %d not found", qid)
	}

	// Get answer info
	aid, ok := d.qas[qid]
	if !ok {
		return fmt.Errorf("answer for question %d not found", qid)
	}
	answer, ok := d.answers[aid]
	if !ok {
		return fmt.Errorf("answer %d not found", aid)
	}

	// Print formatted QA
	fmt.Println("===== Random QA =====")
	fmt.Printf("QID: %d\n", qid)
	fmt.Printf("Question: %s\n", question)

	// Print passages if available
	if pids, exists := d.qrels[qid]; exists && len(pids) > 0 {
		fmt.Println("\nRelated passages:")
		for i, pid := range pids {
			if text, exists := d.corpus[pid]; exists {
				fmt.Printf("\nPassage %d (PID: %d):\n%s\n", i+1, pid, text)
			}
		}
	} else {
		fmt.Println("\nNo related passages found")
	}

	// Print answer
	fmt.Printf("\nAnswer (AID: %d):\n%s\n", aid, answer)

	return nil
}

// loadParquet loads data from parquet file into specified type
func loadParquet[T any](filePath string) ([]T, error) {
	rows, err := parquet.ReadFile[T](filePath)
	if err != nil {
		return nil, err
	}
	return rows, nil
}
