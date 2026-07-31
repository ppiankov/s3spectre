# Guides

Task-oriented walkthroughs, as opposed to the flag-by-flag
[CLI Reference](../cli-reference.md). Start with the [Quick Start](../QUICKSTART.md)
if you haven't run s3spectre yet; come here once you're looking at real output and
want to understand it or act on it.

- [Understanding discover's risk scores](understanding-risk-scores.md) — how the
  factors add up, how status classification works, and how to tune the threshold
- [Finding which team to talk to first](team-ownership-rollup.md) —
  `--group-by-tag` and why the average matters as much as the sum
- [From "this looks wasteful" to a reviewable fix](cost-and-cleanup-workflow.md) —
  `--estimate-cost` and `--suggest-lifecycle-policy` end to end
- [Scan and discover answer different questions](scan-plus-discover.md) — when to
  use which mode, and a worked example of what happens when you use both
- [Surfacing scan findings as inline PR annotations](ci-sarif-annotations.md) —
  wiring `--format sarif` into GitHub's code-scanning ingestion, and why only
  `scan` (not `discover`) findings can get an inline annotation
