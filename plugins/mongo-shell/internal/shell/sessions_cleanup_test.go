package shell

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"
)

type cleanupDeadlineClient struct {
	*fakeClient
	bounded bool
}

func (c *cleanupDeadlineClient) Disconnect(ctx context.Context) error {
	deadline, ok := ctx.Deadline()
	c.bounded = ok && time.Until(deadline) <= disconnectTimeout
	return c.fakeClient.Disconnect(ctx)
}

func TestSessionsFailedConnectCleanupHasDeadline(t *testing.T) {
	for _, failureAt := range []string{"ping", "list databases"} {
		t.Run(failureAt, func(t *testing.T) {
			c := &cleanupDeadlineClient{fakeClient: newFakeClient("app")}
			if failureAt == "ping" {
				c.pingErr = errors.New("unreachable")
			} else {
				c.listErr = errors.New("forbidden")
			}
			runtime := NewSessions()
			runtime.factory = func(ConnectionConfig) (Client, error) { return c, nil }
			t.Cleanup(func() { runtime.Close(context.Background()) })
			value, err := runtime.Connect(context.Background(), json.RawMessage(`{"config":{"type":"mongodb","host":"db"}}`))
			if err != nil {
				t.Fatal(err)
			}
			errorOf(t, decodeOutcome(t, value))
			if !c.bounded || !c.disconnected {
				t.Fatal("failed connect cleanup has no bounded deadline")
			}
		})
	}
}
