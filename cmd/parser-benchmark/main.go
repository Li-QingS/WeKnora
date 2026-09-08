package main

import (
	"context"
	"crypto/sha256"
	"encoding/csv"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"html"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/Tencent/WeKnora/internal/infrastructure/docparser"
	"github.com/Tencent/WeKnora/internal/types"
)

var benchmarkEngines = []string{
	docparser.BuiltinEngineName,
	docparser.SimpleEngineName,
	docparser.AnydocEngineName,
	docparser.WeKnoraCloudEngineName,
	docparser.MinerUEngineName,
	docparser.MinerUCloudEngineName,
	docparser.PaddleOCRVLEngineName,
	docparser.PaddleOCRVLCloudEngineName,
}

type goldFile struct {
	SchemaVersion  int              `json:"schema_version"`
	Fixture        string           `json:"fixture"`
	FixtureSHA256  string           `json:"fixture_sha256"`
	ContentPhrases []string         `json:"content_phrases"`
	Structure      []structureCheck `json:"structure_checks"`
}

type structureCheck struct {
	Name    string `json:"name"`
	Pattern string `json:"pattern"`
}

type engineResult struct {
	Engine             string   `json:"engine"`
	Available          bool     `json:"available"`
	Supported          bool     `json:"supported"`
	Status             string   `json:"status"`
	Reason             string   `json:"reason,omitempty"`
	DurationMS         int64    `json:"duration_ms"`
	ContentHits        int      `json:"content_hits,omitempty"`
	ContentTotal       int      `json:"content_total,omitempty"`
	StructureHits      int      `json:"structure_hits,omitempty"`
	StructureTotal     int      `json:"structure_total,omitempty"`
	ContentRecall      *float64 `json:"content_recall"`
	StructureRecall    *float64 `json:"structure_recall"`
	QualityScore       *float64 `json:"quality_score"`
	OutputBytes        int      `json:"output_bytes,omitempty"`
	OutputSHA256       string   `json:"output_sha256,omitempty"`
	RawOutput          string   `json:"raw_output,omitempty"`
	ParserMetadataName string   `json:"parser_metadata_name,omitempty"`
}

type benchmarkReport struct {
	SchemaVersion int            `json:"schema_version"`
	GeneratedAt   string         `json:"generated_at"`
	Commit        string         `json:"commit"`
	Fixture       string         `json:"fixture"`
	FixtureSHA256 string         `json:"fixture_sha256"`
	Gold          string         `json:"gold"`
	Formula       string         `json:"formula"`
	Engines       []engineResult `json:"engines"`
}

func main() {
	fixturePath := flag.String("fixture", "evaluation/parser_benchmark/fixture.pdf", "benchmark fixture")
	goldPath := flag.String("gold", "evaluation/parser_benchmark/gold.json", "gold definition")
	outDir := flag.String("out", "docs/evidence/parser-baseline-2026-09-08", "report output directory")
	docreaderAddr := flag.String("docreader", envOr("DOCREADER_ADDR", "127.0.0.1:50051"), "DocReader gRPC address")
	timeout := flag.Duration("timeout", 12*time.Minute, "timeout per parser")
	flag.Parse()

	if err := run(*fixturePath, *goldPath, *outDir, *docreaderAddr, *timeout); err != nil {
		fmt.Fprintln(os.Stderr, "parser benchmark:", err)
		os.Exit(1)
	}
}

func run(fixturePath, goldPath, outDir, docreaderAddr string, timeout time.Duration) error {
	gold, err := loadGold(goldPath)
	if err != nil {
		return err
	}
	fixture, err := os.ReadFile(fixturePath)
	if err != nil {
		return fmt.Errorf("read fixture: %w", err)
	}
	fixtureHash := sha256Hex(fixture)
	if !strings.EqualFold(fixtureHash, gold.FixtureSHA256) {
		return fmt.Errorf("fixture hash mismatch: got %s, gold requires %s", fixtureHash, gold.FixtureSHA256)
	}
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return fmt.Errorf("create output directory: %w", err)
	}

	overrides := parserOverridesFromEnv()
	remote, err := docparser.NewGRPCDocumentReader(docreaderAddr)
	if err != nil {
		return fmt.Errorf("connect DocReader: %w", err)
	}
	defer remote.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	remoteEngines, listErr := remote.ListEngines(ctx, overrides)
	cancel()
	if listErr != nil {
		return fmt.Errorf("list DocReader engines: %w", listErr)
	}
	metadata := docparser.ListAllEngines(remote.IsConnected(), overrides, remoteEngines)
	metaByName := make(map[string]types.ParserEngineInfo, len(metadata))
	for _, info := range metadata {
		metaByName[info.Name] = info
	}

	cloudID := strings.TrimSpace(os.Getenv("WEKNORA_CLOUD_APP_ID"))
	cloudSecret := strings.TrimSpace(os.Getenv("WEKNORA_CLOUD_APP_SECRET"))
	deps := docparser.ReaderDeps{
		Overrides: overrides,
		Remote:    remote,
		WeKnoraCloudCredentials: func(context.Context) *types.WeKnoraCloudCredentials {
			if cloudID == "" || cloudSecret == "" {
				return nil
			}
			return &types.WeKnoraCloudCredentials{AppID: cloudID, AppSecret: cloudSecret}
		},
	}

	report := benchmarkReport{
		SchemaVersion: 1,
		GeneratedAt:   time.Now().UTC().Format(time.RFC3339),
		Commit:        gitCommit(),
		Fixture:       filepath.ToSlash(fixturePath),
		FixtureSHA256: fixtureHash,
		Gold:          filepath.ToSlash(goldPath),
		Formula:       "quality_score = 0.8 * content_recall + 0.2 * structure_recall; unavailable/unsupported/failed scores are null",
		Engines:       make([]engineResult, 0, len(benchmarkEngines)),
	}

	for _, engine := range benchmarkEngines {
		info, ok := metaByName[engine]
		result := engineResult{Engine: engine, Status: "unavailable"}
		if !ok {
			result.Reason = "engine missing from application registry"
			report.Engines = append(report.Engines, result)
			continue
		}
		result.Available = info.Available
		result.Supported = slices.Contains(info.FileTypes, "pdf")
		if !result.Available {
			result.Reason = info.UnavailableReason
			report.Engines = append(report.Engines, result)
			continue
		}
		if !result.Supported {
			result.Status = "unsupported"
			result.Reason = "engine does not declare PDF support"
			report.Engines = append(report.Engines, result)
			continue
		}

		reader, buildErr := docparser.NewReader(context.Background(), engine, "pdf", false, deps)
		if buildErr != nil {
			result.Status = "unavailable"
			result.Reason = buildErr.Error()
			report.Engines = append(report.Engines, result)
			continue
		}
		parseCtx, parseCancel := context.WithTimeout(context.Background(), timeout)
		started := time.Now()
		parsed, parseErr := reader.Read(parseCtx, &types.ReadRequest{
			FileContent:           fixture,
			FileName:              filepath.Base(fixturePath),
			FileType:              "pdf",
			ParserEngine:          engine,
			ParserEngineOverrides: overrides,
			RequestID:             "parser-benchmark-" + engine,
		})
		result.DurationMS = time.Since(started).Milliseconds()
		parseCancel()
		if parseErr != nil {
			result.Status = "failed"
			result.Reason = parseErr.Error()
			report.Engines = append(report.Engines, result)
			continue
		}
		if parsed == nil {
			result.Status = "failed"
			result.Reason = "parser returned nil result"
			report.Engines = append(report.Engines, result)
			continue
		}
		if strings.TrimSpace(parsed.Error) != "" {
			result.Status = "failed"
			result.Reason = parsed.Error
			report.Engines = append(report.Engines, result)
			continue
		}

		result.Status = "success"
		result.ContentHits, result.ContentTotal, result.StructureHits, result.StructureTotal,
			result.ContentRecall, result.StructureRecall, result.QualityScore = score(parsed.MarkdownContent, gold)
		result.OutputBytes = len([]byte(parsed.MarkdownContent))
		result.OutputSHA256 = sha256Hex([]byte(parsed.MarkdownContent))
		if parsed.Metadata != nil {
			result.ParserMetadataName = parsed.Metadata["parser"]
		}
		rawName := "raw-" + strings.ReplaceAll(engine, "_", "-") + ".md"
		result.RawOutput = rawName
		if err := os.WriteFile(filepath.Join(outDir, rawName), []byte(parsed.MarkdownContent), 0o644); err != nil {
			return fmt.Errorf("write %s output: %w", engine, err)
		}
		report.Engines = append(report.Engines, result)
	}

	if err := writeReports(outDir, report); err != nil {
		return err
	}
	fmt.Printf("parser benchmark wrote %d engine rows to %s\n", len(report.Engines), outDir)
	return nil
}

func loadGold(path string) (goldFile, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return goldFile{}, fmt.Errorf("read gold: %w", err)
	}
	var gold goldFile
	if err := json.Unmarshal(data, &gold); err != nil {
		return goldFile{}, fmt.Errorf("parse gold: %w", err)
	}
	if gold.SchemaVersion != 1 || gold.FixtureSHA256 == "" || len(gold.ContentPhrases) == 0 || len(gold.Structure) == 0 {
		return goldFile{}, errors.New("invalid gold: schema_version=1, fixture hash, phrases and structure checks are required")
	}
	for _, check := range gold.Structure {
		if check.Name == "" || check.Pattern == "" {
			return goldFile{}, errors.New("invalid gold: structure check name and pattern are required")
		}
		if _, err := regexp.Compile(check.Pattern); err != nil {
			return goldFile{}, fmt.Errorf("invalid structure check %q: %w", check.Name, err)
		}
	}
	return gold, nil
}

func score(markdown string, gold goldFile) (contentHits, contentTotal, structureHits, structureTotal int, contentRecall, structureRecall, quality *float64) {
	normalized := strings.ToLower(strings.Join(strings.Fields(markdown), " "))
	for _, phrase := range gold.ContentPhrases {
		if strings.Contains(normalized, strings.ToLower(strings.Join(strings.Fields(phrase), " "))) {
			contentHits++
		}
	}
	contentTotal = len(gold.ContentPhrases)
	for _, check := range gold.Structure {
		if regexp.MustCompile(check.Pattern).MatchString(markdown) {
			structureHits++
		}
	}
	structureTotal = len(gold.Structure)
	cr := float64(contentHits) / float64(contentTotal)
	sr := float64(structureHits) / float64(structureTotal)
	qs := 0.8*cr + 0.2*sr
	return contentHits, contentTotal, structureHits, structureTotal, &cr, &sr, &qs
}

func writeReports(outDir string, report benchmarkReport) error {
	jsonData, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return err
	}
	jsonData = append(jsonData, '\n')
	if err := os.WriteFile(filepath.Join(outDir, "parser-baseline.json"), jsonData, 0o644); err != nil {
		return err
	}
	if err := writeCSV(filepath.Join(outDir, "parser-baseline.csv"), report.Engines); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(outDir, "parser-baseline.md"), []byte(renderMarkdown(report)), 0o644); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(outDir, "parser-baseline.svg"), []byte(renderSVG(report)), 0o644)
}

func writeCSV(path string, results []engineResult) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	w := csv.NewWriter(f)
	if err := w.Write([]string{"engine", "status", "available", "supported", "duration_ms", "content_recall", "structure_recall", "quality_score", "output_sha256", "reason"}); err != nil {
		return err
	}
	for _, r := range results {
		if err := w.Write([]string{r.Engine, r.Status, strconv.FormatBool(r.Available), strconv.FormatBool(r.Supported), strconv.FormatInt(r.DurationMS, 10), formatFloat(r.ContentRecall), formatFloat(r.StructureRecall), formatFloat(r.QualityScore), r.OutputSHA256, r.Reason}); err != nil {
			return err
		}
	}
	w.Flush()
	return w.Error()
}

func renderMarkdown(report benchmarkReport) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# 8 个解析引擎横向基线（2026-09-08）\n\n")
	fmt.Fprintf(&b, "- Commit：`%s`\n- Fixture：`%s`\n- SHA-256：`%s`\n- 评分：`%s`\n\n", report.Commit, report.Fixture, report.FixtureSHA256, report.Formula)
	b.WriteString("| 引擎 | 状态 | 耗时 ms | 内容 Recall | 结构 Recall | 综合分 | 说明 |\n|---|---:|---:|---:|---:|---:|---|\n")
	for _, r := range report.Engines {
		fmt.Fprintf(&b, "| `%s` | %s | %d | %s | %s | %s | %s |\n", r.Engine, r.Status, r.DurationMS, formatPercent(r.ContentRecall), formatPercent(r.StructureRecall), formatPercent(r.QualityScore), markdownCell(r.Reason))
	}
	b.WriteString("\n## 口径说明\n\n质量分只针对真实成功输出计算。`unavailable` 表示当前环境缺服务、凭据或构建能力；`unsupported` 表示引擎登记的能力不包含 PDF。二者均显示 N/A，不能解释为解析质量 0 分。该小型合成 PDF 用于建立可复查横向基线，不代表扫描件、中文复杂版面或超长文档的完整表现。\n")
	return b.String()
}

func renderSVG(report benchmarkReport) string {
	const width = 1000
	rowHeight := 58
	height := 120 + len(report.Engines)*rowHeight
	var b strings.Builder
	fmt.Fprintf(&b, `<svg xmlns="http://www.w3.org/2000/svg" width="%d" height="%d" viewBox="0 0 %d %d">`, width, height, width, height)
	b.WriteString(`<rect width="100%" height="100%" fill="#ffffff"/><style>text{font-family:Arial,"Noto Sans CJK SC",sans-serif;fill:#172033}.title{font-size:24px;font-weight:700}.label{font-size:15px}.value{font-size:14px;font-weight:700}.note{font-size:12px;fill:#667085}</style>`)
	b.WriteString(`<text x="30" y="38" class="title">8 个解析引擎横向基线</text><text x="30" y="64" class="note">综合分 = 80% 内容 Recall + 20% Markdown 结构 Recall；N/A 表示当前环境无真实输出</text>`)
	for i, r := range report.Engines {
		y := 96 + i*rowHeight
		fmt.Fprintf(&b, `<text x="30" y="%d" class="label">%s</text>`, y+21, html.EscapeString(r.Engine))
		b.WriteString(fmt.Sprintf(`<rect x="230" y="%d" width="700" height="24" rx="4" fill="#edf1f7"/>`, y))
		if r.QualityScore != nil {
			bar := int(*r.QualityScore * 700)
			fmt.Fprintf(&b, `<rect x="230" y="%d" width="%d" height="24" rx="4" fill="#377dff"/><text x="%d" y="%d" class="value">%.1f%%</text>`, y, bar, 240+bar, y+18, *r.QualityScore*100)
		} else {
			fmt.Fprintf(&b, `<text x="245" y="%d" class="note">N/A — %s</text>`, y+17, html.EscapeString(shortReason(r.Reason)))
		}
	}
	b.WriteString(`</svg>`)
	return b.String()
}

func parserOverridesFromEnv() map[string]string {
	mapping := map[string]string{
		"mineru_endpoint":                    "MINERU_ENDPOINT",
		"mineru_api_key":                     "MINERU_API_KEY",
		"mineru_model":                       "MINERU_MODEL",
		"mineru_vlm_server_url":              "MINERU_VLM_SERVER_URL",
		"mineru_parse_method":                "MINERU_PARSE_METHOD",
		"mineru_language":                    "MINERU_LANGUAGE",
		"mineru_cloud_model":                 "MINERU_CLOUD_MODEL",
		"paddleocr_vl_endpoint":              "PADDLEOCR_VL_ENDPOINT",
		"paddleocr_vl_cloud_token":           "PADDLEOCR_VL_CLOUD_TOKEN",
		"paddleocr_vl_cloud_model":           "PADDLEOCR_VL_CLOUD_MODEL",
		"paddleocr_vl_use_seal_recognition":  "PADDLEOCR_VL_USE_SEAL_RECOGNITION",
		"paddleocr_vl_use_chart_recognition": "PADDLEOCR_VL_USE_CHART_RECOGNITION",
	}
	overrides := make(map[string]string)
	for key, envName := range mapping {
		if value := strings.TrimSpace(os.Getenv(envName)); value != "" {
			overrides[key] = value
		}
	}
	if len(overrides) == 0 {
		return nil
	}
	return overrides
}

func gitCommit() string {
	out, err := exec.Command("git", "rev-parse", "--short=12", "HEAD").Output()
	if err != nil {
		return "unknown"
	}
	return strings.TrimSpace(string(out))
}

func sha256Hex(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func envOr(key, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(key)); value != "" {
		return value
	}
	return fallback
}

func formatFloat(value *float64) string {
	if value == nil {
		return ""
	}
	return fmt.Sprintf("%.6f", *value)
}

func formatPercent(value *float64) string {
	if value == nil {
		return "N/A"
	}
	return fmt.Sprintf("%.2f%%", *value*100)
}

func markdownCell(value string) string {
	value = strings.ReplaceAll(value, "|", "\\|")
	value = strings.ReplaceAll(value, "\n", " ")
	return strings.TrimSpace(value)
}

func shortReason(value string) string {
	value = strings.Join(strings.Fields(value), " ")
	if len(value) > 72 {
		return value[:69] + "..."
	}
	return value
}
