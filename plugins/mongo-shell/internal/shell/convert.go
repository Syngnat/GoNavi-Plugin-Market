package shell

import (
	"encoding/base64"
	"fmt"
	"math"
	"time"

	"github.com/dop251/goja"
	"go.mongodb.org/mongo-driver/v2/bson"
)

// table is the flat, JSON-safe projection the window renders below the editor.
// Only the most recent result-producing operation is kept per run.
type table struct {
	Label     string   `json:"label,omitempty"`
	Columns   []string `json:"columns"`
	Rows      [][]any  `json:"rows"`
	Truncated bool     `json:"truncated,omitempty"`
}

// normalize converts BSON values into JSON/JS-safe values. No size-based
// truncation happens here; callers bound rows, output lines and time.
func normalize(value any) any {
	switch typed := value.(type) {
	case nil:
		return nil
	case bson.D:
		return normalizeDocument(typed)
	case bson.M:
		return normalizeMap(typed)
	case bson.A:
		return normalizeSlice(typed)
	case map[string]any:
		return normalizeMap(typed)
	case []any:
		return normalizeSlice(typed)
	case bson.ObjectID:
		return typed.Hex()
	case bson.DateTime:
		return time.UnixMilli(typed.Time().UnixMilli()).UTC().Format(time.RFC3339Nano)
	case bson.Timestamp:
		return fmt.Sprintf("Timestamp(%d, %d)", typed.T, typed.I)
	case bson.Decimal128:
		return typed.String()
	case bson.Binary:
		return base64.StdEncoding.EncodeToString(typed.Data)
	case bson.Regex:
		return map[string]any{"pattern": typed.Pattern, "options": typed.Options}
	case bson.JavaScript:
		return string(typed)
	case bson.CodeWithScope:
		return map[string]any{"code": typed.Code, "scope": normalize(typed.Scope)}
	case bson.Symbol:
		return string(typed)
	case bson.MinKey:
		return "MinKey"
	case bson.MaxKey:
		return "MaxKey"
	case bson.Null, bson.Undefined:
		return nil
	case time.Time:
		return typed.UTC().Format(time.RFC3339Nano)
	case int:
		return int64(typed)
	case int8:
		return int64(typed)
	case int16:
		return int64(typed)
	case int32:
		return int64(typed)
	case int64:
		return typed
	case uint:
		return uint64(typed)
	case uint8:
		return uint64(typed)
	case uint16:
		return uint64(typed)
	case uint32:
		return uint64(typed)
	case uint64:
		return typed
	case float32:
		return float64(typed)
	case float64:
		if math.IsNaN(typed) || math.IsInf(typed, 0) {
			return fmt.Sprint(typed)
		}
		return typed
	case string, bool:
		return typed
	default:
		return fmt.Sprintf("%v", typed)
	}
}

// normalizeDocuments converts driver documents for the JS runtime.
func normalizeDocuments(documents []bson.D) []any {
	result := make([]any, len(documents))
	for index, document := range documents {
		result[index] = normalizeDocument(document)
	}
	return result
}

func normalizeMap(values map[string]any) map[string]any {
	result := make(map[string]any, len(values))
	for key, value := range values {
		result[key] = normalize(value)
	}
	return result
}

func normalizeSlice(values []any) []any {
	result := make([]any, len(values))
	for index, value := range values {
		result[index] = normalize(value)
	}
	return result
}

// normalizeDocument preserves bson.D declaration order because the window
// shows columns in the order MongoDB returned them.
func normalizeDocument(document bson.D) map[string]any {
	result := make(map[string]any, len(document))
	for _, element := range document {
		result[element.Key] = normalize(element.Value)
	}
	return result
}

func documentKeys(document bson.D) []string {
	keys := make([]string, 0, len(document))
	seen := make(map[string]bool, len(document))
	for _, element := range document {
		if seen[element.Key] {
			continue
		}
		seen[element.Key] = true
		keys = append(keys, element.Key)
	}
	return keys
}

// tableFromDocuments builds a table with a stable, first-seen column order.
func tableFromDocuments(label string, documents []bson.D, truncated bool) *table {
	columns := make([]string, 0, 8)
	index := make(map[string]int, 8)
	for _, document := range documents {
		for _, key := range documentKeys(document) {
			if _, exists := index[key]; exists {
				continue
			}
			index[key] = len(columns)
			columns = append(columns, key)
		}
	}
	rows := make([][]any, 0, len(documents))
	for _, document := range documents {
		row := make([]any, len(columns))
		for _, element := range document {
			position, exists := index[element.Key]
			if exists {
				row[position] = normalize(element.Value)
			}
		}
		rows = append(rows, row)
	}
	return &table{Label: label, Columns: columns, Rows: rows, Truncated: truncated}
}

func tableFromRow(label string, columns []string, values []any) *table {
	return &table{Label: label, Columns: columns, Rows: [][]any{values}}
}

// commandTable renders a runCommand result. Cursor-shaped replies are shown as
// rows; everything else becomes a one-row key/value table.
func commandTable(label string, document bson.D) *table {
	if batch := cursorBatch(document); len(batch) > 0 {
		return tableFromDocuments(label, batch, false)
	}
	columns := documentKeys(document)
	row := make([]any, len(columns))
	for position, key := range columns {
		row[position] = normalize(documentValue(document, key))
	}
	return tableFromRow(label, columns, row)
}

func cursorBatch(document bson.D) []bson.D {
	cursorValue, ok := lookupDocument(document, "cursor")
	if !ok {
		return nil
	}
	cursorDocument := toDocument(cursorValue)
	if cursorDocument == nil {
		return nil
	}
	for _, key := range []string{"firstBatch", "nextBatch"} {
		batchValue, ok := lookupDocument(cursorDocument, key)
		if !ok {
			continue
		}
		return toDocumentSlice(batchValue)
	}
	return nil
}

func toDocument(value any) bson.D {
	switch typed := value.(type) {
	case bson.D:
		return typed
	case bsondRaw:
		var document bson.D
		if err := bson.Unmarshal(typed, &document); err != nil {
			return nil
		}
		return document
	case map[string]any:
		document := make(bson.D, 0, len(typed))
		for key, item := range typed {
			document = append(document, bson.E{Key: key, Value: item})
		}
		return document
	default:
		return nil
	}
}

func toDocumentSlice(value any) []bson.D {
	switch typed := value.(type) {
	case bson.A:
		return documentsFromAny(typed)
	case []any:
		return documentsFromAny(typed)
	case []bson.D:
		return typed
	default:
		return nil
	}
}

func documentsFromAny(values []any) []bson.D {
	documents := make([]bson.D, 0, len(values))
	for _, value := range values {
		if document := toDocument(value); document != nil {
			documents = append(documents, document)
		}
	}
	return documents
}

func lookupDocument(document bson.D, key string) (any, bool) {
	for _, element := range document {
		if element.Key == key {
			return element.Value, true
		}
	}
	return nil, false
}

func documentValue(document bson.D, key string) any {
	value, _ := lookupDocument(document, key)
	return value
}

// jsResult converts the last script expression into JSON-safe data. Objects
// that cannot be represented (functions, the db proxy) yield nil instead of
// breaking the run.
func jsResult(vm *goja.Runtime, value goja.Value) (result any) {
	if value == nil || goja.IsUndefined(value) || goja.IsNull(value) {
		return nil
	}
	defer func() {
		if recover() != nil {
			result = nil
		}
	}()
	exported := value.Export()
	switch exported.(type) {
	case nil, bool, string, int, int64, float64, map[string]any, []any, bson.D, bson.M, bson.A:
		return normalize(exported)
	default:
		return nil
	}
}

// bsondRaw is an alias placeholder for bson.Raw so the type switch above does
// not silently miss raw command replies.
type bsondRaw = bson.Raw
