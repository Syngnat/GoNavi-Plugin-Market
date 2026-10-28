package shell

import (
	"context"
	"encoding/json"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func sessionRaw(t *testing.T, id, script string) json.RawMessage {
	t.Helper()
	raw, err := json.Marshal(map[string]any{"sessionId": id, "script": script})
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func sessionConnect(t *testing.T, runtime *Sessions) string {
	t.Helper()
	value, err := runtime.Connect(context.Background(), json.RawMessage(`{"config":{"type":"mongodb","host":"db","database":"app"}}`))
	if err != nil {
		t.Fatal(err)
	}
	return dataOf(t, decodeOutcome(t, value))["sessionId"].(string)
}

func TestSessionsConcurrentLimitAndSlotReuse(t *testing.T) {
	runtime := NewSessions()
	var created atomic.Int64
	runtime.factory = func(ConnectionConfig) (Client, error) { created.Add(1); return newFakeClient("app"), nil }
	t.Cleanup(func() { runtime.Close(context.Background()) })
	var wg sync.WaitGroup
	ids := make(chan string, maxSessions*2)
	for range maxSessions * 2 {
		wg.Go(func() {
			value, err := runtime.Connect(context.Background(), json.RawMessage(`{"config":{"type":"mongodb","host":"db"}}`))
			if err != nil {
				t.Error(err)
				return
			}
			outcome := decodeOutcome(t, value)
			if outcome["ok"] == true {
				ids <- dataOf(t, outcome)["sessionId"].(string)
				return
			}
			if errorOf(t, outcome)["code"] != "session_limit" {
				t.Errorf("unexpected rejection: %v", outcome)
			}
		})
	}
	wg.Wait()
	close(ids)
	if len(ids) != maxSessions || created.Load() != maxSessions {
		t.Fatalf("admitted=%d clients=%d want %d", len(ids), created.Load(), maxSessions)
	}
	unique := make(map[string]bool)
	for id := range ids {
		if unique[id] {
			t.Fatalf("duplicate sessionId %q", id)
		}
		unique[id] = true
	}
	var first string
	for id := range unique {
		first = id
		break
	}
	value, err := runtime.Disconnect(context.Background(), sessionRaw(t, first, ""))
	if err != nil {
		t.Fatal(err)
	}
	dataOf(t, decodeOutcome(t, value))
	next := sessionConnect(t, runtime)
	if unique[next] {
		t.Fatal("replacement reused a sessionId")
	}
}

func TestSessionsDisconnectCancelsOnlyItsRunningScript(t *testing.T) {
	runtime := NewSessions()
	entered := make(chan struct{})
	first := true
	runtime.factory = func(ConnectionConfig) (Client, error) {
		if first {
			first = false
			return &blockingNamesClient{fakeClient: newFakeClient("app"), entered: entered}, nil
		}
		return newFakeClient("app"), nil
	}
	t.Cleanup(func() { runtime.Close(context.Background()) })
	a, b := sessionConnect(t, runtime), sessionConnect(t, runtime)
	done := make(chan struct{})
	raw := sessionRaw(t, a, "db.getCollectionNames()")
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	go func() { defer close(done); runtime.RunCommand(ctx, raw) }()
	<-entered
	started := time.Now()
	value, err := runtime.Disconnect(context.Background(), sessionRaw(t, a, ""))
	if err != nil {
		t.Fatal(err)
	}
	dataOf(t, decodeOutcome(t, value))
	if time.Since(started) > time.Second {
		t.Fatal("disconnect waited for the request timeout instead of cancelling it")
	}
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("disconnect did not cancel the script")
	}
	value, err = runtime.RunCommand(context.Background(), sessionRaw(t, b, "db.getName()"))
	if err != nil {
		t.Fatal(err)
	}
	if dataOf(t, decodeOutcome(t, value))["result"] != "app" {
		t.Fatal("B changed after disconnect A")
	}
}

type lateConnectClient struct {
	*fakeClient
	entered chan struct{}
	release chan struct{}
}

func (c *lateConnectClient) Ping(context.Context) error { close(c.entered); <-c.release; return nil }

func TestSessionsAbortFencesLateConnectAndCloseReleasesClient(t *testing.T) {
	client := &lateConnectClient{fakeClient: newFakeClient("app"), entered: make(chan struct{}), release: make(chan struct{})}
	runtime := NewSessions()
	runtime.factory = func(ConnectionConfig) (Client, error) { return client, nil }
	done := make(chan any, 1)
	go func() {
		value, _ := runtime.Connect(context.Background(), json.RawMessage(`{"config":{"type":"mongodb","host":"db"}}`))
		done <- value
	}()
	<-client.entered
	runtime.Abort()
	runtime.Close(context.Background())
	close(client.release)
	select {
	case value := <-done:
		errorOf(t, decodeOutcome(t, value))
	case <-time.After(2 * time.Second):
		t.Fatal("late connect did not return")
	}
	client.mu.Lock()
	disconnected := client.disconnected
	client.mu.Unlock()
	if !disconnected {
		t.Fatal("late connect leaked its client")
	}
	value, err := runtime.Ping(context.Background(), sessionRaw(t, "unknown", ""))
	if err != nil {
		t.Fatal(err)
	}
	errorOf(t, decodeOutcome(t, value))
}

func TestSessionsCloseCancelsWorkAndReleasesAllClients(t *testing.T) {
	runtime := NewSessions()
	var clients []*fakeClient
	runtime.factory = func(ConnectionConfig) (Client, error) {
		c := newFakeClient("app")
		clients = append(clients, c)
		return c, nil
	}
	a := sessionConnect(t, runtime)
	sessionConnect(t, runtime)
	done := make(chan struct{})
	raw := sessionRaw(t, a, "while (true) {}")
	go func() { defer close(done); runtime.RunCommand(context.Background(), raw) }()
	runtime.Abort()
	runtime.Close(context.Background())
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("close did not cancel work")
	}
	for _, c := range clients {
		c.mu.Lock()
		closed := c.disconnected
		c.mu.Unlock()
		if !closed {
			t.Fatal("close leaked a client")
		}
	}
}

// The entry signal proves the real script has reached the driver boundary.
type blockingNamesClient struct {
	*fakeClient
	entered chan struct{}
}

func (c *blockingNamesClient) Database(name string) Database {
	return &blockingNamesDatabase{Database: c.fakeClient.Database(name), entered: c.entered}
}

type blockingNamesDatabase struct {
	Database
	entered chan struct{}
}

func (d *blockingNamesDatabase) CollectionNames(ctx context.Context) ([]string, error) {
	close(d.entered)
	<-ctx.Done()
	return nil, ctx.Err()
}
