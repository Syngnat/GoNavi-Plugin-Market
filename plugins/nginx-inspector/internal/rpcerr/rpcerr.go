// Package rpcerr turns handler failures into plugin.Error values whose
// application code is safe to expose and lets the UI localize messages.
package rpcerr

import (
	"encoding/json"

	plugin "github.com/Syngnat/GoNavi/sdk/go"
)

// Application codes understood by the plugin window UI.
const (
	CodeInvalidArgs      = "INVALID_ARGS"
	CodeProfileNotFound  = "PROFILE_NOT_FOUND"
	CodeStoreUnavailable = "STORE_UNAVAILABLE"
	CodeSSHConnectFailed = "SSH_CONNECT_FAILED"
	CodeSSHAuthFailed    = "SSH_AUTH_FAILED"
	CodeSSHTimeout       = "SSH_TIMEOUT"
	CodeHostKeyChanged   = "HOSTKEY_CHANGED"
	CodeNginxNotFound    = "NGINX_NOT_FOUND"
	CodeExecFailed       = "EXEC_FAILED"
	CodeFileUnreadable   = "FILE_UNREADABLE"
	CodeParseFailed      = "CONFIG_PARSE_FAILED"
	CodeTestFailed       = "NGINX_TEST_FAILED"
	CodeReloadBlocked    = "RELOAD_BLOCKED"
	CodeReloadConfirm    = "RELOAD_CONFIRMATION_REQUIRED"
	CodeCertUnreadable   = "CERT_UNREADABLE"
)

// E builds a plugin.Error carrying a stable application code and a
// human-readable message. details must contain no secrets.
func E(code, message string, details ...any) *plugin.Error {
	payload := map[string]any{"code": code}
	if len(details) == 1 {
		payload["details"] = details[0]
	} else if len(details) > 1 {
		payload["details"] = details
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return &plugin.Error{Code: -32000, Message: message}
	}
	return &plugin.Error{Code: -32000, Message: message, Data: data}
}

// FromCode maps a transport/auth failure to the right application code.
func FromCode(err error) *plugin.Error {
	msg := err.Error()
	switch {
	case containsAny(msg, "unable to authenticate", "permission denied (publickey", "no supported methods remain"):
		return E(CodeSSHAuthFailed, "SSH authentication failed: "+msg)
	case containsAny(msg, "passphrase", "decode private key", "ParsePrivateKey", "ssh: no key found", "cannot parse"):
		return E(CodeSSHAuthFailed, "SSH key could not be used: "+msg)
	case containsAny(msg, "i/o timeout", "connection timed out", "deadline exceeded", "context canceled"):
		return E(CodeSSHTimeout, "SSH operation timed out: "+msg)
	case containsAny(msg, "connection refused", "no route to host", "network is unreachable", "lookup ", "dial tcp"):
		return E(CodeSSHConnectFailed, "SSH connection failed: "+msg)
	default:
		return E(CodeSSHConnectFailed, "SSH connection failed: "+msg)
	}
}

func containsAny(s string, needles ...string) bool {
	for _, n := range needles {
		if n != "" && containsFold(s, n) {
			return true
		}
	}
	return false
}

func containsFold(s, sub string) bool {
	return len(sub) <= len(s) && indexOfFold(s, sub) >= 0
}

func indexOfFold(s, sub string) int {
	lower := func(r rune) rune {
		if r >= 'A' && r <= 'Z' {
			return r + 32
		}
		return r
	}
	ls, lsub := []rune(s), []rune(sub)
	for i := 0; i+len(lsub) <= len(ls); i++ {
		match := true
		for j := range lsub {
			if lower(ls[i+j]) != lower(lsub[j]) {
				match = false
				break
			}
		}
		if match {
			return i
		}
	}
	return -1
}
