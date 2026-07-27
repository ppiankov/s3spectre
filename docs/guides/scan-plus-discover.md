# Scan and discover answer different questions

It's easy to reach for just one of s3spectre's two modes and miss what the other
one would have told you. They're not interchangeable:

- **`scan`** answers "does the code match reality?" — it cross-references bucket
  and prefix references in a repository against live AWS state. It needs a repo
  path and only knows about buckets your code actually mentions.
- **`discover`** answers "what does the whole account look like?" — it inspects
  every bucket in the account, with no dependency on any particular repo. It
  doesn't know or care whether a bucket is referenced anywhere in code.

Run only `discover` and you'll never learn that a specific stale prefix your code
still preloads on every page load. Run only `scan` and you'll never see the 200
other buckets in the account that no repo references at all. Run both against the
same bucket, and each mode either corroborates the other or surfaces a genuine
disagreement worth investigating.

## A worked scenario

Say `scan` against a frontend repo turns up:

```
  [VERSION_SPRAWL]: acme-web-assets-prod
    Versioning enabled but no lifecycle rules to clean up old versions

  [STALE_PREFIX]: onboarding/videos
    No modifications for 400 days (threshold: 90)
```

Two findings on one bucket. The natural first instinct for `STALE_PREFIX` is "this
looks abandoned, maybe safe to clean up" — **check the code before acting on that
instinct.** A prefix search in this hypothetical repo might turn up something like:

```
index.html:  <link rel="preload" as="video" href="https://acme-web-assets-prod.s3.../onboarding/videos/welcome.mp4">
src/app/onboarding/onboarding.component.ts:  const videoDir = 'https://acme-web-assets-prod.s3.../onboarding/videos';
```

That prefix isn't dead — it's preloaded on page load and rendered by a live
component. It's just *stable* content: onboarding videos that don't change often
by design. "Unmodified for 400 days" means "this is finished," not "this is
abandoned." Deleting it would break the onboarding flow.

The `VERSION_SPRAWL` finding is the one worth acting on here: versioning is on, so
every rare update to these videos leaves the previous version sitting in S3
forever, accumulating cost with no cleanup. That's exactly the case
`--suggest-lifecycle-policy` is for — see
[From "this looks wasteful" to a reviewable fix](cost-and-cleanup-workflow.md).

## The general lesson

A `STALE_PREFIX` finding is a fact about *modification recency*, not a verdict on
whether something is dead. s3spectre reports the fact and lets you decide — it
never infers intent or auto-remediates (see the project's
[Philosophy](../cli-reference.md#philosophy)). Before treating any "stale" finding
as "safe to remove," grep your own codebase for the prefix. If it turns up in a
live code path, the finding was about staleness, not abandonment, and the real fix
is usually the neighboring `VERSION_SPRAWL` or cost finding instead.
