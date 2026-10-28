package shell

import (
	"context"
	"encoding/json"
	"testing"
	"time"
)

// A closing client still consumes capacity until bounded cleanup finishes.
func TestSessionsClosingClientStillConsumesCapacity(t *testing.T) {
	client := &slowCloseClient{fakeClient: newFakeClient("app"), entered: make(chan struct{}), release: make(chan struct{})}
	runtime := NewSessions()
	runtime.limit = 1
	created := false
	runtime.factory = func(ConnectionConfig) (Client, error) {
		if !created {
			created = true
			return client, nil
		}
		return newFakeClient("app"), nil
	}
	id := sessionConnect(t, runtime)
	done := make(chan struct{})
	go func() { defer close(done); runtime.Disconnect(context.Background(), sessionRaw(t, id, "")) }()
	<-client.entered
	value, err := runtime.Connect(context.Background(), json.RawMessage(`{"config":{"type":"mongodb","host":"db"}}`))
	close(client.release)
	<-done
	runtime.Close(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	failure := errorOf(t, decodeOutcome(t, value))
	if failure["code"] != "session_limit" {
		t.Fatalf("closing slot not reserved: %v", failure)
	}
}

type slowCloseClient struct {
	*fakeClient
	entered chan struct{}
	release chan struct{}
}

func (c *slowCloseClient) Disconnect(ctx context.Context) error {
	close(c.entered)
	select {
	case <-c.release:
	case <-ctx.Done():
		return ctx.Err()
	}
	return c.fakeClient.Disconnect(ctx)
}

func TestSessionsReclaimAbandonedIdleClientWithoutReplacingFreshSession(t *testing.T) {
	runtime := NewSessions()
	runtime.limit = 1
	now := time.Now()
	runtime.now = func() time.Time { return now }
	var clients []*fakeClient
	runtime.factory = func(ConnectionConfig) (Client, error) {
		c := newFakeClient("app")
		clients = append(clients, c)
		return c, nil
	}
	t.Cleanup(func() { runtime.Close(context.Background()) })
	old := sessionConnect(t, runtime)
	raw := json.RawMessage(`{"config":{"type":"mongodb","host":"db","database":"app"}}`)
	value, err := runtime.Connect(context.Background(), raw)
	if err != nil {
		t.Fatal(err)
	}
	if errorOf(t, decodeOutcome(t, value))["code"] != "session_limit" {
		t.Fatal("fresh session was replaced")
	}
	now = now.Add(idleSessionTimeout + time.Second)
	next := sessionConnect(t, runtime)
	if old == next || !clients[0].disconnected {
		t.Fatal("abandoned idle client was not reclaimed")
	}
	value, err = runtime.Ping(context.Background(), sessionRaw(t, old, ""))
	if err != nil {
		t.Fatal(err)
	}
	errorOf(t, decodeOutcome(t, value))
	value, err = runtime.Ping(context.Background(), sessionRaw(t, next, ""))
	if err != nil {
		t.Fatal(err)
	}
	dataOf(t, decodeOutcome(t, value))
}
