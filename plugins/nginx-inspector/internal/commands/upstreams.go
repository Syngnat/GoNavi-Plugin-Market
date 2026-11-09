package commands

import (
	"context"
	"encoding/json"
	"errors"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/Syngnat/GoNavi-Plugin-Market/plugins/nginx-inspector/internal/sshx"
)

const probeTimeout = 3 * time.Second
const probeWorkers = 8
const probeOverallTimeout = 20 * time.Second

// UpstreamCheck is the upstreams.check response. Down counts primary members
// only; backup members are reported through Backup so a healthy backup pool
// does not look like a degraded primary pool.
type UpstreamCheck struct {
	Upstreams []UpstreamCheckInfo `json:"upstreams"`
	Alive     int                 `json:"alive"`
	Down      int                 `json:"down"`
	Backup    int                 `json:"backup"`
	Skipped   int                 `json:"skipped"`
}

// UpstreamCheckInfo mirrors config.tree's upstream entries and adds the
// per-member probe verdict fields owned by this command.
type UpstreamCheckInfo struct {
	Name    string                `json:"name"`
	File    string                `json:"file,omitempty"`
	Line    int                   `json:"line"`
	Servers []UpstreamMemberCheck `json:"servers,omitempty"`
}

// UpstreamMemberCheck is one upstream member with its probe verdict.
// Disabled marks a member configured with "down" (never probed), Backup marks
// a backup member (probed but counted separately), and SkipReason explains
// why a member was not probed at all.
type UpstreamMemberCheck struct {
	Address    string   `json:"address"`
	Params     []string `json:"params,omitempty"`
	Line       int      `json:"line"`
	Alive      *bool    `json:"alive,omitempty"`
	LatencyMs  *int64   `json:"latencyMs,omitempty"`
	Backup     bool     `json:"backup,omitempty"`
	Disabled   bool     `json:"disabled,omitempty"`
	SkipReason string   `json:"skipReason,omitempty"`
}

type probeTarget struct {
	upstream int
	member   int
	host     string
	port     int
	backup   bool
}

type probeOutcome struct {
	latencyMs  int64
	skipped    bool
	skipReason string
}

func (r *Runtime) upstreamsCheck(ctx context.Context, args json.RawMessage) (any, error) {
	input, perr := decodeArgs[struct {
		ConnectionID string `json:"connectionId"`
	}](args)
	if perr != nil {
		return nil, perr
	}
	loaded, perr := r.loadTree(ctx, input.ConnectionID)
	if perr != nil {
		return nil, perr
	}
	check := &UpstreamCheck{Upstreams: make([]UpstreamCheckInfo, 0, len(loaded.tree.Upstreams))}
	for i := range loaded.tree.Upstreams {
		source := &loaded.tree.Upstreams[i]
		info := UpstreamCheckInfo{Name: source.Name, File: source.File, Line: source.Line}
		info.Servers = make([]UpstreamMemberCheck, 0, len(source.Servers))
		for j := range source.Servers {
			server := source.Servers[j]
			info.Servers = append(info.Servers, UpstreamMemberCheck{
				Address: server.Address, Params: server.Params, Line: server.Line,
				Backup:   hasParam(server.Params, "backup"),
				Disabled: hasParam(server.Params, "down"),
			})
		}
		check.Upstreams = append(check.Upstreams, info)
	}

	var targets []probeTarget
	for i := range check.Upstreams {
		for j := range check.Upstreams[i].Servers {
			member := &check.Upstreams[i].Servers[j]
			if member.Disabled {
				continue
			}
			if member.Backup {
				check.Backup++
			}
			host, port, ok := splitAddress(member.Address)
			if !ok {
				member.SkipReason = probeSkipReason(member.Address)
				check.Skipped++
				continue
			}
			targets = append(targets, probeTarget{
				upstream: i, member: j, host: host, port: port, backup: member.Backup,
			})
		}
	}
	results := make([]probeOutcome, len(targets))
	probeAll(ctx, r, input.ConnectionID, targets, results)

	for i := range targets {
		target := targets[i]
		outcome := results[i]
		member := &check.Upstreams[target.upstream].Servers[target.member]
		if outcome.skipped {
			member.SkipReason = outcome.skipReason
			check.Skipped++
			continue
		}
		alive := outcome.latencyMs >= 0
		member.Alive = &alive
		if alive {
			latency := outcome.latencyMs
			member.LatencyMs = &latency
			check.Alive++
			continue
		}
		if !target.backup {
			check.Down++
		}
	}
	return check, nil
}

// probeSkipReason explains why an address is not probeable.
func probeSkipReason(address string) string {
	trimmed := strings.TrimSpace(address)
	switch {
	case trimmed == "":
		return "empty address"
	case strings.HasPrefix(trimmed, "unix:"):
		return "unix socket addresses cannot be probed over SSH"
	case strings.Contains(trimmed, "$"):
		return "address contains a variable and cannot be resolved"
	default:
		return "address is not a host:port pair"
	}
}

func hasParam(params []string, name string) bool {
	for _, param := range params {
		if param == name {
			return true
		}
	}
	return false
}

func probeAll(ctx context.Context, r *Runtime, connectionID string, targets []probeTarget, results []probeOutcome) {
	if len(targets) == 0 {
		return
	}
	profile, perr := r.profile(connectionID)
	if perr != nil {
		for i := range results {
			results[i] = probeOutcome{skipped: true, skipReason: "connection profile is unavailable: " + perr.Message}
		}
		return
	}
	probeCtx, cancel := context.WithTimeout(ctx, probeOverallTimeout)
	defer cancel()
	sem := make(chan struct{}, probeWorkers)
	var wg sync.WaitGroup
	for i := range targets {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()
			select {
			case sem <- struct{}{}:
			case <-probeCtx.Done():
				results[index] = probeOutcome{skipped: true, skipReason: "probe deadline exceeded"}
				return
			}
			defer func() { <-sem }()
			target := targets[index]
			latency, reason, perr := r.pool.Probe(probeCtx, profile, target.host, target.port, probeTimeout)
			switch {
			case perr != nil:
				if errors.Is(probeCtx.Err(), context.DeadlineExceeded) {
					results[index] = probeOutcome{skipped: true, skipReason: "probe deadline exceeded"}
					return
				}
				results[index] = probeOutcome{skipped: true, skipReason: "SSH probe failed: " + perr.Message}
			case latency < 0:
				if reason == sshx.ProbeForwardingDisabled {
					results[index] = probeOutcome{
						skipped:    true,
						skipReason: "TCP forwarding is disabled on this SSH server (AllowTcpForwarding no)",
					}
					return
				}
				results[index] = probeOutcome{latencyMs: -1}
			default:
				results[index] = probeOutcome{latencyMs: latency}
			}
		}(i)
	}
	wg.Wait()
}

// splitAddress parses "host:port", "[v6]:port" and bare hosts (default 80).
// Unix sockets and variable addresses are not probeable.
func splitAddress(address string) (string, int, bool) {
	address = strings.TrimSpace(address)
	if address == "" || strings.HasPrefix(address, "unix:") || strings.Contains(address, "$") {
		return "", 0, false
	}
	if strings.HasPrefix(address, "[") {
		end := strings.Index(address, "]")
		if end < 0 {
			return "", 0, false
		}
		host := address[1:end]
		rest := address[end+1:]
		if !strings.HasPrefix(rest, ":") {
			return host, 80, true
		}
		port, err := strconv.Atoi(strings.TrimPrefix(rest, ":"))
		if err != nil || port < 1 || port > 65535 {
			return "", 0, false
		}
		return host, port, true
	}
	idx := strings.LastIndex(address, ":")
	if idx < 0 {
		return address, 80, true
	}
	host := address[:idx]
	port, err := strconv.Atoi(address[idx+1:])
	if err != nil {
		// "host:weight" style mistakes are not addresses.
		return "", 0, false
	}
	if host == "" {
		return "", 0, false
	}
	return host, port, true
}
