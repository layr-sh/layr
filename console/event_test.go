package console

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestConsoleEventUnit(t *testing.T) {
	t.Parallel()

	t.Run("config updated event", func(t *testing.T) {
		t.Parallel()
		config := DefaultConfig()
		event := NewConfigUpdatedEvent("console.config", ConfigUpdatedEventData(config))
		require.Equal(t, "console.config.updated", event.Type)
		require.NotNil(t, event.ResourceID)
		require.Equal(t, "console.config", *event.ResourceID)
	})

	t.Run("user lifecycle events", func(t *testing.T) {
		t.Parallel()
		user := User{
			ID:        "usr_123",
			Email:     "operator@example.com",
			IsEnabled: true,
			CreatedAt: time.Now().UTC(),
		}

		createdEvent := NewUserCreatedEvent(user.ID, UserCreatedEventData(user))
		require.Equal(t, "console.user.created", createdEvent.Type)
		require.NotNil(t, createdEvent.ResourceID)
		require.Equal(t, user.ID, *createdEvent.ResourceID)

		updatedEvent := NewUserUpdatedEvent(user.ID, UserUpdatedEventData(user))
		require.Equal(t, "console.user.updated", updatedEvent.Type)
		require.NotNil(t, updatedEvent.ResourceID)
		require.Equal(t, user.ID, *updatedEvent.ResourceID)

		deletedEvent := NewUserDeletedEvent(user.ID, UserDeletedEventData{
			User:                 user,
			RevokedSessionsCount: 2,
		})
		require.Equal(t, "console.user.deleted", deletedEvent.Type)
		require.NotNil(t, deletedEvent.ResourceID)
		require.Equal(t, user.ID, *deletedEvent.ResourceID)

		resetEvent := NewUserPasswordResetEvent(user.ID, UserPasswordResetEventData{User: user})
		require.Equal(t, "console.user.password_reset", resetEvent.Type)
		require.NotNil(t, resetEvent.ResourceID)
		require.Equal(t, user.ID, *resetEvent.ResourceID)
	})

	t.Run("session lifecycle events", func(t *testing.T) {
		t.Parallel()
		user := User{
			ID:    "usr_123",
			Email: "operator@example.com",
		}
		now := time.Now().UTC()

		createdEvent := NewSessionCreatedEvent("sess_123", SessionCreatedEventData{
			ID:        "sess_123",
			User:      user,
			IPAddress: "127.0.0.1",
			UserAgent: "TestAgent",
			ExpiresAt: now.Add(time.Hour),
			CreatedAt: now,
		})
		require.Equal(t, "console.session.created", createdEvent.Type)
		require.NotNil(t, createdEvent.ResourceID)
		require.Equal(t, "sess_123", *createdEvent.ResourceID)

		revokedEvent := NewSessionRevokedEvent("sess_123", SessionRevokedEventData{
			SessionID: "sess_123",
			UserID:    user.ID,
			Reason:    "manual_revocation",
		})
		require.Equal(t, "console.session.revoked", revokedEvent.Type)
		require.NotNil(t, revokedEvent.ResourceID)
		require.Equal(t, "sess_123", *revokedEvent.ResourceID)
	})
}
