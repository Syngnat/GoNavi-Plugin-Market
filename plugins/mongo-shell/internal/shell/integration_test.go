package shell

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"
)

// TestRealMongoRoundTrip talks to a real deployment. It is skipped unless
// MONGO_SHELL_TEST_URI points at a disposable MongoDB instance, for example:
//
//	MONGO_SHELL_TEST_URI=mongodb://127.0.0.1:27018 go test ./internal/shell -run TestRealMongoRoundTrip
//
// The test creates a uniquely named database, exercises every documented helper
// and drops the database before returning.
func TestRealMongoRoundTrip(t *testing.T) {
	uri := strings.TrimSpace(os.Getenv("MONGO_SHELL_TEST_URI"))
	if uri == "" {
		t.Skip("set MONGO_SHELL_TEST_URI to run the real MongoDB round trip")
	}
	database := fmt.Sprintf("gonavi_shell_test_%d", time.Now().UnixNano())
	shell := New()
	t.Cleanup(shell.Abort)
	t.Cleanup(func() { dropDatabase(t, uri, database) })

	data := dataOf(t, callConnect(t, shell, connectArgs{
		Config:   &ConnectionConfig{Type: "mongodb", URI: uri, Database: database},
		Database: database,
	}))
	if data["readOnly"] != false {
		t.Fatalf("readOnly = %v", data["readOnly"])
	}

	inserted := dataOf(t, callRun(t, shell, runArgs{Script: `db.people.insertOne({name: "ada", age: 36})`}))
	if inserted["result"] == nil {
		t.Fatalf("insert result = %v", inserted)
	}
	if label := tableOf(t, inserted)["label"]; label != "people.insertOne" {
		t.Fatalf("insert table = %v", label)
	}

	found := dataOf(t, callRun(t, shell, runArgs{Script: `db.people.find({}, {}, 10)`}))
	if rows := rowsOf(t, tableOf(t, found)); len(rows) != 1 {
		t.Fatalf("find rows = %v", rows)
	}

	one := dataOf(t, callRun(t, shell, runArgs{Script: `db.people.findOne({name: "ada"})`}))
	if document, ok := one["result"].(map[string]any); !ok || document["name"] != "ada" {
		t.Fatalf("findOne = %v", one["result"])
	}

	count := dataOf(t, callRun(t, shell, runArgs{Script: `db.people.countDocuments({})`}))
	if count["result"] != float64(1) {
		t.Fatalf("count = %v", count["result"])
	}

	updated := dataOf(t, callRun(t, shell, runArgs{
		Script: `db.people.updateOne({name: "ada"}, {$set: {age: 37}}); db.people.findOne({name: "ada"})`,
	}))
	if document, ok := updated["result"].(map[string]any); !ok || document["age"] != float64(37) {
		t.Fatalf("update = %v", updated["result"])
	}

	aggregated := dataOf(t, callRun(t, shell, runArgs{
		Script: `db.people.aggregate([{$match: {name: "ada"}}, {$project: {_id: 0, name: 1}}])`,
	}))
	if rows := rowsOf(t, tableOf(t, aggregated)); len(rows) != 1 {
		t.Fatalf("aggregate rows = %v", rows)
	}

	command := dataOf(t, callRun(t, shell, runArgs{Script: `db.runCommand({count: "people"})`}))
	if result, ok := command["result"].(map[string]any); !ok || result["n"] != float64(1) {
		t.Fatalf("runCommand = %v", command["result"])
	}

	deleted := dataOf(t, callRun(t, shell, runArgs{Script: `db.people.deleteOne({name: "ada"})`}))
	if result, ok := deleted["result"].(map[string]any); !ok || result["deletedCount"] != float64(1) {
		t.Fatalf("delete = %v", deleted["result"])
	}

	if _, err := shell.Disconnect(context.Background(), nil); err != nil {
		t.Fatalf("disconnect: %v", err)
	}
	readOnlyData := dataOf(t, callConnect(t, shell, connectArgs{
		Config:   &ConnectionConfig{Type: "mongodb", URI: uri, Database: database},
		Database: database,
		ReadOnly: true,
	}))
	if readOnlyData["readOnly"] != true {
		t.Fatalf("read-only connect = %v", readOnlyData["readOnly"])
	}
	blocked := errorOf(t, callRun(t, shell, runArgs{Script: `db.people.insertOne({name: "blocked"})`}))
	if blocked["code"] != codeReadOnly {
		t.Fatalf("read-only insert = %v", blocked)
	}
}

func dropDatabase(t *testing.T, uri, database string) {
	t.Helper()
	client := New()
	defer client.Abort()
	connectRaw, _ := json.Marshal(connectArgs{
		Config:   &ConnectionConfig{Type: "mongodb", URI: uri, Database: database},
		Database: database,
	})
	if _, err := client.Connect(context.Background(), connectRaw); err != nil {
		t.Logf("cleanup connect failed: %v", err)
		return
	}
	dropRaw, _ := json.Marshal(runArgs{Script: "db.runCommand({dropDatabase: 1})"})
	if _, err := client.RunCommand(context.Background(), dropRaw); err != nil {
		t.Logf("cleanup drop failed: %v", err)
	}
}
