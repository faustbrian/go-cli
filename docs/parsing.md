# Parsing contract

[Documentation map](index.md) · [Repository README](../README.md)
[Next: Operations and deployment recipes](operations.md)

`Request.Args` is already shell-tokenized argv excluding the executable name.
`cli` never splits a command string, interpolates variables, expands globs,
normalizes paths, or invokes a shell.

Supported option forms are `--name=value`, `--name value`, `-n value`, attached
short values, and combined boolean shorthands such as `-qv`. `--` terminates
option parsing. Options may appear before or after positionals. Repeated scalar
options use the last occurrence; slices append. An assigned empty value is
explicit.

`CompileFlagCommand` is a separate, flat service-command surface backed by
Go's native `flag.FlagSet`. It preserves one- and two-dash names, bare booleans,
assigned values, first-positional termination and native help ordering without
an application-local argv rewriting layer. Typed conversion runs once per
occurrence, so an earlier malformed value is not hidden by later help or a
valid replacement. Scalar values retain their last successful occurrence.
Native diagnostics are discarded: unknown/missing flags and positional input
return categorical `ErrUsage`; conversion failures retain `ErrMalformedValue`
and secret protection. A registered long `h` name takes precedence over native
help recognition, as it does in `flag.FlagSet`.

This surface rejects children, positional definitions and shorthands at
construction. Normal argv bounds, typed defaults, validations, lifecycle and
IO remain shared. Generated help uses canonical double-dash names rather than
native flag diagnostics. Both `Run` and `RunCommand` use this native grammar
without hidden completion dispatch or implicit version flags. `Compile` and
`CompileCommandSet` retain their normal grammar, described below.

Negative numeric positional tokens are protected from shorthand parsing and
restored byte-for-byte before typed conversion. Negative option values are
accepted as separate or assigned values. Tokens beginning with a hyphen that
are not numeric remain options unless they follow `--` or are consumed as an
option value.

A registered digit shorthand takes precedence over the same single-token
negative number on that command. Use `--` to force the positional meaning at
that boundary. Digit shorthands on sibling commands do not change negative
number parsing for commands where the shorthand is unavailable.

Command names and aliases are byte-exact valid UTF-8. The package does not
perform Unicode normalization: composed and decomposed spellings are distinct
registrations. Names cannot contain whitespace, controls, or a leading hyphen.
Option names are ASCII letters, digits, and hyphens; shorthands are one ASCII
letter or digit. Invalid UTF-8 and NUL argv are rejected before parsing.

Windows drive-letter paths, backslashes, forward slashes, and platform-specific
path syntax are opaque string tokens. The framework never interprets them as
files or changes current working directory.

Unknown commands, unknown options, missing values, and malformed typed values
have separate stable classifications. Command suggestions use bounded
edit-distance work over at most 100 visible candidates and 64 runes; hidden
commands never appear. Parsing changes are SemVer compatibility changes and
must update the changelog and differential tests.

Default limits bound command depth, command count, definitions per command,
argv token count, cumulative argv bytes, completion result count, and completion
bytes. `WithLimits` overrides individual non-zero fields.
