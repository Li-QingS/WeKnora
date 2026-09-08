#!/usr/bin/env python3
"""Build a reproducible Wiki prompt-cache report from exported call records."""

from __future__ import annotations

import argparse
import csv
import datetime as dt
import html
import json
import statistics
from pathlib import Path


SCOPES = (
    ("all_wiki", "全部 Wiki 调用"),
    ("wiki_page_modify", "页面生成"),
    ("wiki_chunk_citation", "引用抽取"),
)
RUN_ORDER = ("baseline_1", "baseline_2", "baseline_3", "baseline_4", "current_cold", "current_warm")


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser()
    parser.add_argument("--input", required=True, type=Path)
    parser.add_argument("--out", required=True, type=Path)
    parser.add_argument("--commit", required=True)
    parser.add_argument("--knowledge-base", required=True)
    parser.add_argument("--document", required=True)
    return parser.parse_args()


def load_rows(path: Path) -> list[dict[str, object]]:
    required = {
        "run_id",
        "phase",
        "created_at",
        "purpose",
        "status",
        "model_name",
        "prompt_tokens",
        "cache_read_tokens",
        "cache_write_tokens",
        "cache_miss_tokens",
        "duration_ms",
    }
    with path.open(encoding="utf-8", newline="") as handle:
        reader = csv.DictReader(handle)
        missing = required - set(reader.fieldnames or [])
        if missing:
            raise ValueError(f"missing CSV columns: {', '.join(sorted(missing))}")
        rows = []
        for raw in reader:
            row: dict[str, object] = dict(raw)
            for name in (
                "prompt_tokens",
                "cache_read_tokens",
                "cache_write_tokens",
                "cache_miss_tokens",
                "duration_ms",
            ):
                row[name] = int(str(raw[name]))
            rows.append(row)
    if not rows:
        raise ValueError("input CSV has no records")
    unknown = {str(row["run_id"]) for row in rows} - set(RUN_ORDER)
    if unknown:
        raise ValueError(f"unexpected run_id values: {', '.join(sorted(unknown))}")
    return rows


def summarize(rows: list[dict[str, object]]) -> list[dict[str, object]]:
    results = []
    for run_id in RUN_ORDER:
        run_rows = [row for row in rows if row["run_id"] == run_id]
        if not run_rows:
            raise ValueError(f"run {run_id} has no records")
        for scope, _ in SCOPES:
            selected = run_rows if scope == "all_wiki" else [row for row in run_rows if row["purpose"] == scope]
            if not selected:
                raise ValueError(f"run {run_id} has no records for {scope}")
            cache_read = sum(int(row["cache_read_tokens"]) for row in selected)
            cache_miss = sum(int(row["cache_miss_tokens"]) for row in selected)
            denominator = cache_read + cache_miss
            results.append(
                {
                    "run_id": run_id,
                    "phase": selected[0]["phase"],
                    "scope": scope,
                    "calls": len(selected),
                    "successful_calls": sum(row["status"] == "success" for row in selected),
                    "prompt_tokens": sum(int(row["prompt_tokens"]) for row in selected),
                    "cache_read_tokens": cache_read,
                    "cache_write_tokens": sum(int(row["cache_write_tokens"]) for row in selected),
                    "cache_miss_tokens": cache_miss,
                    "hit_rate": cache_read / denominator if denominator else 0.0,
                    "duration_ms": sum(int(row["duration_ms"]) for row in selected),
                    "started_at": min(str(row["created_at"]) for row in selected),
                    "finished_at": max(str(row["created_at"]) for row in selected),
                }
            )
    return results


def median_metric(items: list[dict[str, object]], field: str) -> float:
    return float(statistics.median(float(item[field]) for item in items))


def compare(summaries: list[dict[str, object]]) -> list[dict[str, object]]:
    comparisons = []
    for scope, label in SCOPES:
        scoped = [item for item in summaries if item["scope"] == scope]
        baseline = [item for item in scoped if str(item["run_id"]).startswith("baseline_")]
        cold = next(item for item in scoped if item["run_id"] == "current_cold")
        warm = next(item for item in scoped if item["run_id"] == "current_warm")
        base_rate = median_metric(baseline, "hit_rate")
        comparisons.append(
            {
                "scope": scope,
                "label": label,
                "baseline_run_count": len(baseline),
                "baseline_hit_rate_median": base_rate,
                "baseline_cache_read_tokens_median": median_metric(baseline, "cache_read_tokens"),
                "baseline_cache_miss_tokens_median": median_metric(baseline, "cache_miss_tokens"),
                "current_cold_hit_rate": cold["hit_rate"],
                "current_warm_hit_rate": warm["hit_rate"],
                "cold_delta_percentage_points": 100 * (float(cold["hit_rate"]) - base_rate),
                "warm_delta_percentage_points": 100 * (float(warm["hit_rate"]) - base_rate),
                "current_cold_cache_read_tokens": cold["cache_read_tokens"],
                "current_warm_cache_read_tokens": warm["cache_read_tokens"],
                "current_cold_cache_miss_tokens": cold["cache_miss_tokens"],
                "current_warm_cache_miss_tokens": warm["cache_miss_tokens"],
            }
        )
    return comparisons


def pct(value: object) -> str:
    return f"{float(value) * 100:.2f}%"


def render_markdown(report: dict[str, object]) -> str:
    metadata = report["metadata"]
    comparisons = report["comparison"]
    summaries = report["runs"]
    lines = [
        "# Wiki Prompt 缓存优化前后实测（2026-09-08）",
        "",
        f"- 当前代码：`{metadata['commit']}`",
        f"- 知识库：`{metadata['knowledge_base']}`",
        f"- 文档：`{metadata['document']}`",
        f"- 数据源：`{metadata['source_csv']}`（真实 `model_call_records` 脱敏导出）",
        "- 命中率：`cache_read_tokens / (cache_read_tokens + cache_miss_tokens)`",
        "- 优化前：Prompt 特定缓存调整前的 4 次历史运行中位数；优化后：最新代码连续冷、热两次运行。",
        "",
        "## 前后对比",
        "",
        "| 范围 | 优化前中位数 | 当前冷运行 | 冷运行变化 | 当前热运行 | 热运行变化 |",
        "|---|---:|---:|---:|---:|---:|",
    ]
    for item in comparisons:
        lines.append(
            f"| {item['label']} | {pct(item['baseline_hit_rate_median'])} | "
            f"{pct(item['current_cold_hit_rate'])} | {item['cold_delta_percentage_points']:+.2f} pp | "
            f"{pct(item['current_warm_hit_rate'])} | {item['warm_delta_percentage_points']:+.2f} pp |"
        )
    lines += [
        "",
        "## 逐轮原始聚合",
        "",
        "| 运行 | 阶段 | 范围 | 调用数 | cache read | cache miss | 命中率 | 总耗时 ms |",
        "|---|---|---|---:|---:|---:|---:|---:|",
    ]
    labels = dict(SCOPES)
    for item in summaries:
        lines.append(
            f"| `{item['run_id']}` | {item['phase']} | {labels[str(item['scope'])]} | "
            f"{item['calls']} | {item['cache_read_tokens']} | {item['cache_miss_tokens']} | "
            f"{pct(item['hit_rate'])} | {item['duration_ms']} |"
        )
    lines += [
        "",
        "## 结论边界",
        "",
        "表中的前后差异是同一知识库、同一文档在真实模型调用台账中的观测结果。模型服务端缓存是否命中仍受前缀稳定性、调用顺序和厂商缓存生命周期影响，因此逐轮值一并保留，不能把单次热运行外推为固定收益。",
        "",
    ]
    return "\n".join(lines)


def render_svg(comparisons: list[dict[str, object]]) -> str:
    width, height = 1080, 570
    plot_left, plot_top, plot_width, plot_height = 110, 105, 900, 340
    colors = ("#98A2B3", "#53B1FD", "#1570EF")
    keys = ("baseline_hit_rate_median", "current_cold_hit_rate", "current_warm_hit_rate")
    legend = ("优化前中位数", "当前冷运行", "当前热运行")
    parts = [
        f'<svg xmlns="http://www.w3.org/2000/svg" width="{width}" height="{height}" viewBox="0 0 {width} {height}">',
        '<rect width="100%" height="100%" fill="#fff"/>',
        '<style>text{font-family:Arial,"Noto Sans CJK SC",sans-serif;fill:#172033}.title{font-size:25px;font-weight:700}.axis{font-size:13px;fill:#667085}.value{font-size:12px;font-weight:700}.label{font-size:15px;font-weight:600}.note{font-size:12px;fill:#667085}</style>',
        '<text x="36" y="42" class="title">Wiki Prompt 缓存优化前后实测</text>',
        '<text x="36" y="68" class="note">命中率 = cache read / (cache read + cache miss)；优化前为 4 次运行中位数</text>',
    ]
    for tick in range(0, 41, 10):
        y = plot_top + plot_height - tick / 40 * plot_height
        parts.append(f'<line x1="{plot_left}" y1="{y:.1f}" x2="{plot_left + plot_width}" y2="{y:.1f}" stroke="#E4E7EC"/>')
        parts.append(f'<text x="{plot_left - 16}" y="{y + 4:.1f}" text-anchor="end" class="axis">{tick}%</text>')
    group_width = plot_width / len(comparisons)
    bar_width = 58
    gap = 12
    for group_index, item in enumerate(comparisons):
        center = plot_left + group_width * (group_index + 0.5)
        total_bars_width = 3 * bar_width + 2 * gap
        start_x = center - total_bars_width / 2
        for bar_index, key in enumerate(keys):
            value = float(item[key])
            capped = min(value, 0.4)
            bar_height = capped / 0.4 * plot_height
            x = start_x + bar_index * (bar_width + gap)
            y = plot_top + plot_height - bar_height
            parts.append(f'<rect x="{x:.1f}" y="{y:.1f}" width="{bar_width}" height="{bar_height:.1f}" rx="5" fill="{colors[bar_index]}"/>')
            parts.append(f'<text x="{x + bar_width / 2:.1f}" y="{max(plot_top + 13, y - 7):.1f}" text-anchor="middle" class="value">{value * 100:.1f}%</text>')
        parts.append(f'<text x="{center:.1f}" y="{plot_top + plot_height + 31}" text-anchor="middle" class="label">{html.escape(str(item["label"]))}</text>')
    legend_y = 522
    for index, label in enumerate(legend):
        x = 250 + index * 220
        parts.append(f'<rect x="{x}" y="{legend_y - 13}" width="18" height="18" rx="3" fill="{colors[index]}"/>')
        parts.append(f'<text x="{x + 27}" y="{legend_y + 1}" class="axis">{label}</text>')
    parts.append("</svg>")
    return "".join(parts)


def main() -> None:
    args = parse_args()
    rows = load_rows(args.input)
    summaries = summarize(rows)
    comparisons = compare(summaries)
    report: dict[str, object] = {
        "schema_version": 1,
        "metadata": {
            "generated_at": dt.datetime.now(dt.timezone.utc).isoformat(),
            "commit": args.commit,
            "knowledge_base": args.knowledge_base,
            "document": args.document,
            "source_csv": args.input.as_posix(),
            "formula": "cache_read_tokens / (cache_read_tokens + cache_miss_tokens)",
            "baseline_definition": "median of baseline_1..baseline_4 before prompt-specific cache changes",
            "current_definition": "consecutive cold and warm reruns on the current merged code",
        },
        "comparison": comparisons,
        "runs": summaries,
    }
    args.out.mkdir(parents=True, exist_ok=True)
    (args.out / "wiki-cache-summary.json").write_text(json.dumps(report, ensure_ascii=False, indent=2) + "\n", encoding="utf-8")
    (args.out / "wiki-cache-report.md").write_text(render_markdown(report), encoding="utf-8")
    (args.out / "wiki-cache-comparison.svg").write_text(render_svg(comparisons), encoding="utf-8")
    print(f"wrote Wiki cache report to {args.out}")


if __name__ == "__main__":
    main()
