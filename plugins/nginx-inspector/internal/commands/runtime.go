// Package commands implements the plugin commands over SSH: connection
// management, host discovery, configuration inspection, validation, service
// control, log analysis, TLS certificate review and upstream probing.
package commands

import (
	"bytes"
	"encoding/json"
	"regexp"
	"sync"

	plugin "github.com/Syngnat/GoNavi/sdk/go"
	"github.com/Syngnat/GoNavi-Plugin-Market/plugins/nginx-inspector/internal/rpcerr"
	"github.com/Syngnat/GoNavi-Plugin-Market/plugins/nginx-inspector/internal/sshx"
	"github.com/Syngnat/GoNavi-Plugin-Market/plugins/nginx-inspector/internal/store"
)

// Runtime owns shared state: locale, profile store and SSH pool.
type Runtime struct {
	mu     sync.Mutex
	locale string
	store  *store.Store
	pool   *sshx.Pool
	info   map[string]*HostInfo // cached nginx discovery per profile
}

// NewRuntime opens the profile store. A missing user-config directory only
// degrades persistence; commands report STORE_UNAVAILABLE on use.
func NewRuntime() *Runtime {
	path, err := store.DefaultPath()
	var st *store.Store
	if err == nil {
		st = store.Open(path)
	}
	return &Runtime{store: st, pool: sshx.NewPool(st), info: map[string]*HostInfo{}}
}

// SetLocale records the host locale for future responses.
func (r *Runtime) SetLocale(locale string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.locale = locale
}

// Locale returns the recorded host locale.
func (r *Runtime) Locale() string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.locale
}

// Close releases cached SSH clients.
func (r *Runtime) Close() { r.pool.Close() }

// Handlers returns the command registry declared in the manifest.
func (r *Runtime) Handlers() map[string]plugin.Handler {
	return map[string]plugin.Handler{
		"connections.list":   r.listConnections,
		"connections.save":   r.saveConnection,
		"connections.delete": r.deleteConnection,
		"connections.test":   r.testConnection,
		"host.info":          r.hostInfo,
		"config.tree":        r.configTree,
		"config.get":         r.configGet,
		"config.validate":    r.configValidate,
		"service.status":     r.serviceStatus,
		"service.reload":     r.serviceReload,
		"logs.tail":          r.logsTail,
		"logs.search":        r.logsSearch,
		"access.stats":       r.accessStats,
		"certs.list":         r.certsList,
		"upstreams.check":    r.upstreamsCheck,
	}
}

func decodeArgs[T any](args json.RawMessage) (*T, *plugin.Error) {
	var out T
	if len(args) == 0 {
		return &out, nil
	}
	dec := json.NewDecoder(bytes.NewReader(args))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&out); err != nil {
		return nil, rpcerr.E(rpcerr.CodeInvalidArgs, "invalid command arguments: "+err.Error())
	}
	return &out, nil
}

func (r *Runtime) profile(id string) (store.Profile, *plugin.Error) {
	if r.store == nil {
		return store.Profile{}, rpcerr.E(rpcerr.CodeStoreUnavailable, "profile storage is unavailable on this machine")
	}
	if id == "" {
		return store.Profile{}, rpcerr.E(rpcerr.CodeInvalidArgs, "connectionId is required")
	}
	profile, ok, err := r.store.Get(id)
	if err != nil {
		return store.Profile{}, rpcerr.E(rpcerr.CodeStoreUnavailable, "read connection profile: "+err.Error())
	}
	if !ok {
		return store.Profile{}, rpcerr.E(rpcerr.CodeProfileNotFound, "connection profile not found")
	}
	return profile, nil
}

func (r *Runtime) poolFor() *sshx.Pool { return r.pool }

func (r *Runtime) invalidateInfo(id string) {
	r.mu.Lock()
	delete(r.info, id)
	r.mu.Unlock()
}

func compileFilter(pattern string) (*regexp.Regexp, *plugin.Error) {
	if pattern == "" {
		return nil, nil
	}
	re, err := regexp.Compile(pattern)
	if err != nil {
		return nil, rpcerr.E(rpcerr.CodeInvalidArgs, "invalid filter regex: "+err.Error())
	}
	return re, nil
}
