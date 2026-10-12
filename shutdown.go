package cli

import (
	"context"
	"errors"
	"sync"
)

// ErrSignal is the default cancellation cause when no signal cause is supplied.
var ErrSignal = errors.New("shutdown signal received")

// ShutdownAction describes a caller-owned signal policy transition.
type ShutdownAction uint8

const (
	// ShutdownGraceful means the first signal canceled the graceful context.
	ShutdownGraceful ShutdownAction = iota + 1
	// ShutdownForced means a repeated signal requested forced termination.
	ShutdownForced
	// ShutdownAlreadyForced means forced termination was already requested.
	ShutdownAlreadyForced
)

// ShutdownController translates caller-delivered signals without registering
// process signal handlers or starting goroutines. The application remains
// responsible for signal.Notify/signal.Stop, waiting and forced process exit.
// Construct with NewShutdownController; methods are synchronized and nil-safe,
// but a non-nil zero value is not usable. Do not copy after construction.
type ShutdownController struct {
	context context.Context
	cancel  context.CancelCauseFunc
	forced  chan struct{}
	mu      sync.Mutex
	count   uint64
	closed  bool
}

// NewShutdownController derives a cancel-cause context; nil parent is an
// ErrInternal-compatible construction error. Call Close to release it even when
// no signal arrives. Parent cancellation does not close the Forced channel.
func NewShutdownController(parent context.Context) (*ShutdownController, error) {
	if parent == nil {
		return nil, newInternalError("create shutdown controller with nil context", nil)
	}
	ctx, cancel := context.WithCancelCause(parent)

	return &ShutdownController{
		context: ctx,
		cancel:  cancel,
		forced:  make(chan struct{}),
	}, nil
}

// Context returns the graceful cancellation context.
func (controller *ShutdownController) Context() context.Context {
	if controller == nil {
		return nil
	}

	return controller.context
}

// Forced returns the controller-owned channel closed by the second signal,
// never by Close/parent cancellation. Callers only receive; nil receiver returns nil.
func (controller *ShutdownController) Forced() <-chan struct{} {
	if controller == nil {
		return nil
	}

	return controller.forced
}

// Signal applies first-cancel/second-forced policy with nil cause replaced by
// ErrSignal. The first cancellation cause wins. It never exits or stops work;
// ShutdownAlreadyForced also describes nil/closed receivers and is not proof
// that Forced closed. Repeated calls are safe alongside Close.
func (controller *ShutdownController) Signal(cause error) ShutdownAction {
	if controller == nil {
		return ShutdownAlreadyForced
	}
	if cause == nil {
		cause = ErrSignal
	}
	controller.mu.Lock()
	defer controller.mu.Unlock()
	if controller.closed {
		return ShutdownAlreadyForced
	}
	controller.count++
	switch controller.count {
	case 1:
		controller.cancel(cause)
		return ShutdownGraceful
	case 2:
		close(controller.forced)
		return ShutdownForced
	default:
		return ShutdownAlreadyForced
	}
}

// Close idempotently cancels/releases the derived context and rejects later
// signal transitions. It does not close Forced, wait for work or stop signal.Notify;
// the application owns those operations. Nil receiver is a no-op.
func (controller *ShutdownController) Close() {
	if controller == nil {
		return
	}
	controller.mu.Lock()
	defer controller.mu.Unlock()
	if controller.closed {
		return
	}
	controller.closed = true
	controller.cancel(context.Canceled)
}
