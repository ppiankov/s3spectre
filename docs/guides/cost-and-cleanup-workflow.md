# From "this looks wasteful" to a reviewable fix

Two flags turn a `discover` finding into something you can act on instead of just
worry about: `--estimate-cost` puts an approximate dollar figure on the waste, and
`--suggest-lifecycle-policy` generates a ready-to-review remediation snippet. Used
together they take you from "this bucket looks bad" to "here's roughly what it
costs and here's a snippet to fix it" in one run.

## Estimating cost

```sh
s3spectre discover --estimate-cost --format json
```

This prices two distinct, narrow scopes — s3spectre is explicit that it is **not**
a general cost calculator:

- **Version-sprawl overhead** — for a bucket with versioning enabled and no
  lifecycle rules, the cost of the noncurrent-version storage sitting on top of the
  live data (`estimated_monthly_cost_usd`).
- **Inactive/unused bucket storage** — for a bucket flagged `INACTIVE` or
  `UNUSED_BUCKET`, the cost of its full storage (`estimated_storage_cost_usd`).

A bucket only ever gets one of the two fields populated (whichever applies to its
classification), and a `CostUSD()`-equivalent total across every priced bucket
lands in the summary as `total_estimated_cost_usd` — one headline number instead of
manually adding up every bucket's figure yourself. Everything else (healthy
buckets, public-access-only findings, scan mode) stays unpriced; the estimate uses
an embedded S3 Standard per-GB-month table and doesn't account for request or
data-transfer charges, so treat it as directionally useful, not a bill.

## Generating a remediation snippet

```sh
s3spectre discover --suggest-lifecycle-policy --format markdown
```

For each `VERSION_SPRAWL` finding, this attaches a deterministic JSON snippet
(matching the shape of S3's `PutBucketLifecycleConfiguration` API) and an
equivalent Terraform `aws_s3_bucket_lifecycle_configuration` block, both
suggesting a noncurrent-version expiration rule:

```json
{
  "Rules": [
    {
      "ID": "expire-noncurrent-versions",
      "Status": "Enabled",
      "NoncurrentVersionExpiration": {
        "NoncurrentDays": 90
      }
    }
  ]
}
```

This is generated purely from data s3spectre already collected — no new AWS API
call is made to produce it, and no AWS write API is ever called by s3spectre
itself. It's text for a human to review, adjust the retention window if 90 days
doesn't match your own policy, and apply through whatever deployment path you
already use (console, `aws s3api`, Terraform apply). Nothing is applied
automatically.

## Putting them together

A typical pass looks like:

1. Run `discover --estimate-cost --group-by-tag Team` to see which team's
   footprint carries the most estimated cost (see
   [Team ownership rollup](team-ownership-rollup.md) for reading the rollup).
2. For the buckets actually worth acting on, rerun with
   `--suggest-lifecycle-policy --format markdown` to get a snippet per bucket,
   ready to paste into a PR.
3. Review each snippet — the 90-day default retention is a starting point, not a
   policy recommendation for your account. Adjust it, then apply it yourself.

None of this deletes anything, prices anything you haven't asked it to, or applies
anything without a human reading it first — see the project's
[Safety](../../README.md#safety) section.
