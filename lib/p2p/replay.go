package p2p

import (
	"sync"
	"time"
)

const defaultReplayWindow = 30 * time.Second

type ReplayWindow struct {
	mu       sync.Mutex
	maxAge   time.Duration
	observed map[string]int64
}

func NewReplayWindow(maxAge time.Duration) *ReplayWindow {
	if maxAge <= 0 {
		maxAge = defaultReplayWindow
	}
	return &ReplayWindow{
		maxAge:   maxAge,
		observed: make(map[string]int64),
	}
}

func (w *ReplayWindow) Accept(timestampMs int64, nonce string) bool {
	if w == nil {
		return true
	}
	if nonce == "" || timestampMs <= 0 {
		return false
	}
	now := time.Now().UnixMilli()
	windowMs := w.maxAge.Milliseconds()
	if timestampMs < now-windowMs || timestampMs > now+windowMs {
		return false
	}
	expireBefore := now - windowMs
	w.mu.Lock()
	defer w.mu.Unlock()
	for seenNonce, seenTs := range w.observed {
		if seenTs < expireBefore {
			delete(w.observed, seenNonce)
		}
	}
	if _, ok := w.observed[nonce]; ok {
		return false
	}
	w.observed[nonce] = timestampMs
	return true
}
