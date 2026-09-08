#!/usr/bin/env python3
"""Copy an evaluation result and set retrieval Recall for CI evidence runs."""

from __future__ import annotations

import argparse
import json
from pathlib import Path


def main() -> None:
    parser = argparse.ArgumentParser()
    parser.add_argument("--input", required=True, type=Path)
    parser.add_argument("--output", required=True, type=Path)
    parser.add_argument("--recall", required=True, type=float)
    args = parser.parse_args()

    if not 0 <= args.recall <= 1:
        parser.error("--recall must be between 0 and 1")
    payload = json.loads(args.input.read_text(encoding="utf-8"))
    metrics = payload.get("metric", {}).get("retrieval_metrics", {})
    if "recall" not in metrics:
        raise ValueError("input has no metric.retrieval_metrics.recall")
    previous = float(metrics["recall"])
    metrics["recall"] = args.recall
    payload["run_id"] = f"{payload.get('run_id', 'evaluation')}_recall_{args.recall:.2f}"
    args.output.parent.mkdir(parents=True, exist_ok=True)
    args.output.write_text(json.dumps(payload, ensure_ascii=False, indent=2) + "\n", encoding="utf-8")
    print(f"recall changed from {previous:.4f} to {args.recall:.4f}: {args.output}")


if __name__ == "__main__":
    main()
