package main

import (
	"context"
	"encoding/json"
	"strings"
	"unicode/utf8"
)

type updateKeyInput struct {
	ID      int64  `json:"id"`
	Name    string `json:"name"`
	GroupID *int64 `json:"groupId"`
}

type deleteKeyInput struct {
	ID int64 `json:"id"`
}

type idResult struct {
	ID int64 `json:"id"`
}

func (s *accountService) updateKey(ctx context.Context, raw json.RawMessage) (any, error) {
	id, name, groupID, err := parseUpdateKeyInput(raw)
	if err != nil {
		return rejected(err), nil
	}
	record, err := s.session(ctx)
	if err != nil {
		return rejected(err), nil
	}
	key, err := s.panel.updateKey(ctx, record.AccessToken, id, name, groupID)
	if err != nil {
		if codeOf(err) == "panel_unauthorized" {
			s.clearRecord()
		}
		return rejected(err), nil
	}
	if key.ID == 0 {
		key.ID = id
	}
	if key.Name == "" {
		key.Name = name
	}
	return success(mapKey(key)), nil
}

func (s *accountService) deleteKey(ctx context.Context, raw json.RawMessage) (any, error) {
	id, err := parseDeleteKeyInput(raw)
	if err != nil {
		return rejected(err), nil
	}
	record, err := s.session(ctx)
	if err != nil {
		return rejected(err), nil
	}
	if err := s.panel.deleteKey(ctx, record.AccessToken, id); err != nil {
		if codeOf(err) == "panel_unauthorized" {
			s.clearRecord()
		}
		return rejected(err), nil
	}
	return success(idResult{ID: id}), nil
}

func parseUpdateKeyInput(raw json.RawMessage) (int64, string, *int64, error) {
	var input updateKeyInput
	if json.Unmarshal(raw, &input) != nil || input.ID <= 0 {
		return 0, "", nil, coded("invalid_id", messageInvalidID)
	}
	name := strings.TrimSpace(input.Name)
	if name == "" || utf8.RuneCountInString(name) > maxKeyNameRunes {
		return 0, "", nil, coded("invalid_name", messageInvalidName)
	}
	if input.GroupID != nil && *input.GroupID < 0 {
		return 0, "", nil, coded("invalid_id", messageInvalidID)
	}
	return input.ID, name, input.GroupID, nil
}

func parseDeleteKeyInput(raw json.RawMessage) (int64, error) {
	var input deleteKeyInput
	if json.Unmarshal(raw, &input) != nil || input.ID <= 0 {
		return 0, coded("invalid_id", messageInvalidID)
	}
	return input.ID, nil
}
