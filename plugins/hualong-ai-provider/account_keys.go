package main

import (
	"context"
	"encoding/json"
	"strings"
	"unicode/utf8"
)

const maxKeyNameRunes = 64

type groupItem struct {
	ID             int64   `json:"id"`
	Name           string  `json:"name"`
	RateMultiplier float64 `json:"rateMultiplier"`
	Platform       string  `json:"platform,omitempty"`
}

type groupListResult struct {
	State string      `json:"state"`
	Items []groupItem `json:"items"`
}

type createKeyInput struct {
	Name    string `json:"name"`
	GroupID int64  `json:"groupId"`
}

func mapKey(key panelKey) keyItem {
	item := keyItem{
		ID: key.ID, Name: key.Name, Key: key.Key, Status: key.Status,
		GroupID: key.GroupID, Quota: key.Quota, QuotaUsed: key.QuotaUsed,
	}
	if key.LastUsedAt != nil {
		item.LastUsedAt = *key.LastUsedAt
	}
	if key.Group != nil {
		item.GroupName = key.Group.Name
		item.RateMultiplier = key.Group.RateMultiplier
		item.Platform = key.Group.Platform
	}
	return item
}

func (s *accountService) groups(ctx context.Context, _ json.RawMessage) (any, error) {
	record, err := s.session(ctx)
	if err != nil {
		return rejected(err), nil
	}
	groups, err := s.panel.groups(ctx, record.AccessToken)
	if err != nil {
		if codeOf(err) == "panel_unauthorized" {
			s.clearRecord()
		}
		return rejected(err), nil
	}
	items := make([]groupItem, 0, len(groups))
	for _, group := range groups {
		items = append(items, groupItem{
			ID: group.ID, Name: group.Name,
			RateMultiplier: group.RateMultiplier, Platform: group.Platform,
		})
	}
	return success(groupListResult{State: stateReady, Items: items}), nil
}

func (s *accountService) createKey(ctx context.Context, raw json.RawMessage) (any, error) {
	name, groupID, err := parseCreateKeyInput(raw)
	if err != nil {
		return rejected(err), nil
	}
	record, err := s.session(ctx)
	if err != nil {
		return rejected(err), nil
	}
	key, err := s.panel.createKey(ctx, record.AccessToken, name, groupID)
	if err != nil {
		if codeOf(err) == "panel_unauthorized" {
			s.clearRecord()
		}
		return rejected(err), nil
	}
	return success(mapKey(key)), nil
}

func parseCreateKeyInput(raw json.RawMessage) (string, int64, error) {
	var input createKeyInput
	if len(strings.TrimSpace(string(raw))) > 0 && json.Unmarshal(raw, &input) != nil {
		return "", 0, coded("invalid_name", messageInvalidName)
	}
	name := strings.TrimSpace(input.Name)
	if name == "" || utf8.RuneCountInString(name) > maxKeyNameRunes {
		return "", 0, coded("invalid_name", messageInvalidName)
	}
	if input.GroupID < 0 {
		return "", 0, coded("invalid_name", messageInvalidName)
	}
	return name, input.GroupID, nil
}
