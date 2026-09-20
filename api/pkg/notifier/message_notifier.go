// Package notifier provides an in-process fan-out so long-polling phones wake
// up the moment a message is queued for their user, instead of waiting for the
// next poll interval.
package notifier

import (
	"sync"

	"github.com/NdoleStudio/httpsms/pkg/entities"
)

// MessageNotifier signals subscribers when a message is queued for a user.
// It is in-process: a single API instance serves all long-polls. With multiple
// replicas a queued message only wakes waiters on the replica that handled the
// send, and the others fall back to the long-poll timeout.
type MessageNotifier struct {
	mu      sync.Mutex
	waiters map[entities.UserID]map[chan struct{}]struct{}
}

// NewMessageNotifier creates a new MessageNotifier.
func NewMessageNotifier() *MessageNotifier {
	return &MessageNotifier{waiters: map[entities.UserID]map[chan struct{}]struct{}{}}
}

// Subscribe returns a channel signalled when a message is queued for userID.
// The returned cancel function must be called to release the subscription.
func (n *MessageNotifier) Subscribe(userID entities.UserID) (<-chan struct{}, func()) {
	ch := make(chan struct{}, 1)

	n.mu.Lock()
	if n.waiters[userID] == nil {
		n.waiters[userID] = map[chan struct{}]struct{}{}
	}
	n.waiters[userID][ch] = struct{}{}
	n.mu.Unlock()

	var once sync.Once
	cancel := func() {
		once.Do(func() {
			n.mu.Lock()
			delete(n.waiters[userID], ch)
			if len(n.waiters[userID]) == 0 {
				delete(n.waiters, userID)
			}
			n.mu.Unlock()
		})
	}
	return ch, cancel
}

// Notify wakes all current waiters for userID without blocking.
func (n *MessageNotifier) Notify(userID entities.UserID) {
	n.mu.Lock()
	defer n.mu.Unlock()
	for ch := range n.waiters[userID] {
		select {
		case ch <- struct{}{}:
		default:
		}
	}
}
