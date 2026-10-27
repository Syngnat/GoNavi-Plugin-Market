package shell

import (
	"errors"
	"strings"
	"testing"

	"go.mongodb.org/mongo-driver/v2/bson"
)

func connectedFakeShell(t *testing.T, readOnly bool) (*Shell, *fakeClient, *fakeDatabase) {
	t.Helper()
	client := newFakeClient("app")
	database := client.Database("app").(*fakeDatabase)
	shell := newTestShell(t, client)
	callConnect(t, shell, connectWith(ConnectionConfig{Type: "mongodb", Host: "db", Port: 27017, Database: "app"}, readOnly))
	return shell, client, database
}

func TestRunReadOnlyRejectsWrites(t *testing.T) {
	shell, _, database := connectedFakeShell(t, true)
	collection := &fakeCollection{}
	database.collections["users"] = collection
	database.command = bson.D{{Key: "ok", Value: float64(1)}}

	scripts := []string{
		"db.users.insertOne({name: 'a'})",
		"db.users.updateOne({name: 'a'}, {$set: {name: 'b'}})",
		"db.users.deleteOne({name: 'a'})",
		"db.users.aggregate([{$out: 'copy'}])",
		"db.runCommand({insert: 'users', documents: [{a: 1}]})",
		"db.runCommand({aggregate: 'users', pipeline: [{$merge: {into: 'copy'}}], cursor: {}})",
	}
	for _, script := range scripts {
		failure := errorOf(t, callRun(t, shell, runArgs{Script: script}))
		if failure["code"] != codeReadOnly {
			t.Fatalf("%s: code = %v (%v)", script, failure["code"], failure)
		}
	}
	if len(collection.inserted) != 0 || collection.updateDocument != nil || collection.deleteFilter != nil {
		t.Fatal("read-only session reached a write method")
	}
	if database.commandCalls != 0 {
		t.Fatalf("read-only session ran %d mutating commands", database.commandCalls)
	}

	allowed := []string{
		"db.users.find({})",
		"db.users.countDocuments({})",
		"db.users.aggregate([{$match: {}}])",
		"db.runCommand({find: 'users'})",
	}
	for _, script := range allowed {
		outcome := callRun(t, shell, runArgs{Script: script})
		if outcome["ok"] != true {
			t.Fatalf("%s should be allowed in read-only mode: %v", script, outcome)
		}
	}
}

func TestRunWriteOperations(t *testing.T) {
	shell, _, database := connectedFakeShell(t, false)
	collection := &fakeCollection{insertID: bson.ObjectID{}, updateMatched: 1, updateModified: 1, deletedCount: 1}
	database.collections["users"] = collection

	insert := dataOf(t, callRun(t, shell, runArgs{Script: "db.users.insertOne({name: 'a'})"}))
	insertResult := insert["result"].(map[string]any)
	if insertResult["acknowledged"] != true {
		t.Fatalf("insert result = %v", insertResult)
	}
	if len(collection.inserted) != 1 {
		t.Fatalf("inserted = %v", collection.inserted)
	}
	insertTable := tableOf(t, insert)
	if insertTable["label"] != "users.insertOne" {
		t.Fatalf("table label = %v", insertTable["label"])
	}

	update := dataOf(t, callRun(t, shell, runArgs{
		Script: "db.users.updateOne({name: 'a'}, {$set: {age: 3}}, {upsert: true})",
	}))
	updateResult := update["result"].(map[string]any)
	if updateResult["matchedCount"] != float64(1) || updateResult["modifiedCount"] != float64(1) {
		t.Fatalf("update result = %v", updateResult)
	}
	if !collection.updateUpsert {
		t.Fatal("upsert option was not forwarded")
	}

	deletion := dataOf(t, callRun(t, shell, runArgs{Script: "db.users.deleteOne({name: 'a'})"}))
	deletionResult := deletion["result"].(map[string]any)
	if deletionResult["deletedCount"] != float64(1) {
		t.Fatalf("delete result = %v", deletionResult)
	}
}

func TestRunReadHelpers(t *testing.T) {
	shell, _, database := connectedFakeShell(t, false)
	collection := &fakeCollection{
		docs: []bson.D{
			{{Key: "_id", Value: int32(1)}, {Key: "name", Value: "a"}},
			{{Key: "_id", Value: int32(2)}, {Key: "name", Value: "b"}},
		},
		findOneDoc:   bson.D{{Key: "_id", Value: int32(2)}, {Key: "name", Value: "b"}},
		findOneFound: true,
		countValue:   2,
	}
	database.collections["users"] = collection

	one := dataOf(t, callRun(t, shell, runArgs{Script: "db.users.findOne({_id: 2})"}))
	result := one["result"].(map[string]any)
	if result["name"] != "b" {
		t.Fatalf("findOne result = %v", result)
	}
	if len(rowsOf(t, tableOf(t, one))) != 1 {
		t.Fatalf("findOne table = %v", tableOf(t, one))
	}

	collection.findOneFound = false
	missing := dataOf(t, callRun(t, shell, runArgs{Script: "db.users.findOne({_id: 9})"}))
	if missing["result"] != nil {
		t.Fatalf("missing findOne result = %v", missing["result"])
	}
	if len(rowsOf(t, tableOf(t, missing))) != 0 {
		t.Fatalf("missing findOne table = %v", tableOf(t, missing))
	}

	count := dataOf(t, callRun(t, shell, runArgs{Script: "db.users.countDocuments({})"}))
	if count["result"] != float64(2) {
		t.Fatalf("count result = %v", count["result"])
	}
	if tableOf(t, count)["columns"].([]any)[0] != "count" {
		t.Fatalf("count table = %v", tableOf(t, count))
	}

	resolved := dataOf(t, callRun(t, shell, runArgs{Script: "db.getCollection('users').find({})"}))
	if len(rowsOf(t, tableOf(t, resolved))) != 2 {
		t.Fatalf("getCollection table = %v", tableOf(t, resolved))
	}
}

func TestRunCommandCursorShowsRows(t *testing.T) {
	shell, _, database := connectedFakeShell(t, false)
	database.command = bson.D{
		{Key: "cursor", Value: bson.D{
			{Key: "firstBatch", Value: bson.A{
				bson.D{{Key: "_id", Value: int32(1)}, {Key: "name", Value: "a"}},
				bson.D{{Key: "_id", Value: int32(2)}, {Key: "name", Value: "b"}},
			}},
			{Key: "id", Value: int64(0)},
			{Key: "ns", Value: "app.users"},
		}},
		{Key: "ok", Value: float64(1)},
	}
	data := dataOf(t, callRun(t, shell, runArgs{Script: "db.runCommand({find: 'users'})"}))
	table := tableOf(t, data)
	if table["label"] != "runCommand" {
		t.Fatalf("table label = %v", table["label"])
	}
	if len(rowsOf(t, table)) != 2 {
		t.Fatalf("cursor rows = %v", rowsOf(t, table))
	}
	columns := table["columns"].([]any)
	if len(columns) != 2 || columns[0] != "_id" || columns[1] != "name" {
		t.Fatalf("cursor columns = %v", columns)
	}
}

func TestRunCollectionNameLimit(t *testing.T) {
	shell, _, database := connectedFakeShell(t, false)
	names := make([]string, 0, maxCollectionNames+5)
	for index := 0; index <= maxCollectionNames; index++ {
		names = append(names, "c"+strconvItoa(index))
	}
	database.names = names

	data := dataOf(t, callRun(t, shell, runArgs{Script: "db.getCollectionNames()"}))
	table := tableOf(t, data)
	rows := rowsOf(t, table)
	if len(rows) != maxCollectionNames {
		t.Fatalf("rows = %d, want %d", len(rows), maxCollectionNames)
	}
	if table["truncated"] != true {
		t.Fatalf("table should be truncated: %v", table)
	}
	if result := data["result"].([]any); len(result) != maxCollectionNames {
		t.Fatalf("result length = %d", len(result))
	}
}

func TestRunOutputLineLimit(t *testing.T) {
	shell, _, _ := connectedFakeShell(t, false)
	data := dataOf(t, callRun(t, shell, runArgs{Script: "for (let i = 0; i < 600; i++) { print('line ' + i) }"}))
	output := data["output"].([]any)
	if len(output) != maxOutputLines+1 {
		t.Fatalf("output lines = %d, want %d", len(output), maxOutputLines+1)
	}
	last := output[len(output)-1].(string)
	if !strings.Contains(last, "truncated") {
		t.Fatalf("last output line = %q", last)
	}
}

func TestRunRejectsInvalidArguments(t *testing.T) {
	shell, _, database := connectedFakeShell(t, false)
	database.collections["users"] = &fakeCollection{}
	scripts := []string{
		"db.users.find(42)",
		"db.users.aggregate({stage: 1})",
		"db.getCollection('')",
		"db.users.updateOne({}, null)",
	}
	for _, script := range scripts {
		failure := errorOf(t, callRun(t, shell, runArgs{Script: script}))
		if failure["code"] != codeInvalidArgument {
			t.Fatalf("%s: code = %v (%v)", script, failure["code"], failure)
		}
	}
}

func TestRunQueryFailureCode(t *testing.T) {
	shell, _, database := connectedFakeShell(t, false)
	database.collections["users"] = &fakeCollection{findErr: errors.New("boom")}
	failure := errorOf(t, callRun(t, shell, runArgs{Script: "db.users.find({})"}))
	if failure["code"] != codeQueryFailed || !strings.Contains(failure["message"].(string), "boom") {
		t.Fatalf("failure = %v", failure)
	}
}

func strconvItoa(value int) string {
	if value == 0 {
		return "0"
	}
	digits := make([]byte, 0, 6)
	for value > 0 {
		digits = append([]byte{byte('0' + value%10)}, digits...)
		value /= 10
	}
	return string(digits)
}
