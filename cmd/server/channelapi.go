package main

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"wacalls/internal/voip/core"

	"go.mau.fi/whatsmeow/types"
)

type ctxKey string

const ctxSession ctxKey = "session"

func withSession(s *server, h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sid := r.PathValue("id")
		sess, ok := s.sessions.Get(sid)
		if !ok {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "no such channel"})
			return
		}
		ctx := context.WithValue(r.Context(), ctxSession, sess)
		h.ServeHTTP(w, r.WithContext(ctx))
	})
}

func channelFromRequest(w http.ResponseWriter, r *http.Request) *Session {
	sid := r.PathValue("id")
	sess, ok := r.Context().Value(ctxSession).(*Session)
	if !ok || sess == nil || sess.id != sid {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "no such channel"})
		return nil
	}
	return sess
}

func withChannelAuth(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sid := r.PathValue("id")
		sess, ok := r.Context().Value(ctxSession).(*Session)
		if !ok || sess == nil || sess.id != sid {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "no such channel"})
			return
		}
		token := channelToken(r)
		if token == "" || subtle.ConstantTimeCompare([]byte(token), []byte(sess.token)) != 1 {
			writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "invalid channel token"})
			return
		}
		h.ServeHTTP(w, r)
	})
}

func channelToken(r *http.Request) string {
	if t := r.Header.Get("X-Channel-Token"); t != "" {
		return t
	}
	if auth := r.Header.Get("Authorization"); strings.HasPrefix(auth, "Bearer ") {
		return strings.TrimPrefix(auth, "Bearer ")
	}
	return r.URL.Query().Get("token")
}

func (s *server) handleChannelInfo(w http.ResponseWriter, r *http.Request) {
	if sess := channelFromRequest(w, r); sess != nil {
		writeJSON(w, http.StatusOK, map[string]any{"channel": sess.info()})
	}
}

func (s *server) handleChannelCall(w http.ResponseWriter, r *http.Request) {
	if sess := channelFromRequest(w, r); sess != nil {
		s.doChannelCall(sess, w, r)
	}
}

func (s *server) doChannelCall(sess *Session, w http.ResponseWriter, r *http.Request) {
	if sess.client.Store.ID == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "not paired"})
		return
	}
	var body struct {
		Phone string `json:"phone"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || strings.TrimSpace(body.Phone) == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "phone required"})
		return
	}
	if max := s.sessions.maxCalls; max > 0 && sess.reg.count() >= max {
		writeJSON(w, http.StatusTooManyRequests, map[string]string{"error": "max concurrent calls"})
		return
	}
	peer := types.NewJID(normalizePhone(body.Phone), types.DefaultUserServer)
	callID, err := sess.startOutgoing(r.Context(), peer, false)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	s.broker.upsertCall(CallRecord{
		SessionID: sess.id, CallID: callID, Direction: "outbound", Peer: peer.String(),
		StartedAt: time.Now().UnixMilli(), Status: StatusRinging,
	})
	s.sessions.emitWebhook(sess, "call.outbound", map[string]any{
		"callId": callID, "peer": peer.String(), "direction": "outbound", "status": "ringing",
	})
	writeJSON(w, http.StatusOK, map[string]string{"callId": callID})
}

func (s *server) handleChannelHistory(w http.ResponseWriter, r *http.Request) {
	if sess := channelFromRequest(w, r); sess != nil {
		writeJSON(w, http.StatusOK, map[string]any{"calls": s.broker.historyRows(sess.id, 50)})
	}
}

func (s *server) handleChannelRecordings(w http.ResponseWriter, r *http.Request) {
	if sess := channelFromRequest(w, r); sess != nil {
		rows, err := s.sessions.recStore.listBySession(r.Context(), sess.id)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"recordings": rows})
	}
}

func (s *server) handleChannelEndCall(w http.ResponseWriter, r *http.Request) {
	if sess := channelFromRequest(w, r); sess != nil {
		callID := r.PathValue("callId")
		if ac, ok := sess.reg.get(callID); ok {
			_ = ac.cm.EndCall(r.Context(), core.EndCallReasonUserEnded)
		}
		sess.removeCall(callID)
		s.broker.endCall(callID, string(core.EndCallReasonUserEnded))
		w.WriteHeader(http.StatusNoContent)
	}
}

func (s *server) handleChannelWebhook(w http.ResponseWriter, r *http.Request) {
	if sess := channelFromRequest(w, r); sess != nil {
		var body struct {
			URL string `json:"url"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid body"})
			return
		}
		if err := s.sessions.SetWebhook(r.Context(), sess.id, strings.TrimSpace(body.URL)); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok", "webhook": strings.TrimSpace(body.URL)})
	}
}
