package commands

import (
	"context"
	"encoding/json"
	"strconv"
	"strings"
	"sync"
	"time"
)

const probeTimeout = 3 * time.Second
const probeWorkers = 8

// UpstreamCheck is the upstreams.check response.
type UpstreamCheck struct {
	Upstreams []UpstreamInfo `json:"upstreams"`
	Alive     int            `json:"alive"`
	Down      int            `json:"down"`
	Skipped   int            `json:"skipped"`
}

type probeTarget struct {
	upstream int
	member   int
	host     string
	port     int
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
	var targets []probeTarget
	for i := range loaded.tree.Upstreams {
		for j := range loaded.tree.Upstreams[i].Servers {
			host, port, ok := splitAddress(loaded.tree.Upstreams[i].Servers[j].Address)
			if !ok {
				loaded.tree.Upstreams[i].Servers[j].Alive = nil
				continue
			}
			targets = append(targets, probeTarget{upstream: i, member: j, host: host, port: port})
		}
	}
	results := make([]probeOutcome, len(targets))
	probeAll(ctx, r, input.ConnectionID, targets, results)

	check := &UpstreamCheck{Upstreams: loaded.tree.Upstreams}
	for i, target := range targets {
		outcome := results[i]
		if outcome.skipped {
			check.Skipped++
			continue
		}
		member := &check.Upstreams[target.upstream].Servers[target.member]
		alive := outcome.latencyMs >= 0
		member.Alive = &alive
		if alive {
			latency := outcome.latencyMs
			member.LatencyMs = &latency
			check.Alive++
		} else {
			check.Down++
		}
	}
	// Members never targeted (variable or unix addresses) stay Alive=nil.
	return check, nil
}

type probeOutcome struct {
	latencyMs int64
	skipped   bool
}

func probeAll(ctx context.Context, r *Runtime, connectionID string, targets []probeTarget, results []probeOutcome) {
	if len(targets) == 0 {
		return
	}
	profile, perr := r.profile(connectionID)
	if perr != nil {
		for i := range results {
			results[i].skipped = true
		}
		return
	}
	sem := make(chan struct{}, probeWorkers)
	var wg sync.WaitGroup
	for i, target := range targets {
		wg.Add(1)
		go func(i int, target probeTarget) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			latency, perr := r.pool.Probe(ctx, profile, target.host, target.port, probeTimeout)
			if perr != nil {
				results[i] = probeOutcome{skipped: true}
				return
			}
			results[i] = probeOutcome{latencyMs: latency}
		}(i, target)
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
