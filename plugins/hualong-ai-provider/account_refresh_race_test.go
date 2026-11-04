package main

import (
	"context"
	"net/http"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// The provider window asks for status and keys together. With an expired access
// token both commands used to spend the same refresh token; the loser cleared
// the record the winner had just written and the window reported "not signed in".
func TestConcurrentStatusRefreshKeepsTheRotatedSession(t *testing.T) {
	const rotated = "rotated-access-token"
	var refreshes atomic.Int32
	stub, baseURL := newPanelStub(t, func(request stubRequest) (int, string) {
		switch request.Path {
		case "/api/v1/auth/refresh":
			if refreshes.Add(1) > 1 {
				return http.StatusUnauthorized, `{"code":401}`
			}
			// Keep the winner inside refresh long enough for the loser to queue on
			// the rotation lock rather than race past it.
			time.Sleep(50 * time.Millisecond)
			return http.StatusOK, `{"code":0,"data":{"access_token":"` + rotated + `","refresh_token":"rotated-refresh","expires_in":3600}}`
		case "/api/v1/auth/me":
			if request.Auth != "Bearer "+rotated {
				return http.StatusUnauthorized, `{"code":401}`
			}
			return http.StatusOK, mePayload
		}
		return http.StatusNotFound, `{"code":404}`
	})
	service := newTestService(t, stubSource{}, baseURL)
	if err := service.saveRecord(accountRecord{AccessToken: testCanaryAccess, RefreshToken: testCanaryRefresh, ExpiresAt: testNowMillis - 1000}); err != nil {
		t.Fatal(err)
	}

	values := make([]any, 2)
	errs := make([]error, 2)
	var wg sync.WaitGroup
	for index := range values {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()
			values[index], errs[index] = service.status(context.Background(), nil)
		}(index)
	}
	wg.Wait()

	for index := range values {
		if errs[index] != nil {
			t.Fatalf("status %d failed: %v", index, errs[index])
		}
		result := outcomeOf(t, values[index])
		if !result.OK || dataOf(t, result)["state"] != stateReady {
			t.Fatalf("status %d did not stay signed in: %#v", index, result)
		}
		assertNoCanary(t, values[index])
	}
	if got := stub.count("/api/v1/auth/refresh"); got != 1 {
		t.Fatalf("expected exactly one rotation, got %d", got)
	}
	record, ok := service.loadRecord()
	if !ok || record.AccessToken != rotated || record.RefreshToken != "rotated-refresh" {
		t.Fatalf("the winner's session was not kept: %#v", record)
	}
}

// status and keys are independent commands, so the window can issue both before
// either finishes. They must share one rotation and both stay signed in.
func TestConcurrentStatusAndKeysShareOneRotation(t *testing.T) {
	const rotated = "rotated-access-token"
	var refreshes atomic.Int32
	stub, baseURL := newPanelStub(t, func(request stubRequest) (int, string) {
		switch request.Path {
		case "/api/v1/auth/refresh":
			if refreshes.Add(1) > 1 {
				return http.StatusUnauthorized, `{"code":401}`
			}
			time.Sleep(50 * time.Millisecond)
			return http.StatusOK, `{"code":0,"data":{"access_token":"` + rotated + `","refresh_token":"rotated-refresh","expires_in":3600}}`
		case "/api/v1/auth/me":
			return http.StatusOK, mePayload
		case "/api/v1/keys":
			if request.Auth != "Bearer "+rotated {
				return http.StatusUnauthorized, `{"code":401}`
			}
			return http.StatusOK, `{"code":0,"data":{"items":[],"total":0,"page":1,"pages":0}}`
		}
		return http.StatusNotFound, `{"code":404}`
	})
	service := newTestService(t, stubSource{}, baseURL)
	if err := service.saveRecord(accountRecord{AccessToken: testCanaryAccess, RefreshToken: testCanaryRefresh, ExpiresAt: testNowMillis - 1000}); err != nil {
		t.Fatal(err)
	}

	values := make([]any, 2)
	errs := make([]error, 2)
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		values[0], errs[0] = service.status(context.Background(), nil)
	}()
	go func() {
		defer wg.Done()
		values[1], errs[1] = service.keys(context.Background(), nil)
	}()
	wg.Wait()

	for index := range values {
		if errs[index] != nil {
			t.Fatalf("command %d failed: %v", index, errs[index])
		}
		result := outcomeOf(t, values[index])
		if !result.OK {
			t.Fatalf("command %d was refused: %#v", index, result)
		}
		assertNoCanary(t, values[index])
	}
	if got := stub.count("/api/v1/auth/refresh"); got != 1 {
		t.Fatalf("expected exactly one rotation, got %d", got)
	}
	if _, ok := service.loadRecord(); !ok {
		t.Fatal("a concurrent status and keys signed the user out")
	}
}

// A 401 earned with the previous token must not remove the record another
// command has already rotated in the meantime.
func TestStaleUnauthorizedDoesNotClearARotatedRecord(t *testing.T) {
	service := newTestService(t, stubSource{}, panelBaseURL)
	rotated := accountRecord{AccessToken: "rotated-access-token", RefreshToken: "rotated-refresh", ExpiresAt: testNowMillis + 3_600_000}
	if err := service.saveRecord(rotated); err != nil {
		t.Fatal(err)
	}

	service.clearRecordIf(testCanaryAccess)
	record, ok := service.loadRecord()
	if !ok || record.AccessToken != rotated.AccessToken {
		t.Fatal("a stale 401 removed the rotated session")
	}

	service.clearRecordIf(rotated.AccessToken)
	if _, ok := service.loadRecord(); ok {
		t.Fatal("a rejection for the stored token must still clear it")
	}
}
