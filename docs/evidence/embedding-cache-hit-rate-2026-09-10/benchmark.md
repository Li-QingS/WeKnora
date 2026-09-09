# Embedding cache hit-rate benchmark

- Dataset: `dataset/enterprise_rag` (85 corpus + 50 queries = 135 base inputs)
- Scenario: cold import, exact replay, then BOM/CRLF/line-tail/outer-whitespace drift replay
- Workload SHA-256: `da0587331aeddc0c3423847e0ac8fbcaa5bb03626ba06e20b5347ead97059a0c`

| Strategy | Requests | Hits | Misses / provider inputs | Hit rate |
|---|---:|---:|---:|---:|
| Baseline raw key | 405 | 135 | 270 | 33.33% |
| Conservative canonical key | 405 | 270 | 135 | 66.67% |

Hit rate gain: **+33.33 percentage points**. Provider inputs saved: **135 (50.00%)**.

This is a controlled formatting-drift benchmark over committed project data. It does not claim that future production traffic has the same mix.
