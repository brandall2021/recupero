package main

import (
	"context"
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
	waLogger  waLog.Logger
	webhook   *webhookClient
	log       *slog.Logger
	maxCalls  int

	mu       sync.RWMutex
	sessions map[string]*Session
	order    []string
}

func newSessionManager(ctx context.Context, container *sqlstore.Container, broker *Broker, store *sessionStore, recStore *recordingStore, waLogger waLog.Logger, log *slog.Logger, maxCalls int) *SessionManager {
	return &SessionManager{
		appCtx:    ctx,
		container: container,
		broker:    broker,
		store:     store,
		recStore:  recStore,
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
	token := s.token
	s.mu.Unlock()
	if url == "" {
		return
	}
	ev := map[string]any{
		"type":      evtType,
		"channelId": s.id,
		"channel":   s.name,
		"ts":        time.Now().UnixMilli(),
	}
	for k, v := range payload {
		ev[k] = v
	}
	m.webhook.post(m.appCtx, url, token, ev)
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

func (m *SessionManager) Restore(ctx context.Context) error {
	rows, err := m.store.list(ctx)
	if err != nil {
		return err
	}
	for _, row := range rows {
		if row.JID == "" {
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
		token := row.Token
		if token == "" {
			token = newSessionToken()
			if err := m.store.setToken(ctx, row.ID, token); err != nil {
				m.log.Warn("failed to set session token", "session", row.ID, "err", err)
			}
		}
		client := whatsmeow.NewClient(device, m.waLogger)
		s := newSession(m, row.ID, row.Name, token, row.Webhook, client)
		m.register(s)
		if err := s.connect(ctx); err != nil {
			m.log.Error("session connect failed", "session", row.ID, "err", err)
		}
	}
	m.broker.emitSessionList(m.infos())
	m.log.Info("sessions restored", "count", len(m.infos()))
	return nil
}

func (m *SessionManager) Create(name string) (string, string, error) {
	id := newSessionID()
	token := newSessionToken()
	if err := m.store.insert(m.appCtx, id, name, token); err != nil {
		return "", "", err
	}
	device := m.container.NewDevice()
	client := whatsmeow.NewClient(device, m.waLogger)
	s := newSession(m, id, name, token, "", client)
	m.register(s)
	m.broker.emitSessionList(m.infos())
	if err := s.startPairing(m.appCtx); err != nil {
		m.log.Error("start pairing failed", "session", id, "err", err)
		return "", "", fmt.Errorf("start pairing: %w", err)
	}
	m.log.Info("session created", "session", id, "name", name)
	return id, token, nil
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
