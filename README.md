# enough

**Know when to stop.**

Software does not become better indefinitely.

Sometimes a project reaches the shape it needed.

enough tries to notice that moment.

A small Go CLI that reads a local Git repository and asks:
**Does this project already do enough?**

It considers declared purpose, promises, scope boundaries, tests, unfinished
markers and recent commit messages. It explains a state rather than assigning
a score. It suggests at most three actions, usually fewer.

## Install

With Go 1.24 or newer:

```sh
go install github.com/vmvarela/enough/cmd/enough@v0.1.0
```

Or build from source: `go build -o enough ./cmd/enough`.
Git must be installed. No runtime service is needed.

## Use

```sh
enough                 # check the current repository
enough check ./project # check another working tree
enough why             # explain the evidence
enough next            # the smallest justified next action
enough --json          # schema_version: 1; same exit codes
enough remember        # explicitly write ENOUGH.md
enough remember --force
enough version
```

The states are `unfinished`, `growing`, `enough`, `resting`, `overgrown` and
`unknown`. Uncertainty is useful: absence of evidence is not proof of failure.

```text
ENOUGH
The project's stated promises appear supported by implementation and tests.

Nothing clearly needs doing.
Then stop.
```

Exit codes: **0** enough/resting, **1** unfinished, **2** growing,
**3** overgrown, **4** unknown, **10** execution/configuration error.
In a script, these are assessment outcomes, not all execution failures.

## Define your frame

An optional `enough.toml` makes your intent explicit:

```toml
version = 1

[purpose]
statement = "Print certificate expiration dates."

[scope]
includes = ["certificate expiration", "JSON output"]
excludes = ["monitoring", "alerts", "dashboards", "plugins"]

[enough]
when = ["installation is documented"]

[test]
command = "go test ./..." # recorded, never executed in 0.1
```

Purpose precedence: configuration, README opening, package description,
then an uncertain repository-name fallback. Configuration criteria are
displayed even when the tool cannot verify them.

`remember` never overwrites an existing declaration without `--force`.
An uncertain assessment produces an honest boundary document without
claiming that completion has been verified.

## Limits, deliberately

enough does not send your repository anywhere.

No network, telemetry, AI, accounts or database. Analysis is read-only.
Tests are detected but not run. Git history is limited to 100 recent commits
within 12 months. Large, generated, binary and linked files are skipped.

The heuristics are deliberately conservative. Source and test references
support a promise; they cannot prove behavior works. Missing references remain
uncertain. A related unfinished marker is stronger evidence. Excluded scope
needs affirmative documentation **and** source evidence. Maintenance patterns
do not prove a stable public API. Expect `unknown` for many real repositories.

Implementation details and deterministic thresholds are in [SPEC.md](SPEC.md).
Contributions should first ask:

> Does this make enough better at knowing when to stop, or does it merely make enough bigger?
