package shell

import (
	"context"
	"encoding/json"
	"testing"
)

type commandBackend interface {
	Connect(context.Context, json.RawMessage) (any, error)
	RunCommand(context.Context, json.RawMessage) (any, error)
	Ping(context.Context, json.RawMessage) (any, error)
	Disconnect(context.Context, json.RawMessage) (any, error)
}

func TestCommandSessionsKeepPagesIsolated(t *testing.T) {
	a, b := newFakeClient("alpha"), newFakeClient("beta")
	a.Database("alpha").(*fakeDatabase).names = []string{"only-A"}
	b.Database("beta").(*fakeDatabase).names = []string{"only-B"}
	runtime := NewSessions()
	runtime.factory = func(config ConnectionConfig) (Client, error) {
		if config.Host == "A" {
			return a, nil
		}
		return b, nil
	}
	t.Cleanup(func() { runtime.Close(context.Background()) })
	var backend commandBackend = runtime
	connect := func(host, database string, readOnly bool) map[string]any {
		raw, err := json.Marshal(connectWith(ConnectionConfig{Type: "mongodb", Host: host, Database: database}, readOnly))
		if err != nil {
			t.Fatal(err)
		}
		value, err := backend.Connect(context.Background(), raw)
		if err != nil {
			t.Fatal(err)
		}
		return dataOf(t, decodeOutcome(t, value))
	}
	first := connect("A", "alpha", true)
	second := connect("B", "beta", false)
	idA, ok := first["sessionId"].(string)
	if !ok || len(idA) < 32 {
		t.Errorf("connect A did not return an unpredictable sessionId: %v", first)
	}
	idB, ok := second["sessionId"].(string)
	if !ok || idB == idA {
		t.Errorf("connect B reused A session: %v", second)
	}
	for _, test := range []struct {
		id, database, collection string
		readOnly                 bool
	}{
		{idA, "alpha", "only-A", true}, {idB, "beta", "only-B", false},
	} {
		raw, _ := json.Marshal(map[string]any{"sessionId": test.id, "script": "db.getCollectionNames()"})
		value, err := backend.RunCommand(context.Background(), raw)
		if err != nil {
			t.Fatal(err)
		}
		data := dataOf(t, decodeOutcome(t, value))
		names := data["result"].([]any)
		if data["database"] != test.database || data["readOnly"] != test.readOnly || len(names) != 1 || names[0] != test.collection {
			t.Fatalf("session %s routed to another page: %v", test.database, data)
		}
	}
	rawWrite, err := json.Marshal(map[string]any{"sessionId": idA, "script": "db.items.insertOne({value: 1})"})
	if err != nil {
		t.Fatal(err)
	}
	valueWrite, err := backend.RunCommand(context.Background(), rawWrite)
	if err != nil {
		t.Fatal(err)
	}
	if errorOf(t, decodeOutcome(t, valueWrite))["code"] != codeReadOnly {
		t.Fatal("A lost its read-only protection after connecting B")
	}
	for _, handler := range []func(context.Context, json.RawMessage) (any, error){backend.Ping, backend.RunCommand, backend.Disconnect} {
		for _, raw := range []json.RawMessage{
			json.RawMessage(`{"script":"1+1"}`),
			json.RawMessage(`{"sessionId":"unknown","script":"1+1"}`),
			json.RawMessage(`{"sessionId":12,"script":"1+1"}`),
		} {
			value, err := handler(context.Background(), raw)
			if err != nil {
				t.Fatal(err)
			}
			errorOf(t, decodeOutcome(t, value))
		}
	}
	raw, _ := json.Marshal(map[string]any{"sessionId": idA})
	value, err := backend.Disconnect(context.Background(), raw)
	if err != nil {
		t.Fatal(err)
	}
	dataOf(t, decodeOutcome(t, value))
	if !a.disconnected || b.disconnected {
		t.Fatalf("disconnect A crossed sessions: A=%v B=%v", a.disconnected, b.disconnected)
	}
	value, err = backend.Ping(context.Background(), raw)
	if err != nil {
		t.Fatal(err)
	}
	errorOf(t, decodeOutcome(t, value))
	raw, _ = json.Marshal(map[string]any{"sessionId": idB})
	value, err = backend.Ping(context.Background(), raw)
	if err != nil {
		t.Fatal(err)
	}
	dataOf(t, decodeOutcome(t, value))
}
