package shell

import (
	"strings"

	"github.com/dop251/goja"
	"go.mongodb.org/mongo-driver/v2/bson"
)

func (s *runState) requireWritable() {
	if s.readOnly {
		s.throw(codeReadOnly, "the connection is read-only; writes and mutating commands are disabled")
	}
}

func (s *runState) optionalDocument(value goja.Value) any {
	if value == nil || goja.IsUndefined(value) || goja.IsNull(value) {
		return nil
	}
	return s.objectArgument(value)
}

func (s *runState) objectArgument(value goja.Value) map[string]any {
	exported := value.Export()
	object, ok := exported.(map[string]any)
	if !ok {
		s.throw(codeInvalidArgument, "expected an object argument")
	}
	return object
}

func (s *runState) requiredDocument(value goja.Value) map[string]any {
	if value == nil || goja.IsUndefined(value) || goja.IsNull(value) {
		s.throw(codeInvalidArgument, "an object argument is required")
	}
	return s.objectArgument(value)
}

func (s *runState) pipelineArgument(value goja.Value) []any {
	if value == nil || goja.IsUndefined(value) || goja.IsNull(value) {
		s.throw(codeInvalidArgument, "aggregate() requires a pipeline array")
	}
	exported := value.Export()
	stages, ok := exported.([]any)
	if !ok {
		s.throw(codeInvalidArgument, "aggregate() requires a pipeline array")
	}
	for _, stage := range stages {
		if _, valid := stage.(map[string]any); !valid {
			s.throw(codeInvalidArgument, "every aggregate stage must be an object")
		}
	}
	return stages
}

func (s *runState) booleanOption(value goja.Value, key string) bool {
	if value == nil || goja.IsUndefined(value) || goja.IsNull(value) {
		return false
	}
	object := s.objectArgument(value)
	flag, _ := object[key].(bool)
	return flag
}

func (s *runState) intArgument(value goja.Value) int {
	if value == nil || goja.IsUndefined(value) || goja.IsNull(value) {
		return 0
	}
	number := value.ToInteger()
	if number < 0 {
		return 0
	}
	if number > maxRowsLimit {
		return maxRowsLimit
	}
	return int(number)
}

func (s *runState) find(collectionName string, call goja.FunctionCall) goja.Value {
	filter := s.optionalDocument(call.Argument(0))
	projection := s.optionalDocument(call.Argument(1))
	limit := resolveLimit(s.intArgument(call.Argument(2)), s.maxRows)
	documents, truncated, err := s.database().Collection(collectionName).Find(s.ctx, filter, projection, limit)
	if err != nil {
		s.opError("find", err)
	}
	s.table = tableFromDocuments(collectionName+".find", documents, truncated)
	return s.vm.ToValue(normalizeDocuments(documents))
}

func (s *runState) findOne(collectionName string, call goja.FunctionCall) goja.Value {
	filter := s.optionalDocument(call.Argument(0))
	projection := s.optionalDocument(call.Argument(1))
	document, found, err := s.database().Collection(collectionName).FindOne(s.ctx, filter, projection)
	if err != nil {
		s.opError("findOne", err)
	}
	if !found {
		s.table = &table{Label: collectionName + ".findOne", Columns: []string{}, Rows: [][]any{}}
		return goja.Null()
	}
	s.table = tableFromDocuments(collectionName+".findOne", []bson.D{document}, false)
	return s.vm.ToValue(normalizeDocument(document))
}

func (s *runState) countDocuments(collectionName string, call goja.FunctionCall) goja.Value {
	filter := s.optionalDocument(call.Argument(0))
	count, err := s.database().Collection(collectionName).CountDocuments(s.ctx, filter)
	if err != nil {
		s.opError("countDocuments", err)
	}
	s.table = tableFromRow(collectionName+".countDocuments", []string{"count"}, []any{count})
	return s.vm.ToValue(count)
}

func (s *runState) aggregate(collectionName string, call goja.FunctionCall) goja.Value {
	pipeline := s.pipelineArgument(call.Argument(0))
	if pipelineWrites(pipeline) {
		s.requireWritable()
	}
	documents, truncated, err := s.database().Collection(collectionName).Aggregate(s.ctx, pipeline, int64(s.maxRows))
	if err != nil {
		s.opError("aggregate", err)
	}
	s.table = tableFromDocuments(collectionName+".aggregate", documents, truncated)
	return s.vm.ToValue(normalizeDocuments(documents))
}

func (s *runState) insertOne(collectionName string, call goja.FunctionCall) goja.Value {
	s.requireWritable()
	document := s.requiredDocument(call.Argument(0))
	insertedID, err := s.database().Collection(collectionName).InsertOne(s.ctx, document)
	if err != nil {
		s.opError("insertOne", err)
	}
	normalizedID := normalize(insertedID)
	s.table = tableFromRow(collectionName+".insertOne", []string{"acknowledged", "insertedId"}, []any{true, normalizedID})
	return s.vm.ToValue(map[string]any{"acknowledged": true, "insertedId": normalizedID})
}

func (s *runState) updateOne(collectionName string, call goja.FunctionCall) goja.Value {
	s.requireWritable()
	filter := s.optionalDocument(call.Argument(0))
	update := s.requiredDocument(call.Argument(1))
	upsert := s.booleanOption(call.Argument(2), "upsert")
	matched, modified, err := s.database().Collection(collectionName).UpdateOne(s.ctx, filter, update, upsert)
	if err != nil {
		s.opError("updateOne", err)
	}
	s.table = tableFromRow(collectionName+".updateOne", []string{"acknowledged", "matchedCount", "modifiedCount"}, []any{true, matched, modified})
	return s.vm.ToValue(map[string]any{"acknowledged": true, "matchedCount": matched, "modifiedCount": modified})
}

func (s *runState) deleteOne(collectionName string, call goja.FunctionCall) goja.Value {
	s.requireWritable()
	filter := s.optionalDocument(call.Argument(0))
	deleted, err := s.database().Collection(collectionName).DeleteOne(s.ctx, filter)
	if err != nil {
		s.opError("deleteOne", err)
	}
	s.table = tableFromRow(collectionName+".deleteOne", []string{"acknowledged", "deletedCount"}, []any{true, deleted})
	return s.vm.ToValue(map[string]any{"acknowledged": true, "deletedCount": deleted})
}

func (s *runState) runCommand(call goja.FunctionCall) goja.Value {
	command := s.requiredDocument(call.Argument(0))
	if s.readOnly && commandMutates(command) {
		s.throw(codeReadOnly, "the connection is read-only; mutating commands are disabled")
	}
	result, err := s.database().RunCommand(s.ctx, command)
	if err != nil {
		s.opError("runCommand", err)
	}
	s.table = commandTable("runCommand", result)
	return s.vm.ToValue(normalizeDocument(result))
}

func pipelineWrites(pipeline []any) bool {
	for _, stage := range pipeline {
		if stageWrites(stage) {
			return true
		}
	}
	return false
}

func stageWrites(stage any) bool {
	object, ok := stage.(map[string]any)
	if !ok {
		return false
	}
	if _, exists := object["$out"]; exists {
		return true
	}
	_, exists := object["$merge"]
	return exists
}

// commandMutates covers the server commands that change data, indexes,
// namespaces, users or server configuration. Read-only sessions refuse them
// before the driver sends anything.
var mutatingCommands = map[string]bool{
	"insert": true, "update": true, "delete": true, "findandmodify": true,
	"bulkwrite": true, "create": true, "drop": true, "dropdatabase": true,
	"createindexes": true, "dropindexes": true, "renamecollection": true,
	"collmod": true, "converttocapped": true, "emptycapped": true,
	"mapreduce": true, "applyops": true, "replsetinitiate": true,
	"replsetreconfig": true, "shutdown": true, "setparameter": true,
	"createuser": true, "updateuser": true, "dropuser": true,
	"grantrolestouser": true, "revokerolesfromuser": true,
	"createrole": true, "updaterole": true, "droprole": true,
	"grantprivilegestorole": true, "revokeprivilegesfromrole": true,
	"committransaction": true, "aborttransaction": true, "eval": true,
}

func commandMutates(command map[string]any) bool {
	for key := range command {
		name := strings.ToLower(strings.TrimSpace(key))
		if mutatingCommands[name] {
			return true
		}
	}
	// An aggregate command names the collection in its command slot, so the
	// pipeline lives under its own key.
	if pipeline, ok := command["pipeline"].([]any); ok && pipelineWrites(pipeline) {
		return true
	}
	return false
}

// opError separates a driver failure caused by the command deadline from an
// ordinary server error, so the window can present a timeout as such.
func (s *runState) opError(operation string, err error) {
	if s.ctx.Err() != nil {
		s.throw(codeTimeout, "the command exceeded the "+operation+" timeout")
	}
	s.throw(codeQueryFailed, operation+" failed: "+err.Error())
}
