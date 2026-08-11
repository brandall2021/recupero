package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"wacalls/internal/voip/core"

	"go.mau.fi/whatsmeow/types"
)

func (s *server) routes() http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("POST /api/auth/register", s.handleRegister)
	mux.HandleFunc("POST /api/auth/login", s.handleLogin)
	mux.HandleFunc("GET /api/auth/me", s.handleMe)
	mux.HandleFunc("POST /api/auth/reseed", s.handleReseed)

	mux.Handle("GET /api/users", withRole(rolePlatformAdmin)(http.HandlerFunc(s.handleUserList)))
	mux.Handle("POST /api/users", withRole(rolePlatformAdmin)(http.HandlerFunc(s.handleUserCreate)))
	mux.Handle("PUT /api/users/{id}", withRole(rolePlatformAdmin)(http.HandlerFunc(s.handleUserUpdate)))
	mux.Handle("DELETE /api/users/{id}", withRole(rolePlatformAdmin)(http.HandlerFunc(s.handleUserDelete)))

	mux.Handle("GET /api/sessions", withAuth(http.HandlerFunc(s.handleSessionList)))
	mux.Handle("POST /api/sessions", withAuth(http.HandlerFunc(s.handleSessionCreate)))
	mux.Handle("DELETE /api/sessions/{sid}", withAuth(http.HandlerFunc(s.handleSessionDelete)))
	mux.Handle("POST /api/sessions/{sid}/logout", withAuth(http.HandlerFunc(s.handleSessionLogout)))
	mux.Handle("POST /api/sessions/{sid}/pair", withAuth(http.HandlerFunc(s.handleSessionPair)))
	mux.Handle("POST /api/sessions/{sid}/calls", withAuth(http.HandlerFunc(s.handleStartCall)))
	mux.Handle("POST /api/sessions/{sid}/calls/{id}/webrtc", withAuth(http.HandlerFunc(s.handleWebRTC)))
	mux.Handle("POST /api/sessions/{sid}/calls/{id}/accept", withAuth(http.HandlerFunc(s.handleAccept)))
	mux.Handle("POST /api/sessions/{sid}/calls/{id}/reject", withAuth(http.HandlerFunc(s.handleReject)))
	mux.Handle("DELETE /api/sessions/{sid}/calls/{id}", withAuth(http.HandlerFunc(s.handleEndCall)))
	mux.Handle("GET /api/sessions/{sid}/history", withAuth(http.HandlerFunc(s.handleHistory)))
	mux.Handle("GET /api/sessions/{sid}/recordings", withAuth(http.HandlerFunc(s.handleRecordingsList)))
	mux.Handle("GET /api/recordings/{rid}/download", withAuth(http.HandlerFunc(s.handleRecordingDownload)))

	mux.Handle("GET /api/events", withAuth(http.HandlerFunc(s.handleEvents)))

	mux.Handle("GET /api/dashboard", withAuth(http.HandlerFunc(s.handleDashboard)))

	// Superadmin platform management
	platform := withRole(rolePlatformAdmin)
	mux.Handle("GET /api/platform/clients", platform(http.HandlerFunc(s.handlePlatformClientList)))
	mux.Handle("POST /api/platform/clients", platform(http.HandlerFunc(s.handlePlatformClientCreate)))
	mux.Handle("GET /api/platform/clients/{clientId}", platform(http.HandlerFunc(s.handlePlatformClientGet)))
	mux.Handle("PUT /api/platform/clients/{clientId}", platform(http.HandlerFunc(s.handlePlatformClientUpdate)))
	mux.Handle("DELETE /api/platform/clients/{clientId}", platform(http.HandlerFunc(s.handlePlatformClientDelete)))
	mux.Handle("PATCH /api/platform/clients/{clientId}/status", platform(http.HandlerFunc(s.handlePlatformClientStatus)))
	mux.Handle("PATCH /api/platform/clients/{clientId}/limits", platform(http.HandlerFunc(s.handlePlatformClientLimits)))

	// Backwards-compatible per-channel API (authenticated with the channel token)
	channel := func(h http.Handler) http.Handler {
		return withChannelAuth(withSession(s, h))
	}
	mux.Handle("GET /api/channels/{id}", channel(http.HandlerFunc(s.handleChannelInfo)))
	mux.Handle("POST /api/channels/{id}/calls", channel(http.HandlerFunc(s.handleChannelCall)))
	mux.Handle("GET /api/channels/{id}/history", channel(http.HandlerFunc(s.handleChannelHistory)))
	mux.Handle("GET /api/channels/{id}/recordings", channel(http.HandlerFunc(s.handleChannelRecordings)))
	mux.Handle("DELETE /api/channels/{id}/calls/{callId}", channel(http.HandlerFunc(s.handleChannelEndCall)))
	mux.Handle("POST /api/channels/{id}/webhook", channel(http.HandlerFunc(s.handleChannelWebhook)))

	// CRM API (authenticated with session id + token)
	crm := func(h http.Handler) http.Handler {
		return withSessionToken(s, h)
	}
	mux.Handle("GET /api/v1/sessions/{id}", crm(http.HandlerFunc(s.handleCRMStatus)))
	mux.Handle("GET /api/v1/sessions/{id}/status", crm(http.HandlerFunc(s.handleCRMStatus)))
	mux.Handle("POST /api/v1/sessions/{id}/calls", crm(http.HandlerFunc(s.handleCRMCall)))
	mux.Handle("GET /api/v1/sessions/{id}/calls", crm(http.HandlerFunc(s.handleCRMCalls)))
	mux.Handle("GET /api/v1/sessions/{id}/calls/{callId}", crm(http.HandlerFunc(s.handleCRMCallGet)))
	mux.Handle("DELETE /api/v1/sessions/{id}/calls/{callId}", crm(http.HandlerFunc(s.handleCRMEndCall)))
	mux.Handle("GET /api/v1/sessions/{id}/history", crm(http.HandlerFunc(s.handleCRMHistory)))
	mux.Handle("GET /api/v1/sessions/{id}/recordings", crm(http.HandlerFunc(s.handleCRMRecordings)))
	mux.Handle("GET /api/v1/sessions/{id}/recordings/{recordingId}", crm(http.HandlerFunc(s.handleCRMRecordingGet)))
	mux.Handle("PUT /api/v1/sessions/{id}/webhook", crm(http.HandlerFunc(s.handleCRMWebhook)))

	if s.staticDir != "" {
		if _, err := os.Stat(s.staticDir); err == nil {
			mux.Handle("/", http.FileServer(http.Dir(s.staticDir)))
		}
	}
	return withCORS(mux)
}

func withCORS(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, X-Client-Id, Authorization, X-Session-Id, X-Session-Token, X-Channel-Token")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		h.ServeHTTP(w, r)
	})
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}

func clientID(r *http.Request) string {
	if id := r.Header.Get("X-Client-Id"); id != "" {
		return id
	}
	return r.URL.Query().Get("clientId")
}

func (s *server) sessionByID(w http.ResponseWriter, r *http.Request) *Session {
	sid := r.PathValue("sid")
	sess, ok := s.sessions.Get(sid)
	if !ok {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "no such session"})
		return nil
	}
	if scope := s.userClientScope(r); scope != "" && scope != sess.clientID {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "no such session"})
		return nil
	}
	return sess
}

func (s *server) handleEvents(w http.ResponseWriter, r *http.Request) {
	scope := s.userClientScope(r)
	s.broker.serveSSE(w, r, scope)
}

// userClientScope returns the client id a client_admin is restricted to, or ""
// for platform admins (no restriction).
func (s *server) userClientScope(r *http.Request) string {
	claims := userClaims(r)
	if claims == nil || claims.Role != roleClientAdmin {
		return ""
	}
	return claims.ClientID
}

func (s *server) handleSessionList(w http.ResponseWriter, r *http.Request) {
	scope := s.userClientScope(r)
	if scope != "" {
		rows, err := s.sessions.store.listByClient(r.Context(), scope)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		out := make([]SessionInfo, 0, len(rows))
		for _, row := range rows {
			info := SessionInfo{
				ID: row.ID, ClientID: row.ClientID, Name: row.Name, JID: row.JID,
				PhoneNumber: row.PhoneNumber, Webhook: row.Webhook,
				TokenConfigured: row.TokenHash != "",
			}
			if sess, ok := s.sessions.Get(row.ID); ok {
				info = sess.info()
			} else {
				info.State = row.Status
				info.Paired = row.JID != ""
			}
			out = append(out, info)
		}
		writeJSON(w, http.StatusOK, map[string]any{"sessions": out})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"sessions": s.sessions.infos()})
}

func (s *server) handleSessionCreate(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Name     string `json:"name"`
		ClientID string `json:"clientId"`
	}
	_ = json.NewDecoder(r.Body).Decode(&body)
	name := strings.TrimSpace(body.Name)
	if name == "" {
		name = "Session"
	}
	clientID := body.ClientID
	if clientID == "" {
		clientID = s.userClientScope(r)
	}
	if clientID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "clientId required for platform admins"})
		return
	}
	// a client_admin may only create sessions for its own client
	if scope := s.userClientScope(r); scope != "" && scope != clientID {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "forbidden"})
		return
	}
	id, token, err := s.sessions.Create(clientID, name)
	if err != nil {
		if errors.Is(err, ErrSessionLimitReached) {
			writeJSON(w, http.StatusTooManyRequests, map[string]string{
				"error":   "session_limit_reached",
				"message": "el límite de sesiones del cliente fue alcanzado",
			})
			return
		}
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{
		"session":     map[string]string{"id": id, "name": name, "status": sessionStatusPending},
		"credentials": map[string]string{"sessionId": id, "token": token},
	})
}

func (s *server) handleSessionDelete(w http.ResponseWriter, r *http.Request) {
	if s.sessionByID(w, r) == nil {
		return
	}
	if err := s.sessions.Delete(r.Context(), r.PathValue("sid")); err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": err.Error()})
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *server) handleSessionLogout(w http.ResponseWriter, r *http.Request) {
	if s.sessionByID(w, r) == nil {
		return
	}
	if err := s.sessions.Logout(r.Context(), r.PathValue("sid")); err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": err.Error()})
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *server) handleSessionPair(w http.ResponseWriter, r *http.Request) {
	if s.sessionByID(w, r) == nil {
		return
	}
	if err := s.sessions.Pair(r.PathValue("sid")); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *server) handleStartCall(w http.ResponseWriter, r *http.Request) {
	if sess := s.sessionByID(w, r); sess != nil {
		s.doStartCall(sess, w, r)
	}
}

func (s *server) handleWebRTC(w http.ResponseWriter, r *http.Request) {
	if sess := s.sessionByID(w, r); sess != nil {
		s.doWebRTC(sess, w, r)
	}
}

func (s *server) handleAccept(w http.ResponseWriter, r *http.Request) {
	if sess := s.sessionByID(w, r); sess != nil {
		s.doAccept(sess, w, r)
	}
}

func (s *server) handleReject(w http.ResponseWriter, r *http.Request) {
	if sess := s.sessionByID(w, r); sess != nil {
		s.doReject(sess, w, r)
	}
}

func (s *server) handleEndCall(w http.ResponseWriter, r *http.Request) {
	if sess := s.sessionByID(w, r); sess != nil {
		s.doEndCall(sess, w, r)
	}
}

func (s *server) handleHistory(w http.ResponseWriter, r *http.Request) {
	if sess := s.sessionByID(w, r); sess != nil {
		writeJSON(w, http.StatusOK, map[string]any{"rows": s.broker.historyRows(sess.id, 50)})
	}
}

func (s *server) doStartCall(sess *Session, w http.ResponseWriter, r *http.Request) {
	if sess.client.Store.ID == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "not paired"})
		return
	}
	var body struct {
		Phone      string `json:"phone"`
		DurationMs int    `json:"duration_ms"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || strings.TrimSpace(body.Phone) == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "phone required"})
		return
	}
	owner := clientID(r)
	if other := s.broker.ownerActiveCall(owner); other != "" {
		writeJSON(w, http.StatusConflict, map[string]string{"error": "operator already on a call"})
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
		SessionID: sess.id, CallID: callID, Owner: &owner, Direction: "outbound", Peer: peer.String(),
		StartedAt: time.Now().UnixMilli(), Status: StatusRinging,
	})
	writeJSON(w, http.StatusOK, map[string]any{"call": map[string]string{"callId": callID}})
}

func (s *server) doWebRTC(sess *Session, w http.ResponseWriter, r *http.Request) {
	callID := r.PathValue("id")
	s.log.Info("doWebRTC called", "session", sess.id, "call_id", callID)
	ac, ok := sess.reg.get(callID)
	if !ok {
		s.log.Warn("doWebRTC: call not found", "call_id", callID)
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "no such call"})
		return
	}
	var body struct {
		SDPOffer string `json:"sdp_offer"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.SDPOffer == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "sdp_offer required"})
		return
	}
	bridge, answer, err := NewBridge(body.SDPOffer, s.log)
	if err != nil {
		s.log.Error("doWebRTC: bridge creation failed", "err", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	bridge.OnBrowserPCM = func(pcm []float32) {
		if ac.recorder != nil {
			ac.recorder.WritePCM(pcm)
		}
		ac.cm.FeedCapturedPCM(pcm)
	}
	bridge.OnTerminalICE = func() {
		go sess.terminateCall(callID, core.EndCallReasonUserEnded)
	}
	sess.setBridge(callID, bridge)
	s.log.Info("doWebRTC: bridge created successfully", "call_id", callID)
	writeJSON(w, http.StatusOK, map[string]string{"sdp_answer": answer})
}

func (s *server) doAccept(sess *Session, w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	ac, ok := sess.reg.get(id)
	if !ok {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "no such call"})
		return
	}
	owner := clientID(r)
	if other := s.broker.ownerActiveCall(owner); other != "" && other != id {
		writeJSON(w, http.StatusConflict, map[string]string{"error": "operator already on a call"})
		return
	}
	if !s.broker.setOwner(id, owner) {
		writeJSON(w, http.StatusConflict, map[string]string{"error": "claimed by another client"})
		return
	}
	s.broker.emitIncomingClaimed(sess.id, id, owner)
	if err := ac.cm.AcceptCall(r.Context(), id); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"call": map[string]string{"callId": id}})
}

func (s *server) doReject(sess *Session, w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if ac, ok := sess.reg.get(id); ok {
		_ = ac.cm.RejectCall(r.Context(), id, core.EndCallReasonDeclined)
	}
	sess.removeCall(id)
	s.broker.endCall(id, string(core.EndCallReasonDeclined))
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *server) doEndCall(sess *Session, w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if ac, ok := sess.reg.get(id); ok {
		_ = ac.cm.EndCall(r.Context(), core.EndCallReasonUserEnded)
	}
	sess.removeCall(id)
	s.broker.endCall(id, string(core.EndCallReasonUserEnded))
	w.WriteHeader(http.StatusNoContent)
}

func normalizePhone(p string) string {
	p = strings.TrimSpace(p)
	p = strings.TrimPrefix(p, "+")
	var b strings.Builder
	for _, c := range p {
		if c >= '0' && c <= '9' {
			b.WriteRune(c)
		}
	}
	return b.String()
}

func (s *server) handleRecordingsList(w http.ResponseWriter, r *http.Request) {
	if sess := s.sessionByID(w, r); sess != nil {
		rows, err := s.sessions.recStore.listBySession(r.Context(), sess.id, sess.clientID)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"recordings": rows})
	}
}

func (s *server) handleRecordingDownload(w http.ResponseWriter, r *http.Request) {
	rec, err := s.sessions.recStore.get(r.Context(), r.PathValue("rid"))
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "recording not found"})
		return
	}
	if scope := s.userClientScope(r); scope != "" {
		sess, ok := s.sessions.Get(rec.SessionID)
		if !ok || sess.clientID != scope {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "recording not found"})
			return
		}
	}
	if _, err := os.Stat(rec.FilePath); os.IsNotExist(err) {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "file not found"})
		return
	}
	w.Header().Set("Content-Type", "audio/wav")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, filepath.Base(rec.FilePath)))
	http.ServeFile(w, r, rec.FilePath)
}
