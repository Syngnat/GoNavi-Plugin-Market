package main

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
)

func TestCreateKeyPostsNameAndGroup(t *testing.T) {
	stub, baseURL := newPanelStub(t, func(request stubRequest) (int, string) {
		if request.Method != http.MethodPost || request.Path != "/api/v1/keys" {
			return http.StatusNotFound, `{"code":404}`
		}
		var body map[string]any
		if json.Unmarshal([]byte(request.Body), &body) != nil {
			t.Fatalf("unreadable body: %q", request.Body)
		}
		if body["name"] != "GoNavi" || body["group_id"] != float64(34) {
			t.Fatalf("unexpected body: %#v", body)
		}
		return http.StatusOK, `{"code":0,"data":{"id":99,"name":"GoNavi","key":"sk-createdkeyvalue","status":"active","quota":0,"quota_used":0,"created_at":"2026-01-01T00:00:00Z","group":{"name":"vip","rate_multiplier":0.1,"platform":"glm"}}}`
	})
	service := newTestService(t, stubSource{}, baseURL)
	if err := service.saveRecord(accountRecord{AccessToken: testCanaryAccess, ExpiresAt: testNowMillis + 3_600_000}); err != nil {
		t.Fatal(err)
	}
	value, err := service.createKey(context.Background(), json.RawMessage(`{"name":"GoNavi","groupId":34}`))
	if err != nil {
		t.Fatal(err)
	}
	result := outcomeOf(t, value)
	if !result.OK {
		t.Fatalf("expected a created key, got %#v", result)
	}
	raw, err := json.Marshal(result.Data)
	if err != nil {
		t.Fatal(err)
	}
	var item keyItem
	if json.Unmarshal(raw, &item) != nil || item.ID != 99 || item.Name != "GoNavi" || item.Key != "sk-createdkeyvalue" || item.GroupName != "vip" {
		t.Fatalf("unexpected key: %#v", item)
	}
	assertNoCanary(t, value)
	requests := stub.seen()
	if len(requests) != 1 || requests[0].Auth != "Bearer "+testCanaryAccess {
		t.Fatalf("unexpected request: %#v", requests)
	}
}

func TestCreateKeyRequiresAName(t *testing.T) {
	service := newTestService(t, stubSource{}, panelBaseURL)
	if err := service.saveRecord(accountRecord{AccessToken: testCanaryAccess, ExpiresAt: testNowMillis + 3_600_000}); err != nil {
		t.Fatal(err)
	}
	value, err := service.createKey(context.Background(), json.RawMessage(`{"name":"   "}`))
	if err != nil {
		t.Fatal(err)
	}
	result := outcomeOf(t, value)
	if result.OK || result.Error.Code != "invalid_name" {
		t.Fatalf("expected invalid_name, got %#v", result)
	}
}

func TestCreateKeyRequiresASession(t *testing.T) {
	service := newTestService(t, stubSource{}, panelBaseURL)
	value, err := service.createKey(context.Background(), json.RawMessage(`{"name":"GoNavi"}`))
	if err != nil {
		t.Fatal(err)
	}
	result := outcomeOf(t, value)
	if result.OK || result.Error.Code != "not_signed_in" {
		t.Fatalf("expected not_signed_in, got %#v", result)
	}
}

func TestGroupsMapsEveryField(t *testing.T) {
	stub, baseURL := newPanelStub(t, func(request stubRequest) (int, string) {
		if request.Path != "/api/v1/groups/available" {
			return http.StatusNotFound, `{"code":404}`
		}
		return http.StatusOK, `{"code":0,"data":[{"id":34,"name":"vip","rate_multiplier":0.1,"platform":"glm"}]}`
	})
	service := newTestService(t, stubSource{}, baseURL)
	if err := service.saveRecord(accountRecord{AccessToken: testCanaryAccess, ExpiresAt: testNowMillis + 3_600_000}); err != nil {
		t.Fatal(err)
	}
	value, err := service.groups(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	result := outcomeOf(t, value)
	if !result.OK {
		t.Fatalf("expected groups, got %#v", result)
	}
	raw, err := json.Marshal(result.Data)
	if err != nil {
		t.Fatal(err)
	}
	var payload groupListResult
	if json.Unmarshal(raw, &payload) != nil || payload.State != stateReady || len(payload.Items) != 1 {
		t.Fatalf("unexpected groups: %#v", payload)
	}
	if payload.Items[0].ID != 34 || payload.Items[0].Name != "vip" || payload.Items[0].RateMultiplier != 0.1 {
		t.Fatalf("unexpected group: %#v", payload.Items[0])
	}
	requests := stub.seen()
	if len(requests) != 1 || requests[0].Auth != "Bearer "+testCanaryAccess {
		t.Fatalf("unexpected request: %#v", requests)
	}
}
