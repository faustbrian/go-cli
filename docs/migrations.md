# Migration guidance

[Documentation map](index.md) · [Repository README](../README.md)
[Next: Parser engine decision](architecture/parser-engine.md)

## go-cli v1 to v2

Version 2 keeps one parser and removes the compatibility grammar instead of
retaining it beside the canonical engine:

- `CompileFlagCommand` is removed. Define the command with `NewCommand` and
  `Compile`, and spell long options with two dashes. Observable differences:

  | Arguments | v1 | v2 |
  |---|---|---|
  | `-name value` | sets `name` | shorthand cluster: `-n` with value `ame` when `n` is a shorthand |
  | `-h value` with an option named `h` | sets `h` | help, exit 0; spell it `--h` |
  | `--count=bad --count=1` | malformed value, exit 2 | last value wins; earlier values are not converted |
  | `extra --help` | usage, exit 2 | help, exit 0 |
  | unknown option | `ErrUsage`, exit 2 | `ErrUnknownOption`, exit 2 |
  | option without a value | `ErrUsage`, exit 2 | `ErrMissingValue`, exit 2 |

  `--help` renders help even after an earlier malformed value, and `Run` also
  answers `__complete` and, when a version is set, `--version`.
- Command sets whose commands declare no options, including sets with no
  commands, now use the same parser as every other command. In a set with
  commands:

  | Arguments | v1 | v2 |
  |---|---|---|
  | `tool <command> --` | unknown option, exit 2 | runs the command |
  | `tool --` | unknown option, exit 2 | no command runs, exit 0 |
  | `tool <command> extra --help` | usage, exit 2 | help, exit 0 |
  | `tool -`, `tool -5`, `tool -- <command>` | unknown option, exit 2 | unknown command, exit 2 |
  | `tool <command> -`, `tool <command> -5`, `tool <command> -- x` | unknown option, exit 2 | usage, exit 2 |
  | `tool <command> extra --unknown` | usage, exit 2 | unknown option, exit 2 |

  In a set with no commands:

  | Arguments | v1 | v2 |
  |---|---|---|
  | `tool --` | unknown option, exit 2 | exit 0 |
  | `tool -`, `tool -5`, `tool -- x` | unknown option, exit 2 | usage, exit 2 |
  | `tool foo` | unknown command, exit 2 | usage, exit 2 |
  | `tool version` with a version | version, exit 0 | usage, exit 2; use `--version` |
  | `tool version` without a version | unknown command, exit 2 | usage, exit 2 |

  An unknown option no longer reports a selected command in
  `Result.Command`.

Replace `github.com/faustbrian/go-cli` imports with
`github.com/faustbrian/go-cli/v2`. Libraries that expose go-cli types, such as
go-service, need a release on v2 before their consumers can adopt it. Generated
output schema identifiers (`go-cli/v1`, `go-cli/manifest/v1`) are unchanged.

## Cobra

Replace exported `*cobra.Command` construction with `NewCommand`, typed binding
descriptors, and `WithHandler`. Move `cmd.Flags().Get*` calls into captured
bindings and `Invocation.Input`. Replace `Execute` side effects with
`Application.Run`. Keep Cobra-specific custom completion or flag types behind
an application adapter until an owned typed parser is available.

## urfave/cli

Replace `*cli.Command` and context lookups with explicit definitions and typed
bindings. Map `Before`, `Action`, and `After` into validation, pre-run, handler,
post-run, and cleanup according to their actual failure semantics. Do not carry
package-global writers or process argv into the new command.

## Kong

Replace reflected tagged structs with explicit argument and option descriptors.
Keep application domain structs by constructing them from typed bindings in
validation or the handler. Reflection-driven command discovery is intentionally
not reproduced.

## Symfony Console

Map command configuration to explicit constructors, `InputArgument` and
`InputOption` values to typed bindings, events to ordered middleware, and return
codes to classified errors plus exit policy. Services are ordinary constructor
arguments; no container lookup occurs in `execute`.

## Laravel Artisan

Replace signatures and container-resolved command classes with explicit Go
command constructors. Captured dependencies replace method injection. Scheduler,
queue, migration, and model-binding behavior remains in the application rather
than entering `cli`.

Migration should pin old and new argv golden cases, output envelopes, help,
completion, exit codes, cancellation, and cleanup before switching the binary.
