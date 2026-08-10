package main

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
)

const (
	ctxSessionRow ctxKey = "session-row"
	ctxClientRow  ctxKey = "client-row"
	ctxCrmSession ctxKey = "crm-session"
)

// crmSession bundles the DB row, owning client and live Session (when present)
// for an authenticated CRM request.
type crmSession struct {
	Row    *sessionRow
	Client *clientRow
	Live   *Session
}

// withSessionToken authenticates CRM calls using X-Session-ID / X-Session-Token
// (also accepted as Authorization: Bearer <token> with the id in the path).
// It verifies the session is enabled and its client is active, then compares
// the SHA-256 of the presented token against the stored hash in constant time
// and stamps token_last_used_at.
func withSessionToken(s *server, h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sid := r.PathValue("id")
		token := crmToken(r)
		if sid == "" || token == "" {
			writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "session id and token required"})
			return
		}
		row, err := s.sessions.store.get(r.Context(), sid)
		if err != nil {
			if errors.Is(err, ErrSessionNotFound) {
				writeJSON(w, http.StatusNotFound, map[string]string{"error": "no such session"})
			} else {
				writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			}
			return
		}
		if row.Status == sessionStatusDisabled {
			writeJSON(w, http.StatusForbidden, map[string]string{"error": "session disabled"})
			return
		}
		cl, err := s.clients.get(r.Context(), row.ClientID)
		if err != nil {
			writeJSON(w, http.StatusForbidden, map[string]string{"error": "client not active"})
			return
		}
		if cl.Status != "active" {
			writeJSON(w, http.StatusForbidden, map[string]string{"error": "client not active"})
			return
		}
		if row.TokenHash == "" || subtle.ConstantTimeCompare([]byte(hashToken(token)), []byte(row.TokenHash)) != 1 {
			writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "invalid session token"})
			return
		}
		_ = s.sessions.store.touchToken(r.Context(), sid)

		live, _ := s.sessions.Get(sid)
		cs := &crmSession{Row: row, Client: cl, Live: live}
		ctx := context.WithValue(r.Context(), ctxSessionRow, row)
		ctx = context.WithValue(ctx, ctxClientRow, cl)
		ctx = context.WithValue(ctx, ctxCrmSession, cs)
		h.ServeHTTP(w, r.WithContext(ctx))
	})
}

func crmToken(r *http.Request) string {
	if t := r.Header.Get("X-Session-Token"); t != "" {
		return t
	}
	if t := r.Header.Get("X-Channel-Token"); t != "" {
		return t
	}
	if auth := r.Header.Get("Authorization"); strings.HasPrefix(auth, "Bearer ") {
		return strings.TrimPrefix(auth, "Bearer ")
	}
	return r.URL.Query().Get("token")
}

func crmFromRequest(w http.ResponseWriter, r *http.Request) *crmSession {
	cs, ok := r.Context().Value(ctxCrmSession).(*crmSession)
	if !ok || cs == nil || cs.Row == nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "no such session"})
		return nil
	}
	return cs
}

func (s *server) handleCRMStatus(w http.ResponseWriter, r *http.Request) {
	cs := crmFromRequest(w, r)
	if cs == nil {
		return
	}
	live := cs.Live
	state := sessionStatusDisconnected
	paired := false
	jid := ""
	if live != nil {
		state = live.authState()
		paired = live.auth.Paired
		if id := live.client.Store.ID; id != nil {
			jid = id.String()
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"session": map[string]any{
			"id":          cs.Row.ID,
			"name":        cs.Row.Name,
			"status":      state,
			"paired":      paired,
			"jid":         jid,
			"phoneNumber": cs.Row.PhoneNumber,
		},
		"client": map[string]any{
			"id":   cs.Client.ID,
			"name": cs.Client.Name,
			"slug": cs.Client.Slug,
		},
	})
}

func (s *server) handleCRMCall(w http.ResponseWriter, r *http.Request) {
	cs := crmFromRequest(w, r)
	if cs == nil {
		return
	}
	if cs.Live == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "session not connected"})
		return
	}
	s.doChannelCall(cs.Live, w, r)
}

func (s *server) handleCRMCalls(w http.ResponseWriter, r *http.Request) {
	cs := crmFromRequest(w, r)
	if cs == nil {
		return
	}
	s.broker.mu.RLock()
	calls := make([]CallRecord, 0)
	for _, c := range s.broker.calls {
		if c.SessionID == cs.Row.ID {
			calls = append(calls, *c)
		}
	}
	s.broker.mu.RUnlock()
	writeJSON(w, http.StatusOK, map[string]any{"calls": calls})
}

func (s *server) handleCRMCallGet(w http.ResponseWriter, r *http.Request) {
	cs := crmFromRequest(w, r)
	if cs == nil {
		return
	}
	callID := r.PathValue("callId")
	if rec, ok := s.broker.getCall(callID); ok && rec.SessionID == cs.Row.ID {
		writeJSON(w, http.StatusOK, map[string]any{"call": rec})
		return
	}
	writeJSON(w, http.StatusNotFound, map[string]string{"error": "no such call"})
}

func (s *server) handleCRMHistory(w http.ResponseWriter, r *http.Request) {
	cs := crmFromRequest(w, r)
	if cs == nil {
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"calls": s.broker.historyRows(cs.Row.ID, 100)})
}

func (s *server) handleCRMRecordings(w http.ResponseWriter, r *http.Request) {
	cs := crmFromRequest(w, r)
	if cs == nil {
		return
	}
	rows, err := s.sessions.recStore.listBySession(r.Context(), cs.Row.ID, cs.Row.ClientID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"recordings": rows})
}

func (s *server) handleCRMRecordingGet(w http.ResponseWriter, r *http.Request) {
	cs := crmFromRequest(w, r)
	if cs == nil {
		return
	}
	rec, err := s.sessions.recStore.getScoped(r.Context(), r.PathValue("recordingId"), cs.Row.ID, cs.Row.ClientID)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "no such recording"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"recording": rec})
}

func (s *server) handleCRMWebhook(w http.ResponseWriter, r *http.Request) {
	cs := crmFromRequest(w, r)
	if cs == nil {
		return
	}
	var body struct {
		URL string `json:"url"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid body"})
		return
	}
	url := strings.TrimSpace(body.URL)
	if err := s.sessions.store.setWebhook(r.Context(), cs.Row.ID, url); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	if cs.Live != nil {
		cs.Live.mu.Lock()
		cs.Live.webhook = url
		cs.Live.mu.Unlock()
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok", "webhook": url})
}

func (s *server) handleCRMEndCall(w http.ResponseWriter, r *http.Request) {
	cs := crmFromRequest(w, r)
	if cs == nil {
		return
	}
	callID := r.PathValue("callId")
	if cs.Live != nil {
		if ac, ok := cs.Live.reg.get(callID); ok {
			_ = ac.cm.EndCall(r.Context(), "user-ended")
		}
		cs.Live.removeCall(callID)
	}
	s.broker.endCall(callID, "user-ended")
	writeJSON(w, http.StatusOK, map[string]any{"status": "ended", "callId": callID})
}

// authState returns the live session state name, falling back to the DB status.
func (s *Session) authState() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.auth.State != "" && s.auth.State != "connecting" {
		return s.auth.State
	}
	if s.status != "" {
		return s.status
	}
	return sessionStatusDisconnected
}
