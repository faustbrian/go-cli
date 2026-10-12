// Package cli builds explicit, typed, composable command-line applications.
//
// Applications construct commands in their composition root, compile immutable
// metadata, and execute already-tokenized argv with caller-owned context and IO.
// Bounded one-binary service processes can compile a direct CommandSet without
// linking features they do not publish.
// The dependency-free parser is internal and does not appear in public
// contracts. The core uses only the standard library, without cgo, external
// services, hidden workers, signal registration or process exit. Start with
// NewCommand and Compile, or CompileCommandSet for direct service commands.
//
// Definitions are mutable and require caller synchronization during compilation.
// Compiled invocations have separate parser/output state, but callbacks, custom
// typed values and caller streams can still share application state. Cancellation
// and cleanup deadlines are cooperative; no callback or IO is forcibly stopped.
// Command-string tokenization, prompts, terminal detection, raw mode and fullscreen UI are not
// supplied. See docs/architecture.md and docs/security.md in the repository.
package cli
