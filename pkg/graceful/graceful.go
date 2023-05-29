// Package graceful contains API for working with graceful application shutdown.
//
// Application starts listening for SIGINT or SIGTERM signals and handles them properly.
package graceful

import (
	"context"
	"errors"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"
)

// Should it be moved to config?
var shutdownTimeout = 10 * time.Second

// ShutdownFunc is a callback-type for registering callbacks before application shutdown.
type ShutdownFunc func() error

var (
	handler   *shutdownHandler
	execOnErr func(error)
)

var (
	// ErrTimeoutExceeded is returned when the application fails to shutdown for a given period of time.
	ErrTimeoutExceeded = errors.New("failed to perform graceful shutdown: timeout exceeded")

	// ErrForceShutdown is returned when the user or operating system is sending SIGINT or SIGTERM
	// for the application being is graceful-shutdown state.
	ErrForceShutdown = errors.New("failed to perform graceful shutdown: force shutdown occurred")
)

func setupHandler() {
	notify := make(chan os.Signal, 1)
	forceStop := make(chan struct{}, 1)
	signal.Notify(notify, syscall.SIGINT, syscall.SIGTERM)
	handler = newHandler(notify, forceStop)

	execOnErr = func(err error) {
		log.Printf("shutdown callback error: %v", err)
	}
}

// nolint gochecknoinits
func init() {
	setupHandler()
}

// AddCallback registers a callback for execution before shutdown.
func AddCallback(fn ShutdownFunc) {
	handler.add(fn)
}

// ExecOnError executes the given handler
// when shutdown callback returns any error.
func ExecOnError(cb func(err error)) {
	execOnErr = cb
}

// WaitShutdown waits for application shutdown.
//
// If the user or operating system interrupts the graceful shutdown,
// ErrForceShutdown is returned.
// If applications fails to shutdown for a given period of time,
// ErrTimeoutExceeded is returned.
func WaitShutdown() error {
	select {
	case <-handler.stop:
	case <-handler.forceStop:
	}

	handler.markAsShutdown()

	notify := make(chan os.Signal, 1)
	signal.Notify(notify, syscall.SIGINT, syscall.SIGTERM)

	ctx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()

	done := make(chan struct{})
	go func() {
		defer close(done)
		for i := len(handler.callbacks) - 1; i >= 0; i-- {
			err := handler.callbacks[i]()
			if err != nil && execOnErr != nil {
				execOnErr(err)
			}
		}
	}()

	select {
	case <-done:
		return nil
	case <-notify:
		return ErrForceShutdown
	case <-ctx.Done():
		return ErrTimeoutExceeded
	}
}

func IsShuttingDown() bool {
	return handler.isShuttingDown()
}

// ShutdownNow sends event to initiate graceful shutdown.
func ShutdownNow() {
	handler.forceStop <- struct{}{}
}
