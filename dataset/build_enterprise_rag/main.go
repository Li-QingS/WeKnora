// Command build-enterprise-rag rebuilds the EnterpriseRAG-Bench sample used
// by the evaluation runner.
//
// Usage:
//
//	go run ./dataset/build_enterprise_rag \
//		-manifest dataset/enterprise_rag/manifest.json \
//		-source-root /path/to/EnterpriseRAG-Bench \
//		-output-dir dataset/enterprise_rag
//
// The source root must contain generated_data/sources/<doc path> as published
// by github.com/onyx-dot-app/EnterpriseRAG-Bench.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/parquet-go/parquet-go"
)

type textInfo struct {
	ID   int64  `parquet:"id"`
	Text string `parquet:"text"`
}

type relsInfo struct {
	QID int64 `parquet:"qid"`
	PID int64 `parquet:"pid"`
}

type qaInfo struct {
	QID int64 `parquet:"qid"`
	AID int64 `parquet:"aid"`
}

type enterpriseManifest struct {
	Source    string               `json:"source"`
	SourceURL string               `json:"source_url"`
	Questions []enterpriseQuestion `json:"questions"`
}

type enterpriseQuestion struct {
	ID           string             `json:"id"`
	Type         string             `json:"question_type"`
	SourceTypes  []string           `json:"source_types"`
	Question     string             `json:"question"`
	GoldAnswer   string             `json:"gold_answer"`
	ExpectedDocs []enterpriseDocRef `json:"docs"`
}

type enterpriseDocRef struct {
	ID     string `json:"id"`
	Source string `json:"source"`
	Path   string `json:"path"`
}

func main() {
	manifestPath := flag.String("manifest", "dataset/enterprise_rag/manifest.json", "path to selection manifest")
	sourceRoot := flag.String("source-root", ".", "EnterpriseRAG-Bench repository root")
	outputDir := flag.String("output-dir", "dataset/enterprise_rag", "output dataset directory")
	flag.Parse()

	manifest, err := loadManifest(*manifestPath)
	if err != nil {
		panic(err)
	}
	if err := os.MkdirAll(*outputDir, 0o755); err != nil {
		panic(err)
	}

	questions := append([]enterpriseQuestion(nil), manifest.Questions...)
	sort.Slice(questions, func(i, j int) bool { return questions[i].ID < questions[j].ID })

	docByID := make(map[string]enterpriseDocRef)
	for _, q := range questions {
		for _, doc := range q.ExpectedDocs {
			docByID[doc.ID] = doc
		}
	}
	docRefs := make([]enterpriseDocRef, 0, len(docByID))
	for _, doc := range docByID {
		docRefs = append(docRefs, doc)
	}
	sort.Slice(docRefs, func(i, j int) bool {
		if docRefs[i].Source != docRefs[j].Source {
			return docRefs[i].Source < docRefs[j].Source
		}
		return docRefs[i].Path < docRefs[j].Path
	})

	pidByDocID := make(map[string]int64, len(docRefs))
	corpus := make([]textInfo, 0, len(docRefs))
	for i, doc := range docRefs {
		pid := int64(i + 1)
		pidByDocID[doc.ID] = pid
		text, err := readEnterpriseDocText(*sourceRoot, doc)
		if err != nil {
			panic(err)
		}
		corpus = append(corpus, textInfo{ID: pid, Text: text})
	}

	queries := make([]textInfo, 0, len(questions))
	answers := make([]textInfo, 0, len(questions))
	qrels := make([]relsInfo, 0, len(questions))
	qas := make([]qaInfo, 0, len(questions))
	for i, q := range questions {
		qid := int64(i + 1)
		aid := int64(i + 1)
		queries = append(queries, textInfo{ID: qid, Text: q.Question})
		answers = append(answers, textInfo{ID: aid, Text: q.GoldAnswer})
		qas = append(qas, qaInfo{QID: qid, AID: aid})

		pids := make([]int64, 0, len(q.ExpectedDocs))
		for _, doc := range q.ExpectedDocs {
			pid, ok := pidByDocID[doc.ID]
			if !ok {
				panic(fmt.Sprintf("question %s references missing doc %s", q.ID, doc.ID))
			}
			pids = append(pids, pid)
		}
		sort.Slice(pids, func(i, j int) bool { return pids[i] < pids[j] })
		for _, pid := range pids {
			qrels = append(qrels, relsInfo{QID: qid, PID: pid})
		}
	}

	writeFiles(*outputDir, queries, corpus, answers, qrels, qas)
	fmt.Printf("Generated EnterpriseRAG-Bench sample in %s\n", *outputDir)
	fmt.Printf("queries=%d corpus=%d answers=%d qrels=%d qas=%d\n",
		len(queries), len(corpus), len(answers), len(qrels), len(qas))
}

func loadManifest(path string) (*enterpriseManifest, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read manifest %s: %w", path, err)
	}
	var manifest enterpriseManifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		return nil, fmt.Errorf("parse manifest %s: %w", path, err)
	}
	if len(manifest.Questions) == 0 {
		return nil, fmt.Errorf("manifest %s has no questions", path)
	}
	return &manifest, nil
}

func readEnterpriseDocText(sourceRoot string, doc enterpriseDocRef) (string, error) {
	relPath := filepath.Join("generated_data", "sources", filepath.FromSlash(doc.Path))
	fullPath := filepath.Join(sourceRoot, relPath)
	data, err := os.ReadFile(fullPath)
	if err != nil {
		return "", fmt.Errorf("read doc %s: %w", fullPath, err)
	}
	var raw map[string]any
	if err := json.Unmarshal(data, &raw); err != nil {
		return "", fmt.Errorf("parse doc %s: %w", fullPath, err)
	}
	return renderEnterpriseDoc(raw, doc.Source), nil
}

func renderEnterpriseDoc(doc map[string]any, source string) string {
	var b strings.Builder
	titleField, _ := doc["title_field_name"].(string)
	if titleField != "" {
		if title, ok := doc[titleField].(string); ok && strings.TrimSpace(title) != "" {
			fmt.Fprintf(&b, "# %s\n", strings.TrimSpace(title))
		}
	}
	fmt.Fprintf(&b, "source_type: %s\n", source)

	contentFields := stringSlice(doc["content_field_names"])
	if len(contentFields) == 0 {
		contentFields = []string{"content", "description", "body", "messages", "notes", "summary"}
	}
	seen := make(map[string]bool)
	for _, field := range contentFields {
		if field == "" || seen[field] {
			continue
		}
		seen[field] = true
		value, ok := doc[field]
		if !ok || isEmptyValue(value) {
			continue
		}
		fmt.Fprintf(&b, "\n## %s\n\n", strings.ReplaceAll(field, "_", " "))
		b.WriteString(renderValue(value))
		b.WriteString("\n")
	}
	return strings.TrimSpace(b.String())
}

func stringSlice(v any) []string {
	raw, ok := v.([]any)
	if !ok {
		return nil
	}
	out := make([]string, 0, len(raw))
	for _, item := range raw {
		if s, ok := item.(string); ok {
			out = append(out, s)
		}
	}
	return out
}

func isEmptyValue(v any) bool {
	switch x := v.(type) {
	case nil:
		return true
	case string:
		return strings.TrimSpace(x) == ""
	case []any:
		return len(x) == 0
	case map[string]any:
		return len(x) == 0
	default:
		return false
	}
}

func renderValue(v any) string {
	switch x := v.(type) {
	case string:
		return strings.TrimSpace(x)
	case float64:
		return fmt.Sprintf("%v", x)
	case bool:
		return fmt.Sprintf("%v", x)
	case []any:
		var b strings.Builder
		for i, item := range x {
			if i > 0 {
				b.WriteString("\n")
			}
			if m, ok := item.(map[string]any); ok {
				b.WriteString(renderMapItem(m))
			} else {
				b.WriteString("- ")
				b.WriteString(renderValue(item))
			}
		}
		return b.String()
	case map[string]any:
		return renderMapItem(x)
	default:
		return fmt.Sprintf("%v", x)
	}
}

func renderMapItem(m map[string]any) string {
	keys := make([]string, 0, len(m))
	for key := range m {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	var b strings.Builder
	for _, key := range keys {
		value := m[key]
		if isEmptyValue(value) || isMetadataKey(key) {
			continue
		}
		label := strings.ReplaceAll(key, "_", " ")
		if scalar, ok := value.(string); ok {
			if strings.TrimSpace(scalar) == "" {
				continue
			}
			fmt.Fprintf(&b, "%s: %s\n", label, strings.TrimSpace(scalar))
			continue
		}
		if number, ok := value.(float64); ok {
			fmt.Fprintf(&b, "%s: %v\n", label, number)
			continue
		}
		if flag, ok := value.(bool); ok {
			fmt.Fprintf(&b, "%s: %v\n", label, flag)
			continue
		}
		fmt.Fprintf(&b, "%s:\n", label)
		b.WriteString(indentBlock(renderValue(value)))
	}
	return strings.TrimRight(b.String(), "\n")
}

func isMetadataKey(key string) bool {
	switch key {
	case "title_field_name", "content_field_names", "dataset_doc_uuid", "original_location",
		"linked_artifacts", "related_links", "attachments", "reviewers", "labels", "participants",
		"collaborators", "related_pages", "linked_issues", "linked_github_prs",
		"linked_confluence_pages", "linked_fireflies", "linked_gmail_threads",
		"linked_drive_docs", "linked_support_tickets", "review_threads":
		return true
	default:
		return false
	}
}

func indentBlock(s string) string {
	if s == "" {
		return ""
	}
	lines := strings.Split(s, "\n")
	for i, line := range lines {
		lines[i] = "  " + line
	}
	return strings.Join(lines, "\n") + "\n"
}

func writeFiles(
	dir string,
	queries []textInfo,
	corpus []textInfo,
	answers []textInfo,
	qrels []relsInfo,
	qas []qaInfo,
) {
	writeParquet(dir, "queries.parquet", queries)
	writeParquet(dir, "corpus.parquet", corpus)
	writeParquet(dir, "answers.parquet", answers)
	writeParquet(dir, "qrels.parquet", qrels)
	writeParquet(dir, "qas.parquet", qas)
}

func writeParquet[T any](dir, name string, rows []T) {
	path := filepath.Join(dir, name)
	if err := parquet.WriteFile(path, rows); err != nil {
		panic(fmt.Sprintf("write %s: %v", path, err))
	}
}
