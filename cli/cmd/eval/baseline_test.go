package evalcmd

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Tencent/WeKnora/cli/internal/cmdutil"
	"github.com/Tencent/WeKnora/cli/internal/iostreams"
)

func writeCompareResultFile(t *testing.T, dir string) string {
	t.Helper()
	path := filepath.Join(dir, "evaluation-result.json")
	data, err := json.Marshal(compareResultReport(0.95))
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}
	return path
}

func baselineCreateArgs(resultPath string, output string, extra ...string) []string {
	args := []string{
		"--result", resultPath,
		"--output", output,
		"--approved-by", "lqs",
		"--approved-commit", "abc123",
	}
	args = append(args, extra...)
	return args
}

func TestBaselineCreateSuccess(t *testing.T) {
	out, _ := iostreams.SetForTest(t)
	dir := t.TempDir()
	resultPath := writeCompareResultFile(t, dir)
	output := filepath.Join(dir, "baseline.yaml")

	cmd := NewCmdBaselineCreate()
	cmd.SetArgs(baselineCreateArgs(resultPath, output))
	if err := cmd.Execute(); err != nil {
		t.Fatalf("baseline create: %v", err)
	}
	data, err := os.ReadFile(output)
	if err != nil {
		t.Fatalf("read baseline: %v", err)
	}
	if !strings.Contains(string(data), "hash123") {
		t.Errorf("baseline=%s, want config_hash hash123", string(data))
	}
	if !strings.Contains(out.String(), `"approved_by":"lqs"`) {
		t.Errorf("output=%s, want approved_by", out.String())
	}
}

func TestBaselineCreateMissingApproval(t *testing.T) {
	_, _ = iostreams.SetForTest(t)
	dir := t.TempDir()
	resultPath := writeCompareResultFile(t, dir)
	cmd := NewCmdBaselineCreate()
	cmd.SetArgs([]string{
		"--result", resultPath,
		"--output", filepath.Join(dir, "baseline.yaml"),
		"--approved-by", "lqs",
	})
	err := cmd.Execute()
	var ce *cmdutil.Error
	if !errors.As(err, &ce) || ce.Code != cmdutil.CodeEvalConfigError {
		t.Fatalf("err=%v, want eval.config_error", err)
	}
}

func TestBaselineCreateRejectsOverwriteWithoutForce(t *testing.T) {
	_, _ = iostreams.SetForTest(t)
	dir := t.TempDir()
	resultPath := writeCompareResultFile(t, dir)
	output := filepath.Join(dir, "baseline.yaml")

	first := NewCmdBaselineCreate()
	first.SetArgs(baselineCreateArgs(resultPath, output))
	if err := first.Execute(); err != nil {
		t.Fatalf("first create: %v", err)
	}

	second := NewCmdBaselineCreate()
	second.SetArgs(baselineCreateArgs(resultPath, output))
	err := second.Execute()
	var ce *cmdutil.Error
	if !errors.As(err, &ce) || ce.Code != cmdutil.CodeEvalConfigError {
		t.Fatalf("err=%v, want eval.config_error", err)
	}

	third := NewCmdBaselineCreate()
	third.SetArgs(baselineCreateArgs(resultPath, output, "--force"))
	if err := third.Execute(); err != nil {
		t.Fatalf("force create: %v", err)
	}
}

func TestBaselineCreateDryRunDoesNotWrite(t *testing.T) {
	_, _ = iostreams.SetForTest(t)
	dir := t.TempDir()
	resultPath := writeCompareResultFile(t, dir)
	output := filepath.Join(dir, "baseline.yaml")

	cmd := NewCmdBaselineCreate()
	cmd.SetArgs(baselineCreateArgs(resultPath, output, "--dry-run"))
	if err := cmd.Execute(); err != nil {
		t.Fatalf("dry-run: %v", err)
	}
	if _, err := os.Stat(output); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("dry-run should not write baseline, err=%v", err)
	}
}
