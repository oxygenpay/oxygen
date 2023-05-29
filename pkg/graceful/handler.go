package graceful

import (
	"os"
	"sync"
	"sync/atomic"
)

const (
	handlerStatusClosed uint32 = 1
)

type shutdownHandler struct {
	stop      chan os.Signal
	forceStop chan struct{}
	mutex     sync.Mutex
	callbacks []ShutdownFunc
	isClosed  uint32
}

func newHandler(notify chan os.Signal, forceStop chan struct{}) *shutdownHandler {
	return &shutdownHandler{
		stop:      notify,
		forceStop: forceStop,
	}
}

func (h *shutdownHandler) add(fn ShutdownFunc) {
	h.mutex.Lock()
	h.callbacks = append(h.callbacks, fn)
	h.mutex.Unlock()
}

func (h *shutdownHandler) markAsShutdown() {
	atomic.StoreUint32(&h.isClosed, handlerStatusClosed)
}

func (h *shutdownHandler) isShuttingDown() bool {
	return atomic.LoadUint32(&h.isClosed) == handlerStatusClosed
}
