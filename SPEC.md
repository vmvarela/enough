# enough 0.1.0

Know when to stop.

## Purpose

Answer whether a local Git project may already fulfill its own purpose.
Explain conclusions with evidence. Prefer uncertainty and silence to invented
work. Deliberate scope exclusions are first-class design constraints.

## Boundaries

A single Go binary and the installed Git executable. One direct dependency:
BurntSushi/toml. No AI, APIs, code parsing, scores, vulnerability detection,
roadmap management, plugins, web UI, telemetry, accounts or persistent state.
No remote repository enrichment. No automatic code edits or test execution.
No GitHub API calls at runtime. All implementation packages remain internal.

## Interface

Commands: `enough` / `enough check`, `why`, `next`, `remember`, `version`.
Optional repository path defaults to the current working tree. A subdirectory
resolves to its Git root. Bare repositories are not supported. Boolean flags
can appear before or after the command. `--` allows a path beginning with `-`.

`--json` supports check, why and next, emitting the same structured result.
Errors go to stderr and never masquerade as successful JSON assessments.
`--force` is valid only with remember. Help exits zero.

Exit codes:

| State | Code |
| --- | --- |
| enough / resting | 0 |
| unfinished | 1 |
| growing | 2 |
| overgrown | 3 |
| unknown | 4 |
| execution/configuration error | 10 |

Default output normally fits within 20 lines. Why shows at most 12 evidence
items and counts omitted items. Terminal fields are shortened to 160 runes and
control characters are removed. JSON retains full structured evidence.
Recommendations are capped at three; no recommendation is valid unless it
could materially affect whether the project fulfills its own promise.

## Configuration

`enough.toml` is optional. If present it must have `version = 1`. Supported keys:

| Key | Meaning |
| --- | --- |
| purpose.statement | strongest declared purpose |
| scope.includes | intentional capabilities |
| scope.excludes | intentional exclusions |
| enough.when | human completion criteria |
| test.command | parsed, never executed |

Purpose and scope entries are single lines of at most 500 bytes. Lists have at
most 100 entries. Unknown keys, type errors, unsupported versions and invalid
TOML are execution errors. Parser errors never include source excerpts.

## Repository inspection

Read the working tree, including current uncommitted source. This is an
assessment of local files, not only the last commit. Never change them during
analysis. Never run hooks, repository programs, test commands or package tools.

Purpose precedence:

1. Nonempty purpose.statement.
2. First conservative descriptive line in README's opening content.
3. package.json description.
4. Unclear purpose, with repository name as a low-confidence fallback.

README candidates in order: README.md, README, README.rst, README.txt.
go.mod's module name and package.json's name supply project identity.
Markdown headings, badges and fenced examples are not purpose statements.
Names and metadata do not establish behavioral completeness.

Inspect common source extensions and test filenames without semantic parsing.
Skip .git, hidden directories, vendor, node_modules, dist, build, coverage,
virtual environments, testdata and fixtures. Fixture content must not influence
the project's own assessment. Do not follow linked files. Confine reads with
Go's os.Root. Binary and non-UTF-8 files are never treated as text.

Limits: 10,000 visited files, 1 MiB per read, 32 MiB combined source text.
An inspection limit, oversized source or unreadable source prevents a confident
complete classification. Read failures for explicit configuration are errors.
The CLI imposes a ten-second overall timeout. Target typical analysis time is
under 250 ms, excluding Git and filesystem effects; no absolute latency promise.

## Promises

Promise fields: text, source, confidence, status, supporting locations.
Statuses: satisfied, unclear, missing. Confidence: low, medium, high.

Sources:

- Configuration includes and completion criteria (high confidence).
- Bullet points under Features, Usage, Commands, Supports (medium confidence).
- Explicit Supports/Provides/Can statements (medium confidence).
- Long options in fenced examples within Usage or Commands (medium confidence).
- Purpose itself if no other promise was detected.

At most 100 distinct promises. Normalize words and omit generic terms.
Require all remaining distinctive words to appear in source and a test before
calling a promise satisfied. Comment lines and unfinished lines do not provide
implementation support. This is lexical evidence, not verified execution.

A TODO/FIXME/XXX matching a promise's distinctive words in non-test source
marks that promise missing. Unrelated markers never establish incompleteness.
Missing source references leave a promise unclear, not missing. Completion
criteria that cannot be resolved remain visible as unverifiable criteria.

## Git history

Use the installed Git executable with controlled machine output:
hash, timestamp and subject separated by NUL bytes. Do not display or retain
raw subjects, bodies or arbitrary source excerpts in the result.

Inspect at most 100 commit messages within twelve calendar months. Read latest
commit time and one root-commit timestamp separately as history anchors.
Never download missing objects; deny all Git transports and disable lazy fetch, prompts and signature
verification. Ignore inherited Git repository overrides and global config.
Unborn working trees are supported. Git errors are execution errors.

Classification:

| Conventional prefix | Kind |
| --- | --- |
| feat | feature |
| fix | fix |
| docs | docs |
| chore, build, ci, test | maintenance |
| refactor, perf | refactor |
| anything else | unknown |

Scoped and breaking-change prefixes are supported. Simple non-conventional
leading verbs identify add/introduce/implement, fix/repair/resolve,
document/docs, refactor/simplify and dependency/compatibility version bumps.
Ambiguous messages remain unknown.

## Scope drift

Strong drift requires an excluded capability to be affirmatively advertised
and have a corresponding non-test source area. A directory alone is never
sufficient. Exclusion sections, negations and fenced examples do not advertise
capabilities. A small fixed set of aliases covers dashboards, alerts,
monitoring, plugins, web UI, renewal and AI API providers.

Without explicit exclusions, a narrow CLI purpose plus at least three
documented functional areas with source evidence may indicate expansion.
This compares the current declared frame to current features. It does not
claim knowledge of the original historical purpose.

## Decision rules, in precedence order

1. **unfinished:** at least one related explicit unfinished marker.
2. **overgrown:** corroborated scope drift.
3. **growing:** at least three feature commits, a feature majority among
   meaningful changes, and a latest commit within 90 days.
4. **resting:** supported completion and a maintenance/rest pattern.
5. **enough:** supported completion without stronger contradictory evidence.
6. **unknown:** insufficient evidence.

Meaningful changes are feature, fix, maintenance and refactor; unknown and
documentation messages do not count toward the feature denominator.

Supported completion requires a non-low-confidence purpose, implementation,
at least one promise, every detected promise supported by source and test
references, and no incomplete inspection.

A maintenance/rest pattern requires at least a year since the reachable root
commit, no features, unknown messages or refactors in the window, and a
non-shallow history. Quiet history alone never proves completion. These rules
describe commit patterns, not verified public-interface stability.

All conclusions cite locations and confidence. Fewer corroborated signals
outweigh many weak ones. Inactivity depends only on an injected clock, which
tests fix. Other behavior uses neither randomness nor external data.

## Recommendations

Missing explicit promises receive up to three focused suggestions. Drift may
receive one suggestion to reconsider scope. Growing and unknown need not
receive any action. Enough and resting normally say nothing clearly needs
doing. Never recommend websites, badges, architecture layers or arbitrary
coverage targets.

## ENOUGH.md

Explicit remember writes purpose, includes, deliberate exclusions, a short
design-boundary rationale and the UTC declaration date. No metrics.
Exclusive creation protects existing files. Force uses replacement rather
than writing through existing hard links, and rejects symlink destinations.
Unknown, unfinished or expanding assessments can record an intended frame;
their document explicitly states completion has not been verified.

## JSON schema 1

Top-level fields: schema_version, name, state, summary, purpose,
recommendations, evidence, analysis. Recommendations and evidence are arrays.
Purpose contains statement, source, confidence. Evidence contains kind,
message, source, confidence. Analysis contains promises, changes, stability,
growth, scope, maintenance, tests_detected, todo_markers and truncated.
No numeric sufficiency or quality score is exposed.

## Validation and release

Unit tests cover TOML types, purpose precedence, promise uncertainty,
commit classification, scope exclusions, TODOs, limits, output and exit codes.
All six states have fixed-clock fixtures and golden check/why/next outputs.
Integration tests exercise installed Git, unborn history and environment
isolation. Remember tests prove refusal to overwrite and symlink protection.
Every result has at most three recommendations.

Release when go test ./..., go test -race ./... and go vet ./... pass, a
single binary runs locally, JSON is versioned, analysis is read-only and the
README explains the philosophy in about two minutes. Do not wait for perfect
promise inference.

## Later, only if needed

Potential 0.2 experiments: opt-in tests, better CLI flag detection,
release-history analysis, ENOUGH.md comparison, post-declaration drift,
explicit --since and enough init. Historical README comparison is outside
0.1. None is a commitment. Evaluate every major proposal against enough.toml.

> Does this make enough better at knowing when to stop, or does it merely make enough bigger?
