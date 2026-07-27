# Understanding discover's risk scores

`s3spectre discover` doesn't just say "this bucket is bad" — it adds up independent
risk factors into a score, then classifies the bucket once that score crosses a
threshold. This guide walks through how the score is built and how to read it, so a
number like `RiskScore: 175` tells you something concrete instead of just "high."

## The factors

Each factor is scored independently and all that apply get added together:

| Factor | Points | Condition |
|---|---|---|
| Old bucket | 20 | Older than `--age-threshold-days` (default 365) |
| Inactivity | 50 / 75 / 100 | No activity past the threshold / 2x / 5x the threshold |
| Empty bucket | 30 | Bucket has zero objects |
| Deprecated tags | 20 | Tag key or value matches a known deprecated marker |
| Version sprawl | 30 | Versioning enabled, no lifecycle rules |
| No encryption | 40 | `--check-encryption`, encryption disabled entirely |
| Default KMS key | 15 | `--check-encryption`, using the AWS-managed `aws/s3` key instead of a customer-managed key |
| Public access | 60 (or 30) | `--check-public`, public access enabled (halved if the bucket name matches an intentional-public naming pattern) |

The inactivity factor scales on its own: a bucket that's been quiet for slightly
past the threshold gets 50 points, one that's been quiet for 2x the threshold gets
75, and one that's been quiet for 5x the threshold gets 100 — a multi-year-stale
bucket can cross the default threshold on inactivity alone, without needing an
unrelated second factor to pile on.

## A worked example

Take a bucket named `acme-reports-archive`, unencrypted, empty, and untouched for
900 days against the default 180-day inactivity threshold (900 is exactly 5x):

```
Empty bucket:        30
Inactivity (5x tier): 100
No encryption:        40
----------------------------
Total:               170
```

At the default `--risk-threshold 100`, this bucket crosses the line easily. Lower
the threshold with `--risk-threshold 40` and buckets with just one moderate factor
(say, only the empty-bucket 30 points) start surfacing too — useful for a stricter
audit pass, noisier for a first look at a large account.

## Status, not just score

Once a bucket's score crosses the threshold, `discover` picks one specific status
for it — a bucket can't be both `UNUSED_BUCKET` and `VERSION_SPRAWL` at once, even
if both conditions are technically true. The classification checks in this order:

1. **`UNUSED_BUCKET`** — empty and inactive
2. **`VERSION_SPRAWL`** — versioning enabled, no lifecycle rules
3. **`INACTIVE`** — inactive but not empty
4. **`RISKY`** — everything else that crossed the threshold (e.g. public access alone)

This means a bucket that's both empty *and* version-sprawling gets classified
`UNUSED_BUCKET`, since that's checked first — but it doesn't lose its version-sprawl
risk contribution to the score, and if you pass `--estimate-cost`, the version-sprawl
overhead cost (not the unused-bucket storage cost) is still the one that gets
priced, since the underlying condition that triggers it is independent of which
status wins.

## Practical tuning

- Start with the defaults on a first run to get oriented.
- If the account is large and the output is overwhelming, raise
  `--risk-threshold` to focus on the worst offenders first, then lower it in
  later passes.
- If you have a lot of intentionally-quiet-but-legitimate buckets (compliance
  archives, cold backups), consider `exclude_buckets` / `exclude_prefixes` in
  `.s3spectre.yaml` rather than fighting the score — see
  [CLI Reference](../cli-reference.md#configuration-file).
- Pair a lowered threshold with `--group-by-tag` (see
  [Team ownership rollup](team-ownership-rollup.md)) to see which team's buckets
  are actually driving the noise before you decide whether to tune it away or act
  on it.
