package market

import (
	"context"
	"errors"
	"os/exec"
	"strings"
	"testing"
)

func TestHostCheckSkipsWhenNoBinaryIsConfigured(t *testing.T) {
	check := hostChecker{}.check(context.Background(), "pkg.zip")
	if check.Status != HostCheckUnavailable {
		t.Fatalf("status = %s, want unavailable", check.Status)
	}
	if err := verifyWithHost(check); err != nil {
		t.Fatalf("an unconfigured checker must not block publishing: %v", err)
	}
}

func TestHostCheckPassesOnZeroExit(t *testing.T) {
	checker := hostChecker{binary: "/host/gonavi", run: func(_ context.Context, _ string, _ ...string) ([]byte, []byte, error) {
		return []byte(`{"valid":true,"inspection":{"digest":"abc"}}`), nil, nil
	}}
	check := checker.check(context.Background(), "pkg.zip")
	if check.Status != HostCheckPassed {
		t.Fatalf("status = %s, want passed", check.Status)
	}
	if err := verifyWithHost(check); err != nil {
		t.Fatalf("a passing host check must not block publishing: %v", err)
	}
}

// The check exists to catch drift between the shared SDK and the real binary, so
// a refusal has to stop the upload rather than warn about it.
func TestHostCheckFailsWhenTheHostRejectsThePackage(t *testing.T) {
	checker := hostChecker{binary: "/host/gonavi", run: func(_ context.Context, _ string, _ ...string) ([]byte, []byte, error) {
		return nil, []byte(`{"error":"unsupported plugin schema or api version"}`), &exec.ExitError{}
	}}
	check := checker.check(context.Background(), "pkg.zip")
	if check.Status != HostCheckFailed {
		t.Fatalf("status = %s, want failed", check.Status)
	}
	err := verifyWithHost(check)
	if err == nil {
		t.Fatal("a host rejection must block publishing")
	}
	if !strings.Contains(err.Error(), "unsupported plugin schema") {
		t.Fatalf("error must carry the host diagnostic: %v", err)
	}
}

func TestHostCheckReportsUnavailableWhenTheBinaryCannotRun(t *testing.T) {
	checker := hostChecker{binary: "/host/gonavi", run: func(_ context.Context, _ string, _ ...string) ([]byte, []byte, error) {
		return nil, nil, errors.New("exec: file does not exist")
	}}
	check := checker.check(context.Background(), "pkg.zip")
	if check.Status != HostCheckUnavailable {
		t.Fatalf("status = %s, want unavailable", check.Status)
	}
	if err := verifyWithHost(check); err != nil {
		t.Fatalf("a missing binary must not block publishing: %v", err)
	}
}

func TestHostCheckTreatsInvalidJSONVerdictAsFailure(t *testing.T) {
	checker := hostChecker{binary: "/host/gonavi", run: func(_ context.Context, _ string, _ ...string) ([]byte, []byte, error) {
		return []byte(`{"valid":false,"reason":"bad manifest"}`), nil, nil
	}}
	check := checker.check(context.Background(), "pkg.zip")
	if check.Status != HostCheckFailed {
		t.Fatalf("status = %s, want failed", check.Status)
	}
}

func TestHostDiagnosticStripsControlCharacters(t *testing.T) {
	got := hostDiagnostic([]byte("\x1b[31mred\x1b[0m\nsecond"), nil)
	if strings.ContainsRune(got, 0x1b) {
		t.Fatalf("escape characters survived: %q", got)
	}
	if !strings.Contains(got, "red") || !strings.Contains(got, "second") {
		t.Fatalf("diagnostic text lost: %q", got)
	}
}

func TestHostDiagnosticBoundsOutput(t *testing.T) {
	got := hostDiagnostic(nil, []byte(strings.Repeat("x", hostCheckOutputLimit*2)))
	if len(got) > hostCheckOutputLimit {
		t.Fatalf("diagnostic length = %d, want at most %d", len(got), hostCheckOutputLimit)
	}
}
