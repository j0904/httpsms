package notifier

import (
	"testing"
	"time"

	"github.com/NdoleStudio/httpsms/pkg/entities"
	"github.com/stretchr/testify/assert"
)

func TestMessageNotifierNotifyWakesSubscriber(t *testing.T) {
	n := NewMessageNotifier()
	wake, cancel := n.Subscribe(entities.UserID("user-1"))
	defer cancel()

	n.Notify(entities.UserID("user-1"))

	select {
	case <-wake:
	case <-time.After(time.Second):
		t.Fatal("subscriber was not woken by Notify")
	}
}

func TestMessageNotifierOnlyWakesMatchingUser(t *testing.T) {
	n := NewMessageNotifier()
	wake, cancel := n.Subscribe(entities.UserID("user-1"))
	defer cancel()

	n.Notify(entities.UserID("user-2"))

	select {
	case <-wake:
		t.Fatal("subscriber was woken by another user's message")
	case <-time.After(50 * time.Millisecond):
	}
}

func TestMessageNotifierCancelRemovesSubscriber(t *testing.T) {
	n := NewMessageNotifier()
	wake, cancel := n.Subscribe(entities.UserID("user-1"))
	cancel()

	// notifying after cancel must not panic or block
	n.Notify(entities.UserID("user-1"))

	select {
	case <-wake:
		t.Fatal("cancelled subscriber was woken")
	case <-time.After(50 * time.Millisecond):
	}
}

func TestMessageNotifierNotifyWithoutWaitersIsSafe(t *testing.T) {
	n := NewMessageNotifier()
	assert.NotPanics(t, func() { n.Notify(entities.UserID("nobody")) })
}

func TestMessageNotifierNotifyIsNonBlocking(t *testing.T) {
	n := NewMessageNotifier()
	_, cancel := n.Subscribe(entities.UserID("user-1"))
	defer cancel()

	// the buffered channel holds one signal; extra notifies must not block
	for i := 0; i < 10; i++ {
		n.Notify(entities.UserID("user-1"))
	}
}
