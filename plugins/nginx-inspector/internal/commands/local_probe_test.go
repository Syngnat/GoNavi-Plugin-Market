package commands

import (
	"context"
	"encoding/json"
	"testing"

	plugin "github.com/Syngnat/GoNavi/sdk/go"
)

func TestLocalHandlerProbe(t *testing.T) {
	runtime := NewRuntime()
	handlers := runtime.Handlers()
	result, err := handlers["connections.list"](context.Background(), nil)
	if err != nil {
		var perr *plugin.Error
		if asPluginError(err, &perr) {
			t.Fatalf("connections.list failed: code=%d message=%q data=%s", perr.Code, perr.Message, string(perr.Data))
		}
		t.Fatalf("connections.list failed: %v", err)
	}
	encoded, _ := json.Marshal(result)
	t.Logf("connections.list ok: %s", string(encoded))
}

func asPluginError(err error, target **plugin.Error) bool {
	p, ok := err.(*plugin.Error)
	if ok {
		*target = p
	}
	return ok
}
