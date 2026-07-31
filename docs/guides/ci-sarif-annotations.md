# Surfacing scan findings as inline PR annotations

`s3spectre scan --format sarif` already produces valid SARIF 2.1.0 output.
GitHub ingests SARIF natively via `github/codeql-action/upload-sarif` — no new
s3spectre feature is needed, just wiring the two together. A ready-to-copy
workflow is at [docs/examples/github-action-scan.yml](../examples/github-action-scan.yml).

## What you get

Once wired up, a PR that references a bucket your scan finds `MISSING_BUCKET`
or `STALE_PREFIX` for gets an inline annotation at the exact file and line
where the code refers to it — the same review experience as a linter comment,
instead of a report someone has to remember to open separately.

## The one honest limitation: only scan mode gets inline annotations

SARIF locations need a file path and line number GitHub can check out and
annotate. `scan` mode has that, but only for findings tied to a reference the
scanner actually found in your code — `Bucket` and `Prefix` values it matched
to a `File`/`Line` pair while walking the repo.

`discover` mode has no such thing. It inspects every bucket in an AWS account
directly, independent of any repository, so a `discover` finding has no
source-code location to point to at all. Its SARIF output uses an `s3://
bucket-name` string as the location URI instead of a real file path — which is
enough to make the SARIF valid and show the finding in the repo's
**Security → Code scanning alerts** list, but GitHub cannot resolve that URI to
a line in a checked-out file, so `discover` findings never produce an inline
PR annotation, no matter how you wire the workflow.

In short:

| | Appears in Security tab | Inline PR annotation |
|---|---|---|
| `scan` finding **with** a code reference | yes | yes |
| `scan` finding **without** a code reference | yes | no |
| any `discover` finding | yes | no |

If you want account-wide `discover` findings surfaced somewhere reviewable,
run `discover` on a schedule and report to wherever your team already reads
dashboards or alerts — SARIF/PR-annotation wiring is specifically a `scan`-mode
fit, not a `discover`-mode one.
