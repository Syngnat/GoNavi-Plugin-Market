package commands

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"github.com/Syngnat/GoNavi-Plugin-Market/plugins/nginx-inspector/internal/rpcerr"
	"github.com/Syngnat/GoNavi-Plugin-Market/plugins/nginx-inspector/internal/store"
)

type connectionInput struct {
	ID            string `json:"id,omitempty"`
	Name          string `json:"name"`
	Host          string `json:"host"`
	Port          int    `json:"port"`
	Username      string `json:"username"`
	AuthType      string `json:"authType"`
	Password      string `json:"password,omitempty"`
	KeyPath       string `json:"keyPath,omitempty"`
	KeyPassphrase string `json:"keyPassphrase,omitempty"`
}

func (r *Runtime) listConnections(ctx context.Context, args json.RawMessage) (any, error) {
	if _, perr := decodeArgs[struct{}](args); perr != nil {
		return nil, perr
	}
	if r.store == nil {
		return nil, rpcerr.E(rpcerr.CodeStoreUnavailable, "profile storage is unavailable on this machine")
	}
	profiles, err := r.store.List()
	if err != nil {
		return nil, rpcerr.E(rpcerr.CodeStoreUnavailable, "list connection profiles: "+err.Error())
	}
	return map[string]any{"connections": store.MaskProfiles(profiles)}, nil
}

func (r *Runtime) saveConnection(ctx context.Context, args json.RawMessage) (any, error) {
	input, perr := decodeArgs[connectionInput](args)
	if perr != nil {
		return nil, perr
	}
	if r.store == nil {
		return nil, rpcerr.E(rpcerr.CodeStoreUnavailable, "profile storage is unavailable on this machine")
	}
	saved, err := r.store.Save(store.Profile{
		ID:            input.ID,
		Name:          strings.TrimSpace(input.Name),
		Host:          strings.TrimSpace(input.Host),
		Port:          input.Port,
		Username:      strings.TrimSpace(input.Username),
		AuthType:      input.AuthType,
		Password:      input.Password,
		KeyPath:       strings.TrimSpace(input.KeyPath),
		KeyPassphrase: input.KeyPassphrase,
	})
	if err != nil {
		return nil, rpcerr.E(rpcerr.CodeInvalidArgs, err.Error())
	}
	r.pool.Invalidate(saved.ID)
	r.invalidateInfo(saved.ID)
	return map[string]any{"connection": maskedOne(saved)}, nil
}

func (r *Runtime) deleteConnection(ctx context.Context, args json.RawMessage) (any, error) {
	input, perr := decodeArgs[struct {
		ID string `json:"id"`
	}](args)
	if perr != nil {
		return nil, perr
	}
	if r.store == nil {
		return nil, rpcerr.E(rpcerr.CodeStoreUnavailable, "profile storage is unavailable on this machine")
	}
	if err := r.store.Delete(input.ID); err != nil {
		return nil, rpcerr.E(rpcerr.CodeProfileNotFound, err.Error())
	}
	r.pool.Invalidate(input.ID)
	r.invalidateInfo(input.ID)
	return map[string]any{"deleted": input.ID}, nil
}

func (r *Runtime) testConnection(ctx context.Context, args json.RawMessage) (any, error) {
	input, perr := decodeArgs[struct {
		ID string `json:"id"`
	}](args)
	if perr != nil {
		return nil, perr
	}
	profile, perr := r.profile(input.ID)
	if perr != nil {
		return nil, perr
	}
	start := time.Now()
	if _, perr := r.pool.Client(ctx, profile); perr != nil {
		return nil, perr
	}
	if _, perr := r.pool.Output(ctx, profile, "echo gonavi-ok", 10*time.Second); perr != nil {
		return nil, perr
	}
	fingerprint := profile.Fingerprint
	return map[string]any{
		"ok":          true,
		"fingerprint": fingerprint,
		"latencyMs":   time.Since(start).Milliseconds(),
	}, nil
}

func maskedOne(p store.Profile) store.MaskedProfile {
	return store.MaskProfiles([]store.Profile{p})[0]
}
