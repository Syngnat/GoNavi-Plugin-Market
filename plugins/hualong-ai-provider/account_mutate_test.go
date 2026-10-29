package main

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
)

func TestUpdateKeyPutsNameAndGroup(t *testing.T) {
	stub, baseURL := newPanelStub(t, func(request stubRequest) (int, string) {
		if request.Method != http.MethodPut || request.Path != "/api/v1/keys/12" {
			return http.StatusNotFound, `{"code":404}`
		}
		var body map[string]any
		if json.Unmarshal([]byte(request.Body), &body) != nil {
			t.Fatalf("unreadable body: %q", request.Body)
		}
		if body["name"] != "renamed" || body["group_id"] != float64(34) {
			t.Fatalf("unexpected body: %#v", body)
		}
		return http.StatusOK, `{"code":0,"data":{"id":12,"name":"renamed","key":"sk-keep","status":"active","group_id":34,"quota":0,"quota_used":0,"created_at":"2026-01-01T00:00:00Z","group":{"name":"vip","rate_multiplier":0.1,"platform":"glm"}}}`
	})
	service := newTestService(t, stubSource{}, baseURL)
	if err := service.saveRecord(accountRecord{AccessToken: testCanaryAccess, ExpiresAt: testNowMillis + 3_600_000}); err != nil {
		t.Fatal(err)
	}
	value, err := service.updateKey(context.Background(), json.RawMessage(`{"id":12,"name":"renamed","groupId":34}`))
	if err != nil {
		t.Fatal(err)
	}
	result := outcomeOf(t, value)
	if !result.OK {
		t.Fatalf("expected an updated key, got %#v", result)
	}
	raw, err := json.Marshal(result.Data)
	if err != nil {
		t.Fatal(err)
	}
	var item keyItem
	if json.Unmarshal(raw, &item) != nil || item.ID != 12 || item.Name != "renamed" || item.GroupID != 34 || item.GroupName != "vip" {
		t.Fatalf("unexpected key: %#v", item)
	}
	assertNoCanary(t, value)
	requests := stub.seen()
	if len(requests) != 1 || requests[0].Auth != "Bearer "+testCanaryAccess {
		t.Fatalf("unexpected request: %#v", requests)
	}
}

func TestDeleteKeyCallsPanel(t *testing.T) {
	stub, baseURL := newPanelStub(t, func(request stubRequest) (int, string) {
		if request.Method != http.MethodDelete || request.Path != "/api/v1/keys/12" {
			return http.StatusNotFound, `{"code":404}`
		}
		return http.StatusOK, `{"code":0,"data":null}`
	})
	service := newTestService(t, stubSource{}, baseURL)
	if err := service.saveRecord(accountRecord{AccessToken: testCanaryAccess, ExpiresAt: testNowMillis + 3_600_000}); err != nil {
		t.Fatal(err)
	}
	value, err := service.deleteKey(context.Background(), json.RawMessage(`{"id":12}`))
	if err != nil {
		t.Fatal(err)
	}
	result := outcomeOf(t, value)
	if !result.OK {
		t.Fatalf("expected delete to succeed, got %#v", result)
	}
	assertNoCanary(t, value)
	requests := stub.seen()
	if len(requests) != 1 || requests[0].Method != http.MethodDelete {
		t.Fatalf("unexpected request: %#v", requests)
	}
}

func TestUpdateKeyRequiresAnID(t *testing.T) {
	service := newTestService(t, stubSource{}, panelBaseURL)
	value, err := service.updateKey(context.Background(), json.RawMessage(`{"name":"renamed"}`))
	if err != nil {
		t.Fatal(err)
	}
	result := outcomeOf(t, value)
	if result.OK || result.Error.Code != "invalid_id" {
		t.Fatalf("expected invalid_id, got %#v", result)
	}
}
