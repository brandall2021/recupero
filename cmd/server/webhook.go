package main

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"time"
)

type webhookClient struct {
	hc *http.Client
}

func newWebhookClient() *webhookClient {
	return &webhookClient{hc: &http.Client{Timeout: 10 * time.Second}}
}

func (w *webhookClient) post(ctx context.Context, url, channelToken string, ev any) {
	body, err := json.Marshal(ev)
	if err != nil {
		return
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Channel-Token", channelToken)
	req.Header.Set("Authorization", "Bearer "+channelToken)
	resp, err := w.hc.Do(req)
	if err != nil {
		return
	}
	defer resp.Body.Close()
}
