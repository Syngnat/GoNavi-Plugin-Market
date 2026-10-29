package main

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"testing"
)

// TestListModelsMergesEveryKey pins the scan contract: one gateway call per key,
// because each group exposes a different model set. Gateway metadata (name,
// context window) is merged too, so the host can show readable model labels even
// when the public catalog has never heard of the id.
func TestListModelsMergesEveryKey(t *testing.T) {
	_, baseURL := newPanelStub(t, func(request stubRequest) (int, string) {
		switch {
		case request.Path == "/api/v1/keys":
			return http.StatusOK, `{"code":0,"data":{"items":[
				{"id":1,"name":"a","key":"sk-first","status":"active"},
				{"id":2,"name":"b","key":"sk-second","status":"active"}],"total":2,"page":1,"pages":1}}`
		case request.Method == http.MethodGet && request.Path == "/v1/models":
			if strings.HasSuffix(request.Auth, "sk-first") {
				return http.StatusOK, `{"data":[
					{"id":"glm-5.3","name":"GLM 5.3","context_length":1000000},
					{"id":"kimi-k3","context_window":262144}]}`
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
	byID := make(map[string]modelItem, len(scanned.Items))
	for _, item := range scanned.Items {
		byID[item.ID] = item
	}
	// 第二把 Key 只回 id：不能把第一把 Key 给的名称与窗口清掉。
	if glm := byID["glm-5.3"]; glm.Name != "GLM 5.3" || glm.ContextWindow != 1000000 {
		t.Fatalf("glm-5.3 metadata lost: %#v", glm)
	}
	if kimi := byID["kimi-k3"]; kimi.Name != "" || kimi.ContextWindow != 262144 {
		t.Fatalf("kimi-k3 metadata wrong: %#v", kimi)
	}
	// 网关没给元数据的模型留空，由宿主用 models.dev 公共目录补齐。
	if gpt := byID["gpt-6"]; gpt.Name != "" || gpt.ContextWindow != 0 {
		t.Fatalf("gpt-6 should carry no metadata: %#v", gpt)
	}
	assertNoCanary(t, value)
}

// TestListModelsDropsOutOfRangeMetadata keeps a hostile gateway from pushing
// absurd names or windows into the host UI.
func TestListModelsDropsOutOfRangeMetadata(t *testing.T) {
	longName := strings.Repeat("n", maxModelNameBytes+1)
	_, baseURL := newPanelStub(t, func(request stubRequest) (int, string) {
		switch {
		case request.Path == "/api/v1/keys":
			return http.StatusOK, `{"code":0,"data":{"items":[
				{"id":1,"name":"a","key":"sk-only","status":"active"}],"total":1,"page":1,"pages":1}}`
		case request.Path == "/v1/models":
			return http.StatusOK, `{"data":[{"id":"glm-5.3","name":` + strconv.Quote(longName) + `,"context_length":-1},
				{"id":"kimi-k3","display_name":"  Kimi K3  ","max_context_length":` + strconv.Itoa(maxModelContextWindow+1) + `}]}`
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
	raw, _ := json.Marshal(outcomeOf(t, value).Data)
	var scanned modelListResult
	if json.Unmarshal(raw, &scanned) != nil {
		t.Fatalf("unreadable scan: %s", raw)
	}
	if len(scanned.Items) != 2 {
		t.Fatalf("unexpected scan: %s", raw)
	}
	byID := make(map[string]modelItem, len(scanned.Items))
	for _, item := range scanned.Items {
		byID[item.ID] = item
	}
	if glm := byID["glm-5.3"]; glm.Name != "" || glm.ContextWindow != 0 {
		t.Fatalf("out-of-range metadata should be dropped: %#v", glm)
	}
	if kimi := byID["kimi-k3"]; kimi.Name != "Kimi K3" || kimi.ContextWindow != 0 {
		t.Fatalf("display name should be trimmed and oversized window dropped: %#v", kimi)
	}
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
