package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/Tencent/WeKnora/internal/models/embedding"
	"github.com/parquet-go/parquet-go"
)

type textRow struct {
	ID   int64  `parquet:"id"`
	Text string `parquet:"text"`
}

type phaseMetric struct {
	Name     string  `json:"name"`
	Requests int     `json:"requests"`
	Hits     int     `json:"hits"`
	Misses   int     `json:"misses"`
	HitRate  float64 `json:"hit_rate"`
}

type strategyMetric struct {
	Name           string        `json:"name"`
	Requests       int           `json:"requests"`
	Hits           int           `json:"hits"`
	Misses         int           `json:"misses"`
	HitRate        float64       `json:"hit_rate"`
	ProviderInputs int           `json:"provider_inputs"`
	Phases         []phaseMetric `json:"phases"`
}

type benchmarkReport struct {
	GeneratedAt            time.Time      `json:"generated_at"`
	Dataset                string         `json:"dataset"`
	BaseInputs             int            `json:"base_inputs"`
	CorpusInputs           int            `json:"corpus_inputs"`
	QueryInputs            int            `json:"query_inputs"`
	WorkloadSHA256         string         `json:"workload_sha256"`
	Scenario               string         `json:"scenario"`
	Baseline               strategyMetric `json:"baseline"`
	Optimized              strategyMetric `json:"optimized"`
	HitRateGainPoints      float64        `json:"hit_rate_gain_points"`
	ProviderInputsSaved    int            `json:"provider_inputs_saved"`
	ProviderInputReduction float64        `json:"provider_input_reduction"`
}

type workloadPhase struct {
	name   string
	inputs []string
}

func main() {
	datasetDir := flag.String("dataset", "dataset/enterprise_rag", "dataset directory")
	outputDir := flag.String("output-dir", "docs/evidence/embedding-cache-hit-rate-2026-09-10", "report output directory")
	flag.Parse()

	corpus := mustRead(filepath.Join(*datasetDir, "corpus.parquet"))
	queries := mustRead(filepath.Join(*datasetDir, "queries.parquet"))
	base := make([]string, 0, len(corpus)+len(queries))
	for _, row := range corpus {
		base = append(base, row.Text)
	}
	for _, row := range queries {
		base = append(base, row.Text)
	}
	drifted := make([]string, len(base))
	for i, text := range base {
		drifted[i] = editorFormattingDrift(text)
		if embedding.CanonicalizeCacheText(drifted[i]) != embedding.CanonicalizeCacheText(text) {
			panic(fmt.Sprintf("format drift changed canonical input %d", i))
		}
	}
	phases := []workloadPhase{
		{name: "cold_import", inputs: base},
		{name: "exact_replay", inputs: base},
		{name: "editor_format_drift_replay", inputs: drifted},
	}
	baseline := simulate("raw exact key", phases, func(text string) string { return text })
	optimized := simulate("conservative canonical key", phases, embedding.CanonicalizeCacheText)
	report := benchmarkReport{
		GeneratedAt:            time.Now().UTC(),
		Dataset:                *datasetDir,
		BaseInputs:             len(base),
		CorpusInputs:           len(corpus),
		QueryInputs:            len(queries),
		WorkloadSHA256:         workloadHash(phases),
		Scenario:               "cold import, exact replay, then BOM/CRLF/line-tail/outer-whitespace drift replay",
		Baseline:               baseline,
		Optimized:              optimized,
		HitRateGainPoints:      (optimized.HitRate - baseline.HitRate) * 100,
		ProviderInputsSaved:    baseline.ProviderInputs - optimized.ProviderInputs,
		ProviderInputReduction: ratio(baseline.ProviderInputs-optimized.ProviderInputs, baseline.ProviderInputs),
	}
	if err := os.MkdirAll(*outputDir, 0o755); err != nil {
		panic(err)
	}
	jsonData, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		panic(err)
	}
	if err := os.WriteFile(filepath.Join(*outputDir, "benchmark.json"), append(jsonData, '\n'), 0o644); err != nil {
		panic(err)
	}
	if err := os.WriteFile(filepath.Join(*outputDir, "benchmark.md"), []byte(renderMarkdown(report)), 0o644); err != nil {
		panic(err)
	}
	fmt.Print(renderMarkdown(report))
}

func mustRead(path string) []textRow {
	rows, err := parquet.ReadFile[textRow](path)
	if err != nil {
		panic(err)
	}
	return rows
}

func editorFormattingDrift(text string) string {
	canonical := embedding.CanonicalizeCacheText(text)
	lines := strings.Split(canonical, "\n")
	for i := range lines {
		lines[i] += " \t"
	}
	return " \t\ufeff" + strings.Join(lines, "\r\n") + "\r\n \t"
}

func simulate(name string, phases []workloadPhase, key func(string) string) strategyMetric {
	seen := make(map[string]struct{})
	metric := strategyMetric{Name: name}
	for _, phase := range phases {
		phaseResult := phaseMetric{Name: phase.name, Requests: len(phase.inputs)}
		for _, input := range phase.inputs {
			cacheKey := key(input)
			if _, ok := seen[cacheKey]; ok {
				metric.Hits++
				phaseResult.Hits++
			} else {
				seen[cacheKey] = struct{}{}
				metric.Misses++
				phaseResult.Misses++
			}
		}
		phaseResult.HitRate = ratio(phaseResult.Hits, phaseResult.Requests)
		metric.Phases = append(metric.Phases, phaseResult)
	}
	metric.Requests = metric.Hits + metric.Misses
	metric.HitRate = ratio(metric.Hits, metric.Requests)
	metric.ProviderInputs = metric.Misses
	return metric
}

func ratio(numerator, denominator int) float64 {
	if denominator == 0 {
		return 0
	}
	return float64(numerator) / float64(denominator)
}

func workloadHash(phases []workloadPhase) string {
	hash := sha256.New()
	for _, phase := range phases {
		hash.Write([]byte(phase.name))
		hash.Write([]byte{0})
		for _, input := range phase.inputs {
			hash.Write([]byte(input))
			hash.Write([]byte{0})
		}
	}
	return hex.EncodeToString(hash.Sum(nil))
}

func renderMarkdown(report benchmarkReport) string {
	var builder strings.Builder
	fmt.Fprintf(&builder, "# Embedding cache hit-rate benchmark\n\n")
	fmt.Fprintf(&builder, "- Dataset: `%s` (%d corpus + %d queries = %d base inputs)\n", report.Dataset, report.CorpusInputs, report.QueryInputs, report.BaseInputs)
	fmt.Fprintf(&builder, "- Scenario: %s\n", report.Scenario)
	fmt.Fprintf(&builder, "- Workload SHA-256: `%s`\n\n", report.WorkloadSHA256)
	fmt.Fprintf(&builder, "| Strategy | Requests | Hits | Misses / provider inputs | Hit rate |\n")
	fmt.Fprintf(&builder, "|---|---:|---:|---:|---:|\n")
	fmt.Fprintf(&builder, "| Baseline raw key | %d | %d | %d | %.2f%% |\n", report.Baseline.Requests, report.Baseline.Hits, report.Baseline.Misses, report.Baseline.HitRate*100)
	fmt.Fprintf(&builder, "| Conservative canonical key | %d | %d | %d | %.2f%% |\n\n", report.Optimized.Requests, report.Optimized.Hits, report.Optimized.Misses, report.Optimized.HitRate*100)
	fmt.Fprintf(&builder, "Hit rate gain: **+%.2f percentage points**. Provider inputs saved: **%d (%.2f%%)**.\n\n", report.HitRateGainPoints, report.ProviderInputsSaved, report.ProviderInputReduction*100)
	fmt.Fprintf(&builder, "This is a controlled formatting-drift benchmark over committed project data. It does not claim that future production traffic has the same mix.\n")
	return builder.String()
}
