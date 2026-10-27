package shell

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/dop251/goja"
)

const (
	defaultMaxRows         = 200
	maxRowsLimit           = 2000
	maxCollectionNames     = 1000
	maxOutputLines         = 500
	maxOutputLineRunes     = 4000
	maxDatabaseNameRunes   = 255
	maxCollectionNameRunes = 255
)

type runArgs struct {
	Script     string `json:"script"`
	Database   string `json:"database,omitempty"`
	Collection string `json:"collection,omitempty"`
	MaxRows    int    `json:"maxRows,omitempty"`
}

type runData struct {
	Database  string   `json:"database"`
	ReadOnly  bool     `json:"readOnly"`
	Output    []string `json:"output"`
	Result    any      `json:"result,omitempty"`
	Table     *table   `json:"table,omitempty"`
	ElapsedMs int64    `json:"elapsedMs"`
	MaxRows   int      `json:"maxRows"`
	TimeoutMs int64    `json:"timeoutMs"`
}

type runState struct {
	vm              *goja.Runtime
	ctx             context.Context
	client          Client
	dbName          string
	readOnly        bool
	maxRows         int
	output          []string
	outputTruncated bool
	table           *table
	db              goja.Value
}

func newRunState(ctx context.Context, client Client, database string, readOnly bool, maxRows int, timeout time.Duration) (*runState, error) {
	state := &runState{
		vm:       goja.New(),
		ctx:      ctx,
		client:   client,
		dbName:   database,
		readOnly: readOnly,
		maxRows:  maxRows,
		output:   make([]string, 0, 8),
	}
	if err := state.installGlobals(); err != nil {
		return nil, err
	}
	return state, nil
}

func (s *runState) installGlobals() error {
	s.db = s.newDatabaseObject()
	if err := s.vm.Set("print", s.print); err != nil {
		return err
	}
	if err := s.vm.Set("use", s.use); err != nil {
		return err
	}
	return s.vm.Set("db", s.db)
}

func (s *runState) database() Database {
	return s.client.Database(s.dbName)
}

func (s *runState) print(call goja.FunctionCall) goja.Value {
	parts := make([]string, 0, len(call.Arguments))
	for _, argument := range call.Arguments {
		parts = append(parts, s.stringify(argument))
	}
	s.appendOutput(strings.Join(parts, " "))
	return goja.Undefined()
}

// stringify renders a print argument like the Mongo shell: strings verbatim,
// objects as compact JSON.
func (s *runState) stringify(value goja.Value) string {
	if value == nil || goja.IsUndefined(value) || goja.IsNull(value) {
		return "null"
	}
	exported := value.Export()
	switch typed := exported.(type) {
	case string:
		return typed
	case bool:
		return fmt.Sprint(typed)
	case int64, float64:
		return value.String()
	}
	if encoded, err := json.Marshal(normalize(exported)); err == nil {
		return string(encoded)
	}
	return value.String()
}

func (s *runState) appendOutput(line string) {
	if s.outputTruncated {
		return
	}
	if len(s.output) >= maxOutputLines {
		s.outputTruncated = true
		s.output = append(s.output, fmt.Sprintf("[output truncated after %d lines]", maxOutputLines))
		return
	}
	s.output = append(s.output, truncateRunes(line, maxOutputLineRunes))
}

func (s *runState) use(call goja.FunctionCall) goja.Value {
	name := strings.TrimSpace(call.Argument(0).String())
	if name == "" || utf8.RuneCountInString(name) > maxDatabaseNameRunes {
		s.throw(codeInvalidArgument, "use() requires a database name")
	}
	s.dbName = name
	s.appendOutput("switched to db " + name)
	return s.db
}

func (s *runState) newDatabaseObject() goja.Value {
	target := s.vm.NewObject()
	proxy := s.vm.NewProxy(target, &goja.ProxyTrapConfig{
		Get: func(_ *goja.Object, property string, _ goja.Value) goja.Value {
			if method := s.databaseMethod(property); method != nil {
				return s.vm.ToValue(method)
			}
			if reservedJSNames[property] {
				return nil
			}
			return s.collectionValue(property)
		},
		Has: func(_ *goja.Object, property string) bool {
			return s.databaseMethod(property) != nil || !reservedJSNames[property]
		},
	})
	return s.vm.ToValue(proxy)
}

func (s *runState) databaseMethod(name string) func(goja.FunctionCall) goja.Value {
	switch name {
	case "getName":
		return func(goja.FunctionCall) goja.Value { return s.vm.ToValue(s.dbName) }
	case "getCollectionNames":
		return s.getCollectionNames
	case "getCollection":
		return s.getCollection
	case "runCommand":
		return s.runCommand
	default:
		return nil
	}
}

var reservedJSNames = map[string]bool{
	"then": true, "toJSON": true, "toString": true, "valueOf": true,
	"constructor": true, "inspect": true, "__proto__": true, "prototype": true,
	"hasOwnProperty": true, "isPrototypeOf": true, "propertyIsEnumerable": true,
}

func (s *runState) getCollectionNames(goja.FunctionCall) goja.Value {
	names, err := s.database().CollectionNames(s.ctx)
	if err != nil {
		s.throw(codeQueryFailed, "cannot list collections: "+err.Error())
	}
	truncated := len(names) > maxCollectionNames
	if truncated {
		names = names[:maxCollectionNames]
	}
	s.table = tableFromNames("collections", names, truncated)
	values := make([]any, len(names))
	for index, name := range names {
		values[index] = name
	}
	return s.vm.ToValue(values)
}

func (s *runState) getCollection(call goja.FunctionCall) goja.Value {
	name := strings.TrimSpace(call.Argument(0).String())
	if err := validateCollectionName(name); err != nil {
		s.throwError(err)
	}
	return s.collectionValue(name)
}

func (s *runState) collectionValue(name string) goja.Value {
	object := s.vm.NewObject()
	_ = object.Set("name", name)
	_ = object.Set("find", func(call goja.FunctionCall) goja.Value { return s.find(name, call) })
	_ = object.Set("findOne", func(call goja.FunctionCall) goja.Value { return s.findOne(name, call) })
	_ = object.Set("countDocuments", func(call goja.FunctionCall) goja.Value { return s.countDocuments(name, call) })
	_ = object.Set("aggregate", func(call goja.FunctionCall) goja.Value { return s.aggregate(name, call) })
	_ = object.Set("insertOne", func(call goja.FunctionCall) goja.Value { return s.insertOne(name, call) })
	_ = object.Set("updateOne", func(call goja.FunctionCall) goja.Value { return s.updateOne(name, call) })
	_ = object.Set("deleteOne", func(call goja.FunctionCall) goja.Value { return s.deleteOne(name, call) })
	_ = object.Set("runCommand", func(call goja.FunctionCall) goja.Value { return s.runCommand(call) })
	return object
}

// RunScript executes one script against the already connected client.
func (s *Shell) RunScript(ctx context.Context, args runArgs) (runData, error) {
	s.opMu.RLock()
	defer s.opMu.RUnlock()
	if s.client == nil {
		return runData{}, newCodedError(codeNotConnected, "not connected: call connect first")
	}
	client := s.client
	session := s.session
	database := strings.TrimSpace(args.Database)
	if database == "" {
		database = session.database
	}
	maxRows := clampMaxRows(args.MaxRows)
	timeout := s.commandTimeout
	if timeout <= 0 {
		timeout = defaultCommandTimeout
	}
	runCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	started := time.Now()
	state, err := newRunState(runCtx, client, database, session.readOnly, maxRows, timeout)
	if err != nil {
		return runData{}, wrapCodedError(codeInternal, "cannot start the JavaScript runtime", err)
	}
	interrupt := context.AfterFunc(runCtx, func() {
		state.vm.Interrupt("script execution cancelled")
	})
	defer interrupt()
	value, runErr := state.vm.RunString(args.Script)
	state.vm.ClearInterrupt()
	if runErr != nil {
		return runData{}, state.classifyError(runErr)
	}
	return runData{
		Database:  state.dbName,
		ReadOnly:  state.readOnly,
		Output:    state.output,
		Result:    jsResult(state.vm, value),
		Table:     state.table,
		ElapsedMs: time.Since(started).Milliseconds(),
		MaxRows:   maxRows,
		TimeoutMs: timeout.Milliseconds(),
	}, nil
}

func (s *runState) classifyError(err error) error {
	var interrupted *goja.InterruptedError
	if errors.As(err, &interrupted) {
		if s.ctx.Err() != nil {
			return wrapCodedError(codeTimeout, "the script exceeded the command timeout", s.ctx.Err())
		}
		return newCodedError(codeTimeout, "the script was interrupted after exceeding the command timeout")
	}
	var exception *goja.Exception
	if errors.As(err, &exception) {
		return codedFromException(exception)
	}
	var syntax *goja.CompilerSyntaxError
	if errors.As(err, &syntax) {
		return newCodedError(codeSyntaxError, syntax.Error())
	}
	if s.ctx.Err() != nil {
		return wrapCodedError(codeTimeout, "the script exceeded the command timeout", s.ctx.Err())
	}
	return newCodedError(codeScriptError, err.Error())
}

func codedFromException(exception *goja.Exception) error {
	value := exception.Value()
	if object, ok := value.(*goja.Object); ok {
		code := stringProperty(object, "code")
		message := stringProperty(object, "message")
		if message == "" {
			message = value.String()
		}
		if code == "" {
			code = codeScriptError
			if stringProperty(object, "name") == "SyntaxError" {
				code = codeSyntaxError
			}
		}
		return newCodedError(code, redactSecrets(message))
	}
	return newCodedError(codeScriptError, redactSecrets(exception.String()))
}

func stringProperty(object *goja.Object, key string) string {
	property := object.Get(key)
	if property == nil || goja.IsUndefined(property) || goja.IsNull(property) {
		return ""
	}
	return property.String()
}

func (s *runState) throw(code, message string) {
	object := s.vm.NewObject()
	_ = object.Set("name", "MongoShellError")
	_ = object.Set("code", code)
	_ = object.Set("message", redactSecrets(message))
	panic(object)
}

func (s *runState) throwError(err error) {
	s.throw(errorCode(err), errorMessage(err))
}

// fail is used by read functions: it throws and never returns.
func (s *runState) fail(err error) goja.Value {
	s.throwError(err)
	return goja.Undefined()
}

func clampMaxRows(requested int) int {
	if requested <= 0 {
		return defaultMaxRows
	}
	if requested > maxRowsLimit {
		return maxRowsLimit
	}
	return requested
}

func resolveLimit(requested int, maxRows int) int64 {
	limit := requested
	if limit <= 0 {
		limit = maxRows
	}
	if limit > maxRows {
		limit = maxRows
	}
	return int64(limit)
}

func validateCollectionName(name string) error {
	if name == "" || utf8.RuneCountInString(name) > maxCollectionNameRunes {
		return newCodedError(codeInvalidArgument, "a collection name is required")
	}
	return nil
}

func truncateRunes(text string, limit int) string {
	if utf8.RuneCountInString(text) <= limit {
		return text
	}
	runes := []rune(text)
	return string(runes[:limit]) + "…"
}

func tableFromNames(label string, names []string, truncated bool) *table {
	rows := make([][]any, len(names))
	for index, name := range names {
		rows[index] = []any{name}
	}
	return &table{Label: label, Columns: []string{"name"}, Rows: rows, Truncated: truncated}
}
