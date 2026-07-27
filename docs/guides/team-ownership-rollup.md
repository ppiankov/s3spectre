# Finding which team to talk to first

A `discover` run against a large account can return hundreds of buckets. The flat
list tells you *what's* wrong, but not *who owns it* — and "who should I go talk
to" is usually the first question once you've seen the numbers. `--group-by-tag`
rolls the same findings up by a bucket tag (most commonly something like `Team` or
`Owner`) so you get an ownership-level view instead of a wall of bucket names.

## Running it

```sh
s3spectre discover --check-public --group-by-tag Team --format json
```

This adds a `tag_rollup` object to the output, one entry per distinct tag value
plus an `untagged` entry for any bucket missing the tag entirely — nothing falls
through silently:

```json
{
  "tag_rollup": {
    "backend": {
      "bucket_count": 40,
      "risk_score": 2100,
      "average_risk_score": 52.5,
      "unused_count": 12,
      "risky_count": 3
    },
    "platform": {
      "bucket_count": 5,
      "risk_score": 620,
      "average_risk_score": 124.0,
      "risky_count": 4
    },
    "untagged": {
      "bucket_count": 8,
      "risk_score": 240,
      "average_risk_score": 30.0
    }
  }
}
```

## Read the average, not just the sum

This is the part worth slowing down for. `risk_score` in the example above makes
`backend` look like the bigger problem — 2100 versus platform's 620. But `backend`
owns 40 buckets and `platform` owns only 5, so the *average* tells a different
story: platform's buckets are, on average, more than **twice** as risky per bucket
(124.0 vs 52.5). A raw sum always favors whichever group owns more buckets; the
average surfaces a small group of consistently bad buckets that would otherwise get
lost next to a bigger group of mostly-fine ones.

Read both numbers together:

- **`risk_score`** (the sum) — total blast radius. Useful for "how much risk does
  this team's whole footprint represent."
- **`average_risk_score`** — per-bucket severity. Useful for "which team should I
  actually go talk to first."

The rendered list stays sorted by the summed score, not the average — so don't
rely on position in the list to reveal which group has the worse average. Read the
`average_risk_score` field explicitly per entry.

## Picking a tag key

`--group-by-tag` takes whatever tag key your account actually uses for ownership —
common choices are `Team`, `Owner`, `BusinessUnit`, or `Project`. If your buckets
don't consistently carry one of these, the `untagged` bucket in the output is
itself a signal: a large `untagged` group usually means tagging discipline is the
first thing to fix, before ownership rollups are useful at all.

## Combining with a lower threshold

`--group-by-tag` pairs naturally with a lowered `--risk-threshold` (see
[Understanding risk scores](understanding-risk-scores.md)) — cast a wider net on
which buckets qualify as findings, then use the rollup to see which team's
footprint that wider net actually landed on.
