package main

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
)

// TestListModelsMergesEveryKey pins the scan contract: one gateway call per key,
// because each group exposes a different model set.
func TestListModelsMergesEveryKey(t *testing.T) {
	_, baseURL := newPanelStub(t, func(request stubRequest) (int, string) {
		switch {
		case request.Path == "/api/v1/keys":
			return http.StatusOK, `{"code":0,"data":{"items":[
				{"id":1,"name":"a","key":"sk-first","status":"active"},
				{"id":2,"name":"b","key":"sk-second","status":"active"}],"total":2,"page":1,"pages":1}}`
		case request.Method == http.MethodGet && request.Path == "/v1/models":
			if strings.HasSuffix(request.Auth, "sk-first") {
				return http.StatusOK, `{"data":[{"id":"glm-5.3"},{"id":"kimi-k3"}]}`
			}
			return http.StatusOK, `{"data":[{"id":"glm-5.3"},{"id":"gpt-6"}]}`
		}
		return http.StatusNotFound, `{"code":404}`
	})
	service := newTestService(t, stubSource{}, baseURL)
	if err := service.saveRecord(accountRecord{AccessToken: testCanaryAccess, ExpiresAt: testNowMillis + 3_600_000}); err != nil {
		t.Fatal(err)
	}
	value, err := service.listModels(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	result := outcomeOf(t, value)
	if !result.OK {
		t.Fatalf("expected a model scan, got %#v", result)
	}
	raw, marshalErr := json.Marshal(result.Data)
	if marshalErr != nil {
		t.Fatal(marshalErr)
	}
	var scanned modelListResult
	if json.Unmarshal(raw, &scanned) != nil {
		t.Fatalf("unreadable scan: %s", raw)
	}
	ids := make([]string, 0, len(scanned.Items))
	for _, item := range scanned.Items {
		ids = append(ids, item.ID)
		if len(item.KeyIDs) == 0 {
			t.Fatalf("model %q lost its key provenance: %#v", item.ID, item)
		}
	}
	want := []string{"glm-5.3", "gpt-6", "kimi-k3"}
	if strings.Join(ids, ",") != strings.Join(want, ",") {
		t.Fatalf("unexpected models: %v", ids)
	}
	assertNoCanary(t, value)
}

// TestListModelsToleratesARejectedKey keeps a partial scan usable.
func TestListModelsToleratesARejectedKey(t *testing.T) {
	_, baseURL := newPanelStub(t, func(request stubRequest) (int, string) {
		switch {
		case request.Path == "/api/v1/keys":
			return http.StatusOK, `{"code":0,"data":{"items":[
				{"id":1,"name":"a","key":"sk-good","status":"active"},
				{"id":2,"name":"b","key":"sk-bad","status":"active"}],"total":2,"page":1,"pages":1}}`
		case request.Path == "/v1/models":
			if strings.HasSuffix(request.Auth, "sk-bad") {
				return http.StatusUnauthorized, `{"error":"nope"}`
			}
			return http.StatusOK, `{"data":[{"id":"glm-5.3"}]}`
		}
		return http.StatusNotFound, `{"code":404}`
	})
	service := newTestService(t, stubSource{}, baseURL)
	if err := service.saveRecord(accountRecord{AccessToken: testCanaryAccess, ExpiresAt: testNowMillis + 3_600_000}); err != nil {
		t.Fatal(err)
	}
	value, err := service.listModels(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	result := outcomeOf(t, value)
	if !result.OK {
		t.Fatalf("expected a partial scan, got %#v", result)
	}
	raw, _ := json.Marshal(result.Data)
	var scanned modelListResult
	if json.Unmarshal(raw, &scanned) != nil || scanned.FailedKeys != 1 || len(scanned.Items) != 1 {
		t.Fatalf("unexpected scan: %s", raw)
	}
}
