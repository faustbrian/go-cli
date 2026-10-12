package cli

import "context"

// Command is a mutable construction node. Compile publishes an immutable copy
// and later metadata changes do not affect an Application. Construction and
// compilation are not synchronized; callbacks and custom default objects remain
// borrowed application state, not isolated by the metadata snapshot.
type Command struct {
	name          string
	version       string
	aliases       []string
	summary       string
	description   string
	examples      []string
	documentation string
	hidden        bool
	experimental  bool
	deprecated    string
	replacement   string
	children      []*Command
	arguments     []ArgumentDefinition
	options       []OptionDefinition
	handler       Handler
	validations   []Validation
	middlewares   []Middleware
	preRun        []Handler
	postRun       []Handler
	cleanup       []Handler
	interaction   Interaction
	groups        []optionGroupDefinition
}

// CommandOption configures a command construction node.
type CommandOption func(*Command)

// NewCommand creates a mutable node and applies options in order, ignoring nil
// options. Validation is deferred to Compile; custom option panics propagate.
func NewCommand(name string, options ...CommandOption) *Command {
	command := &Command{name: name}
	for _, option := range options {
		switch option { //nolint:gocritic // Intentional mutation-safe control flow.
		case nil:
			continue
		}
		option(command)
	}

	return command
}

// AddSubcommands appends subcommands in registration order.
func (command *Command) AddSubcommands(children ...*Command) error {
	if command == nil {
		return newInternalError("add subcommands to a nil command", nil)
	}
	for _, child := range children {
		if child == nil {
			return newInternalError("add a nil subcommand", nil)
		}
	}
	command.children = append(command.children, children...)

	return nil
}

// WithAliases declares alternate command tokens.
func WithAliases(aliases ...string) CommandOption {
	return func(command *Command) {
		command.aliases = append(command.aliases, aliases...)
	}
}

// WithSummary declares a one-line command summary.
func WithSummary(summary string) CommandOption {
	return func(command *Command) { command.summary = summary }
}

// WithVersion declares command version metadata, normally on the root.
func WithVersion(version string) CommandOption {
	return func(command *Command) { command.version = version }
}

// WithDescription declares the long command description.
func WithDescription(description string) CommandOption {
	return func(command *Command) { command.description = description }
}

// WithExamples declares complete command examples in display order.
func WithExamples(examples ...string) CommandOption {
	return func(command *Command) {
		command.examples = append(command.examples, examples...)
	}
}

// WithDocumentation links the command to additional documentation.
func WithDocumentation(documentation string) CommandOption {
	return func(command *Command) { command.documentation = documentation }
}

// WithHidden controls whether generated discovery surfaces omit the command.
func WithHidden(hidden bool) CommandOption {
	return func(command *Command) { command.hidden = hidden }
}

// WithExperimental marks a command whose compatibility is not yet stable.
func WithExperimental(experimental bool) CommandOption {
	return func(command *Command) { command.experimental = experimental }
}

// WithDeprecated records a deprecation message.
func WithDeprecated(message string) CommandOption {
	return func(command *Command) { command.deprecated = message }
}

// WithReplacement records the preferred replacement command path.
func WithReplacement(path string) CommandOption {
	return func(command *Command) { command.replacement = path }
}

// WithSubcommands registers children in deterministic display order.
func WithSubcommands(children ...*Command) CommandOption {
	return func(command *Command) {
		command.children = append(command.children, children...)
	}
}

// WithArguments registers positional arguments in parse order.
func WithArguments(arguments ...ArgumentDefinition) CommandOption {
	return func(command *Command) {
		command.arguments = append(command.arguments, arguments...)
	}
}

// WithOptions registers command options in display order.
func WithOptions(options ...OptionDefinition) CommandOption {
	return func(command *Command) {
		command.options = append(command.options, options...)
	}
}

// Handler receives parsed input, lifecycle context and borrowed invocation IO.
// It runs synchronously, may overlap across invocations and must return errors
// rather than panic. Cancellation cannot preempt its work; it owns external
// resources, idempotency and cleanup of any goroutines it starts.
type Handler func(context.Context, Invocation) error

// Validation checks resolved input in registration order before middleware.
// Error/cancellation stops later phases, including registered cleanup. It is
// synchronous and not panic-contained; shared captured state needs synchronization.
type Validation func(context.Context, Input) error

// Next continues the chain at most once with a non-nil context while middleware
// is active. Duplicate or late calls return ErrInternal. A concurrently started
// continuation is joined before cleanup; there is no deadline that preempts it.
// Downstream must return normally: recovering its panic outside Next does not
// complete the continuation state and can leave the caller waiting indefinitely.
type Next func(context.Context) error

// Middleware observes command metadata without typed input and runs in declared
// outer-to-inner order. It may short-circuit; cleanup then still runs on normal
// return. Borrowed callbacks may overlap across invocations. Return errors; panic
// recovery around Next is not a supported continuation-completion mechanism.
type Middleware func(context.Context, CommandMetadata, Next) error

// Interaction declares application-owned prompting policy, not TTY detection.
// The framework neither starts a prompt nor enforces external direct-IO behavior.
type Interaction uint8

const (
	// InteractionOptional allows an application to add optional prompts.
	InteractionOptional Interaction = iota
	// InteractionRequired rejects explicit non-interactive execution.
	InteractionRequired
	// InteractionForbidden declares that the command must never prompt.
	InteractionForbidden
)

// WithHandler declares the command execution handler.
func WithHandler(handler Handler) CommandOption {
	return func(command *Command) { command.handler = handler }
}

// WithValidation appends validation for this selected command only, in order.
// Ancestor validation is not inherited when a child executes.
func WithValidation(validations ...Validation) CommandOption {
	return func(command *Command) {
		command.validations = append(command.validations, validations...)
	}
}

// WithMiddleware appends this command’s middleware in outer-to-inner order.
// Ancestor middleware is not inherited automatically by child commands.
func WithMiddleware(middlewares ...Middleware) CommandOption {
	return func(command *Command) {
		command.middlewares = append(command.middlewares, middlewares...)
	}
}

// WithPreRun appends this command’s hooks before its handler; ancestor hooks
// are not inherited by a selected child. Returned failure stops later phases.
func WithPreRun(hooks ...Handler) CommandOption {
	return func(command *Command) { command.preRun = append(command.preRun, hooks...) }
}

// WithPostRun appends this command’s hooks after a successful handler (or no
// handler). Returned failure/cancellation skips later hooks; ancestors are not inherited.
func WithPostRun(hooks ...Handler) CommandOption {
	return func(command *Command) { command.postRun = append(command.postRun, hooks...) }
}

// WithCleanup appends cleanup in reverse registration order after lifecycle
// returns, including middleware short-circuit or returned phase errors, but not
// parse/conversion/validation failure or an unrecovered panic. It retains the
// last lifecycle context values via WithoutCancel with one shared 30-second
// deadline. Returned hook errors do not skip later hooks and join after the
// primary error.
// Hooks cooperate with cancellation; blocking and panics are not contained.
func WithCleanup(hooks ...Handler) CommandOption {
	return func(command *Command) { command.cleanup = append(command.cleanup, hooks...) }
}

// WithInteraction declares the command's interactive capability.
func WithInteraction(interaction Interaction) CommandOption {
	return func(command *Command) { command.interaction = interaction }
}

type optionGroupKind uint8

const (
	optionGroupExclusive optionGroupKind = iota + 1
	optionGroupTogether
)

type optionGroupDefinition struct {
	kind    optionGroupKind
	options []OptionDefinition
}

// WithMutuallyExclusive requires at most one registered grouped binding to
// resolve, counting explicit and defaulted values. Compilation rejects fewer
// than two, duplicate/unregistered members or statically unsatisfiable groups.
func WithMutuallyExclusive(options ...OptionDefinition) CommandOption {
	return func(command *Command) {
		command.groups = append(command.groups, optionGroupDefinition{
			kind: optionGroupExclusive, options: append([]OptionDefinition(nil), options...),
		})
	}
}

// WithRequiredTogether requires every grouped binding if any resolves, including
// defaults. Groups affect resolved presence, not the truthiness of typed values.
// Compilation rejects malformed and statically unsatisfiable combinations.
func WithRequiredTogether(options ...OptionDefinition) CommandOption {
	return func(command *Command) {
		command.groups = append(command.groups, optionGroupDefinition{
			kind: optionGroupTogether, options: append([]OptionDefinition(nil), options...),
		})
	}
}
