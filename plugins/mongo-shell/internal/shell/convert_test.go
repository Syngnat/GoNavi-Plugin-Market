package shell

import (
	"testing"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
)

func TestNormalizeBSONValues(t *testing.T) {
	objectID := bson.NewObjectID()
	decimal, err := bson.ParseDecimal128("12.34")
	if err != nil {
		t.Fatalf("ParseDecimal128: %v", err)
	}
	moment := time.Date(2026, 10, 27, 8, 30, 0, 0, time.UTC)

	document := bson.D{
		{Key: "_id", Value: objectID},
		{Key: "when", Value: bson.NewDateTimeFromTime(moment)},
		{Key: "price", Value: decimal},
		{Key: "blob", Value: bson.Binary{Subtype: 0, Data: []byte("hi")}},
		{Key: "pattern", Value: bson.Regex{Pattern: "^a", Options: "i"}},
		{Key: "count", Value: int32(7)},
		{Key: "ratio", Value: float32(0.5)},
		{Key: "tags", Value: bson.A{"a", int64(2)}},
		{Key: "nested", Value: bson.D{{Key: "ok", Value: true}}},
		{Key: "nothing", Value: nil},
	}

	normalized := normalize(document).(map[string]any)
	if normalized["_id"] != objectID.Hex() {
		t.Fatalf("_id = %v, want %s", normalized["_id"], objectID.Hex())
	}
	if normalized["when"] != moment.Format(time.RFC3339Nano) {
		t.Fatalf("when = %v", normalized["when"])
	}
	if normalized["price"] != "12.34" {
		t.Fatalf("price = %v", normalized["price"])
	}
	if normalized["blob"] != "aGk=" {
		t.Fatalf("blob = %v", normalized["blob"])
	}
	regex := normalized["pattern"].(map[string]any)
	if regex["pattern"] != "^a" || regex["options"] != "i" {
		t.Fatalf("regex = %v", regex)
	}
	if normalized["count"] != int64(7) {
		t.Fatalf("count = %v", normalized["count"])
	}
	if normalized["ratio"] != float64(0.5) {
		t.Fatalf("ratio = %v", normalized["ratio"])
	}
	tags := normalized["tags"].([]any)
	if len(tags) != 2 || tags[0] != "a" || tags[1] != int64(2) {
		t.Fatalf("tags = %v", tags)
	}
	nested := normalized["nested"].(map[string]any)
	if nested["ok"] != true {
		t.Fatalf("nested = %v", nested)
	}
	if normalized["nothing"] != nil {
		t.Fatalf("nothing = %v", normalized["nothing"])
	}
}

func TestTableFromDocumentsKeepsFirstSeenColumnOrder(t *testing.T) {
	documents := []bson.D{
		{{Key: "name", Value: "a"}, {Key: "age", Value: int32(3)}},
		{{Key: "age", Value: int32(4)}, {Key: "city", Value: "x"}},
	}
	table := tableFromDocuments("users.find", documents, true)
	if len(table.Columns) != 3 || table.Columns[0] != "name" || table.Columns[1] != "age" || table.Columns[2] != "city" {
		t.Fatalf("columns = %v", table.Columns)
	}
	if !table.Truncated || table.Label != "users.find" {
		t.Fatalf("table metadata = %+v", table)
	}
	if len(table.Rows) != 2 {
		t.Fatalf("rows = %v", table.Rows)
	}
	if table.Rows[0][0] != "a" || table.Rows[0][1] != int64(3) || table.Rows[0][2] != nil {
		t.Fatalf("row 0 = %v", table.Rows[0])
	}
	if table.Rows[1][0] != nil || table.Rows[1][1] != int64(4) || table.Rows[1][2] != "x" {
		t.Fatalf("row 1 = %v", table.Rows[1])
	}
}

func TestCommandTableFallsBackToSingleRow(t *testing.T) {
	table := commandTable("runCommand", bson.D{{Key: "ok", Value: float64(1)}, {Key: "n", Value: int32(2)}})
	if len(table.Columns) != 2 || len(table.Rows) != 1 {
		t.Fatalf("table = %+v", table)
	}
	if table.Columns[0] != "ok" || table.Rows[0][0] != float64(1) {
		t.Fatalf("table = %+v", table)
	}
}

func TestTableFromNames(t *testing.T) {
	table := tableFromNames("collections", []string{"a", "b"}, true)
	if len(table.Columns) != 1 || table.Columns[0] != "name" {
		t.Fatalf("columns = %v", table.Columns)
	}
	if len(table.Rows) != 2 || table.Rows[1][0] != "b" || !table.Truncated {
		t.Fatalf("table = %+v", table)
	}
}

func TestTruncateRunes(t *testing.T) {
	if got := truncateRunes("hello", 10); got != "hello" {
		t.Fatalf("got %q", got)
	}
	if got := truncateRunes("hello", 3); got != "hel…" {
		t.Fatalf("got %q", got)
	}
}
