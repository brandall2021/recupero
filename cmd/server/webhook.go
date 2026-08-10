package main

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/google/uuid"
)

// webhookMaxAttempts is how many times a signed webhook is delivered before
// giving up.
const webhookMaxAttempts = 3

type webhookClient struct {
	hc  *http.Client
	log *slog.Logger
}

func newWebhookClient() *webhookClient {
	return &webhookClient{hc: &http.Client{Timeout: 10 * time.Second}, log: slog.Default()}
}

func newEventID() string {
	return uuid.New().String()
}

// signWebhook computes the sha256=… signature over the raw body using the
// session token hash as the HMAC key. The CRM can verify it with
// HMAC-SHA256(sha256(token), body).
func signWebhook(tokenHash string, body []byte) string {
	mac := hmac.New(sha256.New, []byte(tokenHash))
	mac.Write(body)
	return "sha256=" + hex.EncodeToString(mac.Sum(nil))
}

// post delivers a signed webhook with retries. The signature proves the event
// originates from WaCalls for this session and is verifiable only by holders
// of the session token.
func (w *webhookClient) post(ctx context.Context, url, tokenHash string, ev any) {
	body, err := json.Marshal(ev)
	if err != nil {
		w.log.Warn("webhook marshal failed", "err", err)
		return
	}
	eventID, _ := ev.(map[string]any)["eventId"].(string)
	timestamp := time.Now().UnixMilli()

	attempt := 0
	for {
		attempt++
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
		if err != nil {
			return
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-WaCalls-Event-ID", eventID)
		req.Header.Set("X-WaCalls-Timestamp", itoa(timestamp))
		req.Header.Set("X-WaCalls-Signature", signWebhook(tokenHash, body))

		resp, err := w.hc.Do(req)
		if err == nil {
			resp.Body.Close()
			if resp.StatusCode >= 200 && resp.StatusCode < 300 {
				w.log.Info("webhook delivered", "eventId", eventID, "status", resp.StatusCode, "attempt", attempt)
				return
			}
			w.log.Warn("webhook rejected", "eventId", eventID, "status", resp.StatusCode, "attempt", attempt)
		} else {
			w.log.Warn("webhook request failed", "eventId", eventID, "err", err, "attempt", attempt)
		}
		if attempt >= webhookMaxAttempts {
			w.log.Error("webhook delivery failed after retries", "eventId", eventID, "url", url)
			return
		}
		select {
		case <-time.After(time.Duration(attempt) * time.Second):
		case <-ctx.Done():
			return
		}
	}
}

func itoa(v int64) string {
	return strconv.FormatInt(v, 10)
}
