package market

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

// HostCheckStatus reports what the optional host binary verification concluded.
type HostCheckStatus string

const (
	// HostCheckPassed means a real host binary accepted the package.
	HostCheckPassed HostCheckStatus = "passed"
	// HostCheckFailed means a real host binary refused a package the in-process
	// contract accepted. That is the drift this check exists to catch, so it is a
	// hard publishing error rather than a warning.
	HostCheckFailed HostCheckStatus = "failed"
	// HostCheckUnavailable means no host binary was configured or it could not be
	// run. Publishing continues on the in-process contract alone, but the result
	// says so instead of implying host approval that never happened.
	HostCheckUnavailable HostCheckStatus = "unavailable"
)

// HostCheck is the outcome of running `gonavi plugin check` on one package.
type HostCheck struct {
	Status  HostCheckStatus `json:"status"`
	Version string          `json:"version,omitempty"`
	Detail  string          `json:"detail,omitempty"`
}

// hostCheckTimeout bounds one host binary invocation. A package is at most
// MaxArtifactBytes, and the checker only parses it, so this is generous.
const hostCheckTimeout = 60 * time.Second

// hostCheckOutputLimit bounds how much of the host's diagnostic we keep. The
// real message is short; anything longer is noise or a hostile payload.
const hostCheckOutputLimit = 4096

// hostChecker runs `gonavi plugin check` against a package file.
//
// The in-process contract check shares its rules with the host SDK and should
// always agree, but "should" is what let three drifts through: the check below
// runs the actual released binary, so a rule the SDK has not yet caught still
// blocks the upload instead of reaching users.
type hostChecker struct {
	// binary is the absolute path to a gonavi binary. Empty disables host checking.
	binary string
	// run executes the binary; tests replace it.
	run func(ctx context.Context, binary string, args ...string) ([]byte, []byte, error)
}

func (c hostChecker) check(ctx context.Context, packagePath string) HostCheck {
	if strings.TrimSpace(c.binary) == "" {
		return HostCheck{Status: HostCheckUnavailable, Detail: "no host binary configured"}
	}
	run := c.run
	if run == nil {
		run = defaultHostRun
	}
	checkCtx, cancel := context.WithTimeout(ctx, hostCheckTimeout)
	defer cancel()
	stdout, stderr, err := run(checkCtx, c.binary, "plugin", "check", packagePath)
	if err != nil {
		if errors.Is(checkCtx.Err(), context.DeadlineExceeded) {
			return HostCheck{Status: HostCheckUnavailable, Detail: "host check timed out"}
		}
		// A non-zero exit means the host rejected the package: `gonavi plugin
		// check` writes its diagnostic to stderr and exits non-zero.
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			return HostCheck{Status: HostCheckFailed, Detail: hostDiagnostic(stderr, stdout)}
		}
		return HostCheck{Status: HostCheckUnavailable, Detail: "host binary could not be run"}
	}
	var result struct {
		Valid bool `json:"valid"`
	}
	if len(bytes.TrimSpace(stdout)) > 0 && json.Unmarshal(stdout, &result) == nil && !result.Valid {
		return HostCheck{Status: HostCheckFailed, Detail: hostDiagnostic(stderr, stdout)}
	}
	return HostCheck{Status: HostCheckPassed}
}

// hostDiagnostic prefers the structured stderr message and falls back to stdout.
func hostDiagnostic(stderr, stdout []byte) string {
	text := strings.TrimSpace(string(stderr))
	if text == "" {
		text = strings.TrimSpace(string(stdout))
	}
	if len(text) > hostCheckOutputLimit {
		text = text[:hostCheckOutputLimit]
	}
	// The diagnostic is shown to a publisher; drop any control characters so it
	// cannot rewrite terminal output.
	return strings.Map(func(r rune) rune {
		if r < 0x20 && r != '\n' && r != '\t' {
			return -1
		}
		return r
	}, text)
}

func defaultHostRun(ctx context.Context, binary string, args ...string) ([]byte, []byte, error) {
	cmd := exec.CommandContext(ctx, binary, args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	return stdout.Bytes(), stderr.Bytes(), err
}

// verifyWithHost resolves the check outcome into an error for the publish path.
func verifyWithHost(check HostCheck) error {
	if check.Status == HostCheckFailed {
		if check.Detail == "" {
			return fmt.Errorf("the host rejected this package")
		}
		return fmt.Errorf("the host rejected this package: %s", check.Detail)
	}
	return nil
}
