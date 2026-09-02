# EnterpriseRAG-Bench sample (`enterprise_rag`)

Evaluation dataset for the WeKnora reproducible runner. It is a small,
deterministic subset of the public [EnterpriseRAG-Bench](https://github.com/onyx-dot-app/EnterpriseRAG-Bench)
dataset (MIT license).

## Contents

- 50 QA pairs with official questions and gold answers;
- 85 source documents from Slack, Gmail, Linear, Google Drive, HubSpot,
  Fireflies, GitHub, Jira, and Confluence;
- question types: basic (16), semantic (12), intra-document reasoning (6),
  project related (6), constrained (4), conflicting info (2), completeness (2),
  miscellaneous (2).

The official "high level" and "info not found" categories have no ground-truth
documents, so they are excluded. Selection uses a deterministic seed
(`20260902`); the exact selection is recorded in `manifest.json`.

Each corpus row is one official source document rendered from its
`title_field_name` and `content_field_names`, so the Parquet files stay the same
shape as `dataset/demo` and `dataset/samples` while preserving the enterprise
document content.

## Regenerate

The Parquet files are committed and work out of the box. To rebuild them from
the upstream repository:

```bash
git clone --depth 1 https://github.com/onyx-dot-app/EnterpriseRAG-Bench.git /tmp/EnterpriseRAG-Bench
go run ./dataset/build_enterprise_rag \
  -manifest dataset/enterprise_rag/manifest.json \
  -source-root /tmp/EnterpriseRAG-Bench \
  -output-dir dataset/enterprise_rag
```

Run it with:

```bash
WEKNORA_HOST=http://localhost:8080 WEKNORA_TOKEN=<token> \
  make eval-baseline CONFIG=./evaluation/configs/enterprise.yaml
```
