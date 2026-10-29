package main

import (
	"context"
	"encoding/json"
	"sort"
	"sync"
)

// modelScanConcurrency bounds how many keys are queried at once: the gateway is
// asked once per key because each group exposes a different model set.
const modelScanConcurrency = 4

type modelItem struct {
	ID string `json:"id"`
	// Name and ContextWindow are the gateway's own metadata when it publishes any;
	// the host falls back to the models.dev catalog when they are missing.
	Name          string  `json:"name,omitempty"`
	ContextWindow int     `json:"contextWindow,omitempty"`
	KeyIDs        []int64 `json:"keyIds,omitempty"`
}

type modelListResult struct {
	State string      `json:"state"`
	Items []modelItem `json:"items"`
	// FailedKeys counts keys the gateway refused; a partial scan is still useful.
	FailedKeys int `json:"failedKeys,omitempty"`
}

type keyModelScan struct {
	keyID  int64
	models []panelModel
	fail   bool
}

// listModels asks the gateway once per API key and merges every reachable model,
// remembering which keys exposed it so the window can explain the union.
func (s *accountService) listModels(ctx context.Context, _ json.RawMessage) (any, error) {
	record, err := s.session(ctx)
	if err != nil {
		return rejected(err), nil
	}
	keys, err := s.panel.keys(ctx, record.AccessToken)
	if err != nil {
		if codeOf(err) == "panel_unauthorized" {
			s.clearRecord()
		}
		return rejected(err), nil
	}
	if len(keys) == 0 {
		return success(modelListResult{State: stateReady, Items: []modelItem{}}), nil
	}
	scans := s.scanKeyModels(ctx, record.AccessToken, keys)
	merged := make(map[string]*modelItem, len(keys)*4)
	order := make([]string, 0, len(keys)*4)
	failed := 0
	for _, scan := range scans {
		if scan.fail {
			failed++
			continue
		}
		for _, model := range scan.models {
			item, ok := merged[model.ID]
			if !ok {
				item = &modelItem{ID: model.ID}
				merged[model.ID] = item
				order = append(order, model.ID)
			}
			// 名称取第一个非空，窗口取最大值：同一模型被不同分组收录时以更大的为准，
			// 与宿主 models.dev 目录的合并口径一致。
			if item.Name == "" {
				item.Name = model.Name
			}
			if model.ContextWindow > item.ContextWindow {
				item.ContextWindow = model.ContextWindow
			}
			item.KeyIDs = append(item.KeyIDs, scan.keyID)
		}
	}
	if failed == len(scans) {
		return rejected(coded("panel_unavailable", messagePanelUnavailable)), nil
	}
	sort.Strings(order)
	items := make([]modelItem, 0, len(order))
	for _, id := range order {
		items = append(items, *merged[id])
	}
	return success(modelListResult{State: stateReady, Items: items, FailedKeys: failed}), nil
}

func (s *accountService) scanKeyModels(ctx context.Context, accessToken string, keys []panelKey) []keyModelScan {
	scans := make([]keyModelScan, len(keys))
	slots := make(chan struct{}, modelScanConcurrency)
	var wg sync.WaitGroup
	for index, key := range keys {
		scans[index].keyID = key.ID
		if key.ID <= 0 || key.Key == "" {
			scans[index].fail = true
			continue
		}
		wg.Add(1)
		go func(slot int, keyID int64, keyValue string) {
			defer wg.Done()
			slots <- struct{}{}
			defer func() { <-slots }()
			models, err := s.panel.models(ctx, keyValue)
			if err != nil {
				scans[slot].fail = true
				return
			}
			scans[slot].models = models
		}(index, key.ID, key.Key)
	}
	wg.Wait()
	return scans
}
