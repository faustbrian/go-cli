# Migration guidance

[Documentation map](index.md) · [Repository README](../README.md)
[Next: Parser engine decision](architecture/parser-engine.md)

## go-cli v1 to v2

Version 2 keeps one parser and removes the compatibility grammar instead of
retaining it beside the canonical engine:

- `CompileFlagCommand` is removed. Define the command with `NewCommand` and
  `Compile`. v1 parsed these commands with Go's `flag` package; v2 uses the
  canonical grammar for both `Run` and `RunCommand`. These rules are the
  complete set of differences:

  1. A single-dash long name is not a long option: `-name`, `-name=x`,
     `-verbose`, `-help`, `-help=x`, `-h=x` and `-version` are unknown
     options (exit 2) unless they form a valid shorthand cluster. In v1 they
     set the option or requested help. Spell long options with two dashes.
  2. Only `--help` and `-h` request help. `--h`, `--h=x` and `--help=x` are
     unknown options (exit 2) without an option named `h`; v1 rendered help.
     With an option named `h`, `-h value` renders help; spell it `--h value`.
  3. Options are parsed after positional arguments until `--`. v1 stopped at
     the first positional, so `extra --help` now renders help (exit 0 instead
     of 2) and `extra --count=bad` and `extra --name` report a malformed or
     missing value instead of `ErrUsage`.
  4. `--version` renders the version (exit 0) when a version is set; v1
     reported `ErrUsage` (exit 2).
  5. `Run` answers `__complete` (exit 0); v1 reported `ErrUsage`.
     `RunCommand` does not answer completion.
  6. Only the last value of a repeated scalar or `TypedOption` option is
     converted, and later tokens decide the outcome after an earlier
     malformed value: `--count=bad --count=1` runs, `--count=bad --help`
     renders help and `--count=bad --name` reports a missing value. v1
     reported the malformed value.
  7. A negative number such as `-5` is a positional argument (a usage error
     unless help or version follows) instead of an unknown flag.
  8. Unknown options report `ErrUnknownOption` and options without a value
     report `ErrMissingValue` instead of `ErrUsage`; both still exit 2.

- Command sets whose commands declare no options, including sets with no
  commands, now use the same parser as every other command. v1 stopped at the
  first token that was not a known command: one starting with `-` was an
  unknown option, any other word an unknown command, and a second argument
  after a command a usage error. Later tokens were never read. In v2:

  1. Before a command, `tool --` exits 0 without running anything, and `-`,
     `-5` or a word after `--` is an unknown command (exit 2).
  2. After a command, and anywhere in a set with no commands, tokens are read
     left to right until `--`. The first `--help` or `-h` renders help
     (exit 0); the first `--version` renders the version (exit 0) in a set
     with no commands that has a version and is otherwise an unknown option;
     the first unknown option, including `-help`, reports an unknown option
     (exit 2). Only when none of these appears do leftover arguments (a word,
     `-`, `-5`, or anything after `--`) report a usage error (exit 2); with no
     leftovers the command runs, so `tool <command> --` runs the command.
  3. `version` is a command only in a set with commands. In a set with no
     commands `tool version` is a usage error (v1 rendered the version, or
     reported an unknown command without one); use `--version`.
  4. An unknown option no longer reports a selected command in
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
