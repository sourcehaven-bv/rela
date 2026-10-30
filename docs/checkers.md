# Prose checks

rela can run external prose checkers over entity bodies: spelling, grammar
and style tools such as [Vale](https://vale.sh). You register a checker once
in the metamodel. rela passes it the entity type, so the checker's own config
can apply different rules per type.

Findings are advisory. A save never runs a checker and never fails because of
one.

## Registering a checker

Add a `checkers:` map to the root `schema.yaml`. An included file may not
declare checkers.

```yaml
checkers:
  isms-style:
    command: ["vale", "--output=JSON", "--no-exit", "--config=/etc/rela/vale/.vale.ini", "{in}"]
    format: vale
    exit_codes: [0]
    timeout: 10s
    analyze: true
```

Fields:

- **`command`** (required): an argv array, not a shell string. `{in}` is a
  temp file holding the body at `<type>/<id>.md` (for example
  `procedure/PROC-1.md`). `{type}` is replaced by the entity type name. A command
  that references no `{in}` gets the body on stdin.
- **`format`** (required): how rela reads the output. `vale` reads Vale's
  `--output=JSON`. `rela` reads the native format below.
- **`exit_codes`**: the non-zero exit codes that still mean "findings on
  stdout". Exit code 0 always counts as success. Any other exit code is a
  checker error.
- **`timeout`**: a Go duration, at most `60s`. Default `10s`.
- **`analyze`**: also report this checker's findings from analyze runs that
  ask for prose checks. Default `false`.

When any checker is declared, every entity type name must match
`^[A-Za-z0-9][A-Za-z0-9_-]*$`, because the name becomes the input directory
and a glob in the checker's config. An entity id that is not a safe file name
is written as `body.md`. A type that does not match fails the
metamodel load.

The command's binary must be on `PATH` where rela runs.

## Rules per entity type with Vale

Vale picks rules with glob sections in `.vale.ini`. Vale matches a section
against the file's full path, and `{in}` is an absolute temp path, so a type
section has the form `[**/<type>/*.md]`:

```ini
StylesPath = /etc/rela/vale/styles
MinAlertLevel = suggestion

# The base set, for every type.
[*.md]
BasedOnStyles = Vague

# Policies: the base set, plus no product names.
[**/policy/*.md]
BasedOnStyles = Vague, Overspecific
Overspecific.Product = error

# Tasks: the base set without the passive-voice rule.
[**/task/*.md]
BasedOnStyles = Vague
Vague.Passive = NO
```

A section without `**/`, such as `[policy/*.md]`, matches nothing, and Vale then applies only
the base set, without an error.

A type section replaces the `[*.md]` section. It does not add to it, so it
lists the base styles again.

## The native format

A checker with `format: rela` writes one JSON object to stdout:

```json
{"findings": [
  {"rule": "House.Frequency", "severity": "warning",
   "message": "Say how often: 'every quarter'.",
   "line": 12, "column": 5, "length": 9,
   "replacements": ["every quarter"], "link": "https://example.org/rules"}
]}
```

- `line` and `column` start at 1. `column` and `length` count Unicode code
  points, not bytes.
- `severity` is `error`, `warning` or `suggestion`. Any other value is read
  as `warning`.
- `replacements` and `link` are optional. Unknown fields are ignored.
- A finding without a message is dropped. A finding whose position does not
  fit the body is kept without a position.

Any tool can be wrapped this way, for example a Hunspell or LanguageTool
script.

## Where checker config lives

Checker config, such as Vale's `.vale.ini` and styles, belongs to the
operator. Install it on the host outside the project directory, for example
under `/etc/rela/vale`, and name it in `command`.

On Linux a confined checker can read only the system binary and library
directories and the paths in `RELA_SANDBOX_CHECK_READ_PATHS`. List the
config directory there:

```bash
RELA_SANDBOX_CHECK_READ_PATHS=/etc/rela/vale
```

rela refuses a read path inside the project directory. On macOS reads are not
restricted; network access is still denied. See
[transforms.md](transforms.md#sandbox-read-paths) for the sandbox.

Every checker runs in its own temp directory, so a tool that searches upward
for config does not find files in rela's working directory. Pass the config
path explicitly.

## Running checks

### From the CLI

```bash
rela analyze prose                # every entity, checkers with analyze: true
rela analyze prose --type procedure
rela analyze all --prose          # include prose checks in the full run
rela -o json analyze prose        # JSON for CI
```

`rela analyze all` runs no checker unless you pass `--prose`, because each
entity costs one process. Each entity is checked at the face the CLI's world
selects; other faces are not checked.

Findings do not make the command fail. A checker whose binary is not on
`PATH` stops the command before any entity is checked. A checker that fails,
exits with a code not in `exit_codes` or writes output rela cannot parse is
reported per entity, and the command then exits non-zero with status
`error` in JSON. With `analyze all --prose -o json`, `details.prose_entities`
lists the findings and errors per entity.

## Security

Checkers run through the same confined runner as transforms: argv only, no
network, writes only to the run's temp dir, resource limits, a timeout and an
output cap. Checks use their own small pool, so they cannot hold up exports.
A request can never choose a command, flag or path.
