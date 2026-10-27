package shell

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
)

func newTestShell(t *testing.T, client Client) *Shell {
	t.Helper()
	shell := New()
	shell.factory = func(ConnectionConfig) (Client, error) { return client, nil }
	t.Cleanup(shell.Abort)
	return shell
}

func connectWith(config ConnectionConfig, readOnly bool) connectArgs {
	return connectArgs{
		Config:         &config,
		ReadOnly:       readOnly,
		ConnectionName: "local",
	}
}

func callConnect(t *testing.T, shell *Shell, args connectArgs) map[string]any {
	t.Helper()
	raw, err := json.Marshal(args)
	if err != nil {
		t.Fatalf("marshal connect args: %v", err)
	}
	value, err := shell.Connect(context.Background(), raw)
	if err != nil {
		t.Fatalf("connect returned a Go error: %v", err)
	}
	return decodeOutcome(t, value)
}

func callRun(t *testing.T, shell *Shell, args runArgs) map[string]any {
	t.Helper()
	raw, err := json.Marshal(args)
	if err != nil {
		t.Fatalf("marshal run args: %v", err)
	}
	value, err := shell.RunCommand(context.Background(), raw)
	if err != nil {
		t.Fatalf("run returned a Go error: %v", err)
	}
	return decodeOutcome(t, value)
}

func decodeOutcome(t *testing.T, value any) map[string]any {
	t.Helper()
	encoded, err := json.Marshal(value)
	if err != nil {
		t.Fatalf("marshal outcome: %v", err)
	}
	var decoded map[string]any
	if err := json.Unmarshal(encoded, &decoded); err != nil {
		t.Fatalf("unmarshal outcome: %v", err)
	}
	return decoded
}

func dataOf(t *testing.T, outcome map[string]any) map[string]any {
	t.Helper()
	if outcome["ok"] != true {
		t.Fatalf("expected ok outcome, got %v", outcome)
	}
	data, ok := outcome["data"].(map[string]any)
	if !ok {
		t.Fatalf("outcome has no data object: %v", outcome)
	}
	return data
}

func errorOf(t *testing.T, outcome map[string]any) map[string]any {
	t.Helper()
	if outcome["ok"] != false {
		t.Fatalf("expected failure outcome, got %v", outcome)
	}
	failure, ok := outcome["error"].(map[string]any)
	if !ok {
		t.Fatalf("outcome has no error object: %v", outcome)
	}
	return failure
}

func tableOf(t *testing.T, data map[string]any) map[string]any {
	t.Helper()
	table, ok := data["table"].(map[string]any)
	if !ok {
		t.Fatalf("run data has no table: %v", data)
	}
	return table
}

func rowsOf(t *testing.T, table map[string]any) []any {
	t.Helper()
	rows, ok := table["rows"].([]any)
	if !ok {
		t.Fatalf("table has no rows: %v", table)
	}
	return rows
}

func TestConnectReturnsDatabasesAndNeverEchoesCredentials(t *testing.T) {
	client := newFakeClient("admin", "app")
	client.databases[1].SizeOnDisk = 2048
	shell := newTestShell(t, client)
	outcome := callConnect(t, shell, connectWith(
		ConnectionConfig{Type: "mongodb", Host: "localhost", Port: 27017, User: "root", Password: "secret", Database: "app"},
		true,
	))
	data := dataOf(t, outcome)
	if data["readOnly"] != true {
		t.Fatalf("readOnly = %v, want true", data["readOnly"])
	}
	if data["host"] != "localhost:27017" {
		t.Fatalf("host = %v", data["host"])
	}
	if data["database"] != "app" {
		t.Fatalf("database = %v", data["database"])
	}
	databases, ok := data["databases"].([]any)
	if !ok || len(databases) != 2 {
		t.Fatalf("databases = %v", data["databases"])
	}
	if client.pingCalls != 1 {
		t.Fatalf("ping calls = %d, want 1", client.pingCalls)
	}
	encoded, _ := json.Marshal(outcome)
	if strings.Contains(string(encoded), "secret") {
		t.Fatalf("connect result leaked the password: %s", encoded)
	}
}

func TestConnectValidation(t *testing.T) {
	tests := []struct {
		name string
		args connectArgs
		want string
	}{
		{name: "missing config", args: connectArgs{}, want: codeInvalidArgument},
		{name: "wrong type", args: connectWith(ConnectionConfig{Type: "postgres"}, false), want: codeUnsupportedConnection},
		{name: "ssh tunnel", args: connectWith(ConnectionConfig{Type: "mongodb", UseSSH: true}, false), want: codeUnsupportedTransport},
		{name: "proxy", args: connectWith(ConnectionConfig{Type: "mongodb", UseProxy: true}, false), want: codeUnsupportedTransport},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			shell := newTestShell(t, newFakeClient("admin"))
			outcome := callConnect(t, shell, test.args)
			failure := errorOf(t, outcome)
			if failure["code"] != test.want {
				t.Fatalf("code = %v, want %v (%v)", failure["code"], test.want, failure)
			}
		})
	}
}

func TestConnectDriverFailureMarksClientDisconnected(t *testing.T) {
	client := newFakeClient("admin")
	client.pingErr = errors.New("server selection failed")
	shell := newTestShell(t, client)
	failure := errorOf(t, callConnect(t, shell, connectWith(ConnectionConfig{Type: "mongodb", Host: "db", Port: 27017}, false)))
	if failure["code"] != codeConnectionFailed {
		t.Fatalf("code = %v", failure["code"])
	}
	if !client.disconnected {
		t.Fatal("failed connect did not release the client")
	}
}

func TestPingAndDisconnectLifecycle(t *testing.T) {
	client := newFakeClient("admin")
	shell := newTestShell(t, client)

	before, err := shell.Ping(context.Background(), nil)
	if err != nil {
		t.Fatalf("ping error: %v", err)
	}
	if failure := errorOf(t, decodeOutcome(t, before)); failure["code"] != codeNotConnected {
		t.Fatalf("ping before connect = %v", failure)
	}

	dataOf(t, callConnect(t, shell, connectWith(ConnectionConfig{Type: "mongodb", Host: "db", Port: 27017, Database: "admin"}, false)))
	pingValue, err := shell.Ping(context.Background(), nil)
	if err != nil {
		t.Fatalf("ping error: %v", err)
	}
	pingData := dataOf(t, decodeOutcome(t, pingValue))
	if _, ok := pingData["latencyMs"].(float64); !ok {
		t.Fatalf("ping latency missing: %v", pingData)
	}

	disconnectValue, err := shell.Disconnect(context.Background(), nil)
	if err != nil {
		t.Fatalf("disconnect error: %v", err)
	}
	dataOf(t, decodeOutcome(t, disconnectValue))
	if !client.disconnected {
		t.Fatal("disconnect did not close the client")
	}

	after, err := shell.Ping(context.Background(), nil)
	if err != nil {
		t.Fatalf("ping error: %v", err)
	}
	if failure := errorOf(t, decodeOutcome(t, after)); failure["code"] != codeNotConnected {
		t.Fatalf("ping after disconnect = %v", failure)
	}
}

func TestRunWithoutConnectionFails(t *testing.T) {
	shell := New()
	t.Cleanup(shell.Abort)
	failure := errorOf(t, callRun(t, shell, runArgs{Script: "1+1"}))
	if failure["code"] != codeNotConnected {
		t.Fatalf("code = %v", failure["code"])
	}
}

func TestRunFindRespectsRowLimits(t *testing.T) {
	client := newFakeClient("app")
	collection := &fakeCollection{docs: []bson.D{
		{{Key: "_id", Value: int32(1)}, {Key: "name", Value: "a"}},
		{{Key: "_id", Value: int32(2)}, {Key: "name", Value: "b"}},
		{{Key: "_id", Value: int32(3)}, {Key: "name", Value: "c"}},
		{{Key: "_id", Value: int32(4)}, {Key: "name", Value: "d"}},
		{{Key: "_id", Value: int32(5)}, {Key: "name", Value: "e"}},
	}}
	client.Database("app").(*fakeDatabase).collections["users"] = collection
	shell := newTestShell(t, client)
	callConnect(t, shell, connectWith(ConnectionConfig{Type: "mongodb", Host: "db", Database: "app"}, false))

	outcome := callRun(t, shell, runArgs{Script: "db.users.find({}, {name: 1}, 2)"})
	data := dataOf(t, outcome)
	table := tableOf(t, data)
	rows := rowsOf(t, table)
	if len(rows) != 2 {
		t.Fatalf("rows = %d, want 2", len(rows))
	}
	if table["truncated"] != true {
		t.Fatalf("table should be truncated: %v", table)
	}
	if collection.findLimit != 2 {
		t.Fatalf("find limit = %d, want 2", collection.findLimit)
	}
	if collection.findProjection == nil {
		t.Fatal("projection was not forwarded")
	}
	result, ok := data["result"].([]any)
	if !ok || len(result) != 2 {
		t.Fatalf("result = %v", data["result"])
	}

	defaultOutcome := dataOf(t, callRun(t, shell, runArgs{Script: "db.users.find({})"}))
	if collection.findLimit != defaultMaxRows {
		t.Fatalf("default find limit = %d, want %d", collection.findLimit, defaultMaxRows)
	}
	if defaultOutcome["maxRows"] != float64(defaultMaxRows) {
		t.Fatalf("maxRows = %v", defaultOutcome["maxRows"])
	}

	clamped := dataOf(t, callRun(t, shell, runArgs{Script: "db.users.find({})", MaxRows: 5000}))
	if clamped["maxRows"] != float64(maxRowsLimit) {
		t.Fatalf("clamped maxRows = %v", clamped["maxRows"])
	}
}

func TestRunShellHelpersAndUse(t *testing.T) {
	client := newFakeClient("test", "admin")
	admin := client.Database("admin").(*fakeDatabase)
	admin.names = []string{"logs", "users"}
	shell := newTestShell(t, client)
	callConnect(t, shell, connectWith(ConnectionConfig{Type: "mongodb", Host: "db", Database: "test"}, false))

	data := dataOf(t, callRun(t, shell, runArgs{
		Script: "print('hello', 42)\nuse('admin')\nprint(db.getName())\ndb.getCollectionNames()",
	}))
	if data["database"] != "admin" {
		t.Fatalf("database after use = %v", data["database"])
	}
	output := data["output"].([]any)
	want := []string{"hello 42", "switched to db admin", "admin"}
	if len(output) != len(want) {
		t.Fatalf("output = %v, want %v", output, want)
	}
	for index, line := range want {
		if output[index] != line {
			t.Fatalf("output[%d] = %v, want %v", index, output[index], line)
		}
	}
	table := tableOf(t, data)
	if table["label"] != "collections" {
		t.Fatalf("table label = %v", table["label"])
	}
	if len(rowsOf(t, table)) != 2 {
		t.Fatalf("collection rows = %v", rowsOf(t, table))
	}
	result, ok := data["result"].([]any)
	if !ok || len(result) != 2 || result[1] != "users" {
		t.Fatalf("result = %v", data["result"])
	}
}

func TestRunReportScriptErrors(t *testing.T) {
	shell := newTestShell(t, newFakeClient("app"))
	callConnect(t, shell, connectWith(ConnectionConfig{Type: "mongodb", Host: "db", Database: "app"}, false))

	syntax := errorOf(t, callRun(t, shell, runArgs{Script: "function ("}))
	if syntax["code"] != codeSyntaxError {
		t.Fatalf("syntax code = %v (%v)", syntax["code"], syntax)
	}
	thrown := errorOf(t, callRun(t, shell, runArgs{Script: "throw new Error('boom')"}))
	if thrown["code"] != codeScriptError || !strings.Contains(thrown["message"].(string), "boom") {
		t.Fatalf("thrown = %v", thrown)
	}
	coded := errorOf(t, callRun(t, shell, runArgs{Script: "throw {code: 'custom_code', message: 'custom message'}"}))
	if coded["code"] != "custom_code" || coded["message"] != "custom message" {
		t.Fatalf("coded = %v", coded)
	}
}

func TestRunTimeoutInterruptsInfiniteScript(t *testing.T) {
	shell := newTestShell(t, newFakeClient("app"))
	shell.commandTimeout = 150 * time.Millisecond
	callConnect(t, shell, connectWith(ConnectionConfig{Type: "mongodb", Host: "db", Database: "app"}, false))
	failure := errorOf(t, callRun(t, shell, runArgs{Script: "while (true) {}"}))
	if failure["code"] != codeTimeout {
		t.Fatalf("code = %v (%v)", failure["code"], failure)
	}
}

func TestRunTimeoutClassifiesBlockedDriverCall(t *testing.T) {
	client := newFakeClient("app")
	collection := &fakeCollection{block: true}
	client.Database("app").(*fakeDatabase).collections["users"] = collection
	shell := newTestShell(t, client)
	shell.commandTimeout = 120 * time.Millisecond
	callConnect(t, shell, connectWith(ConnectionConfig{Type: "mongodb", Host: "db", Database: "app"}, false))
	failure := errorOf(t, callRun(t, shell, runArgs{Script: "db.users.find({})"}))
	if failure["code"] != codeTimeout {
		t.Fatalf("code = %v (%v)", failure["code"], failure)
	}
}
