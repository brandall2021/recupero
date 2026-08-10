package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/store/sqlstore"
	"go.mau.fi/whatsmeow/types"
	waLog "go.mau.fi/whatsmeow/util/log"
)

type SessionManager struct {
	appCtx    context.Context
	container *sqlstore.Container
	broker    *Broker
	store     *sessionStore
	recStore  *recordingStore
	clients   *clientStore
	waLogger  waLog.Logger
	webhook   *webhookClient
	log       *slog.Logger
	maxCalls  int

	mu       sync.RWMutex
	sessions map[string]*Session
	order    []string
}

func newSessionManager(ctx context.Context, container *sqlstore.Container, broker *Broker, store *sessionStore, recStore *recordingStore, clients *clientStore, waLogger waLog.Logger, log *slog.Logger, maxCalls int) *SessionManager {
	return &SessionManager{
		appCtx:    ctx,
		container: container,
		broker:    broker,
		store:     store,
		recStore:  recStore,
		clients:   clients,
		waLogger:  waLogger,
		webhook:   newWebhookClient(),
		log:       log,
		maxCalls:  maxCalls,
		sessions:  map[string]*Session{},
	}
}

func (m *SessionManager) emitWebhook(s *Session, evtType string, payload map[string]any) {
	s.mu.Lock()
	url := s.webhook
	signingKey := s.tokenHash
	s.mu.Unlock()
	if url == "" {
		return
	}
	ev := map[string]any{
		"eventId":   newEventID(),
		"type":      evtType,
		"timestamp": time.Now().UnixMilli(),
		"session": map[string]any{
			"id":   s.id,
			"name": s.name,
		},
	}
	for k, v := range payload {
		ev[k] = v
	}
	m.webhook.post(m.appCtx, url, signingKey, ev)
}

func (m *SessionManager) register(s *Session) {
	m.mu.Lock()
	m.sessions[s.id] = s
	m.order = append(m.order, s.id)
	m.mu.Unlock()
}

func (m *SessionManager) unregister(id string) {
	m.mu.Lock()
	delete(m.sessions, id)
	for i, x := range m.order {
		if x == id {
			m.order = append(m.order[:i], m.order[i+1:]...)
			break
		}
	}
	m.mu.Unlock()
}

func (m *SessionManager) Get(id string) (*Session, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	s, ok := m.sessions[id]
	return s, ok
}

func (m *SessionManager) infos() []SessionInfo {
	m.mu.RLock()
	ordered := make([]*Session, 0, len(m.order))
	for _, id := range m.order {
		if s, ok := m.sessions[id]; ok {
			ordered = append(ordered, s)
		}
	}
	m.mu.RUnlock()
	out := make([]SessionInfo, 0, len(ordered))
	for _, s := range ordered {
		out = append(out, s.info())
	}
	return out
}

func (m *SessionManager) snapshotEvents() []any {
	return []any{map[string]any{"type": "session-list", "sessions": m.infos()}}
}

// activeClient returns the client the session belongs to and whether it is
// allowed to run (active clients only). Callers should short-circuit when ok is false.
func (m *SessionManager) sessionClientOK(ctx context.Context, row *sessionRow) (*clientRow, bool) {
	cl, err := m.clients.get(ctx, row.ClientID)
	if err != nil {
		return nil, false
	}
	if cl.Status != "active" {
		return nil, false
	}
	return cl, true
}

func (m *SessionManager) Restore(ctx context.Context) error {
	rows, err := m.store.list(ctx)
	if err != nil {
		return err
	}
	restored := 0
	for _, row := range rows {
		cl, ok := m.sessionClientOK(ctx, &row)
		if !ok {
			m.log.Warn("skipping session: client missing or not active", "session", row.ID, "client", row.ClientID)
			continue
		}
		_ = cl
		if row.JID == "" {
			m.log.Warn("dropping session with no jid", "session", row.ID)
			_ = m.store.delete(ctx, row.ID)
			continue
		}
		jid, err := types.ParseJID(row.JID)
		if err != nil {
			m.log.Warn("dropping session with unparseable jid", "session", row.ID, "jid", row.JID)
			_ = m.store.delete(ctx, row.ID)
			continue
		}
		device, err := m.container.GetDevice(ctx, jid)
		if err != nil || device == nil {
			m.log.Warn("dropping session with no stored device", "session", row.ID, "jid", row.JID, "err", err)
			_ = m.store.delete(ctx, row.ID)
			continue
		}
		tokenHash := row.TokenHash
		if tokenHash == "" {
			if _, err := m.store.rotateToken(ctx, row.ID); err != nil {
				m.log.Warn("failed to set session token", "session", row.ID, "err", err)
			}
			if refreshed, err := m.store.get(ctx, row.ID); err == nil {
				tokenHash = refreshed.TokenHash
			}
		}
		client := whatsmeow.NewClient(device, m.waLogger)
		s := newSession(m, row.ID, row.ClientID, row.Name, tokenHash, row.Webhook, client)
		m.register(s)
		restored++
		if err := s.connect(ctx); err != nil {
			m.log.Error("session connect failed", "session", row.ID, "err", err)
		}
	}
	m.broker.emitSessionList(m.infos())
	m.log.Info("sessions restored", "count", restored)
	return nil
}

// Create creates a new session for a client, enforcing the client's session
// limit. Returns the session id and the plaintext token (shown only once).
func (m *SessionManager) Create(clientID, name string) (string, string, error) {
	if clientID == "" {
		return "", "", errors.New("client required")
	}
	cl, err := m.clients.get(m.appCtx, clientID)
	if err != nil {
		return "", "", err
	}
	if cl.Status != "active" {
		return "", "", ErrClientNotActive
	}
	id, token, err := m.store.createWithLimit(m.appCtx, clientID, name)
	if err != nil {
		return "", "", err
	}
	client := whatsmeow.NewClient(m.container.NewDevice(), m.waLogger)
	s := newSession(m, id, clientID, name, hashToken(token), "", client)
	m.register(s)
	m.broker.emitSessionList(m.infos())
	if err := s.startPairing(m.appCtx); err != nil {
		m.log.Error("start pairing failed", "session", id, "err", err)
		return "", "", fmt.Errorf("start pairing: %w", err)
	}
	m.log.Info("session created", "session", id, "name", name, "client", clientID)
	return id, token, nil
}

func (m *SessionManager) RegenerateToken(ctx context.Context, id string) (string, error) {
	s, ok := m.Get(id)
	if !ok {
		return "", fmt.Errorf("no session %s", id)
	}
	token, err := m.store.rotateToken(ctx, id)
	if err != nil {
		return "", err
	}
	s.mu.Lock()
	s.tokenHash = hashToken(token)
	s.mu.Unlock()
	m.broker.emitSessionList(m.infos())
	m.log.Info("session token regenerated", "session", id)
	return token, nil
}

func (m *SessionManager) Disable(ctx context.Context, id string) error {
	if err := m.store.setStatus(ctx, id, sessionStatusDisabled); err != nil {
		return err
	}
	s, ok := m.Get(id)
	if ok {
		s.teardownAllCalls()
		s.client.Disconnect()
	}
	m.unregister(id)
	m.broker.emitSessionList(m.infos())
	return nil
}

func (m *SessionManager) Delete(ctx context.Context, id string) error {
	s, ok := m.Get(id)
	if !ok {
		return fmt.Errorf("no session %s", id)
	}
	if s.client.Store.ID != nil {
		if err := s.client.Logout(ctx); err != nil {
			m.log.Warn("logout failed; deleting locally", "session", id, "err", err)
			_ = m.container.DeleteDevice(ctx, s.client.Store)
		}
	} else {
		s.client.Disconnect()
		_ = m.container.DeleteDevice(ctx, s.client.Store)
	}
	s.teardownAllCalls()
	m.unregister(id)
	_ = m.store.delete(ctx, id)
	m.broker.emitSessionList(m.infos())
	m.log.Info("session deleted", "session", id)
	return nil
}

func (m *SessionManager) Logout(ctx context.Context, id string) error {
	s, ok := m.Get(id)
	if !ok {
		return fmt.Errorf("no session %s", id)
	}
	if s.client.Store.ID != nil {
		if err := s.client.Logout(ctx); err != nil {
			m.log.Warn("logout failed", "session", id, "err", err)
		}
	}
	s.replaceClient(whatsmeow.NewClient(m.container.NewDevice(), m.waLogger))
	_ = m.store.setJID(ctx, id, "")
	s.setStatus(sessionStatusDisconnected)
	s.setAuth(AuthSnapshot{State: "logged_out", Paired: false})
	m.log.Info("session disconnected", "session", id)
	return nil
}

func (m *SessionManager) Pair(id string) error {
	s, ok := m.Get(id)
	if !ok {
		return fmt.Errorf("no session %s", id)
	}
	if s.client.Store.ID != nil {
		return fmt.Errorf("session already paired")
	}
	s.replaceClient(whatsmeow.NewClient(m.container.NewDevice(), m.waLogger))
	if err := s.startPairing(m.appCtx); err != nil {
		return fmt.Errorf("start pairing: %w", err)
	}
	m.broker.emitSessionList(m.infos())
	m.log.Info("session re-pairing", "session", id)
	return nil
}

func (m *SessionManager) SetWebhook(ctx context.Context, id, url string) error {
	s, ok := m.Get(id)
	if !ok {
		return fmt.Errorf("no session %s", id)
	}
	if err := m.store.setWebhook(ctx, id, url); err != nil {
		return err
	}
	s.mu.Lock()
	s.webhook = url
	s.mu.Unlock()
	m.broker.emitSessionList(m.infos())
	return nil
}

func (m *SessionManager) disconnectAll() {
	m.mu.RLock()
	all := make([]*Session, 0, len(m.sessions))
	for _, s := range m.sessions {
		all = append(all, s)
	}
	m.mu.RUnlock()
	for _, s := range all {
		s.shutdown()
	}
}
