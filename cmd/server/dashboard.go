package main

import (
	"context"
	"net/http"
	"time"
)

type dashboardStats struct {
	TotalSessions     int     `json:"totalSessions"`
	ActiveSessions    int     `json:"activeSessions"`
	TotalUsers        int     `json:"totalUsers"`
	TotalRecordings   int     `json:"totalRecordings"`
	TotalRecSize      int64   `json:"totalRecordingsSize"`
	TotalCallsAll     int     `json:"totalCallsAll"`
	ActiveCalls       int     `json:"activeCalls"`
	InboundCalls      int     `json:"inboundCalls"`
	OutboundCalls     int     `json:"outboundCalls"`
	AvgDuration       float64 `json:"avgDuration"`
	TotalRecordingDur int     `json:"totalRecordingDuration"`
}

type dashboardResponse struct {
	Stats       dashboardStats `json:"stats"`
	RecentCalls []CallRecord   `json:"recentCalls"`
	Sessions    []SessionInfo  `json:"sessions"`
}

func (s *server) handleDashboard(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	scope := s.userClientScope(r)
	stats, err := s.getDashboardStats(ctx, scope)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	recentCalls := s.broker.historyRows("", 20)
	if scope != "" {
		owned := map[string]bool{}
		for _, si := range s.sessions.infos() {
			if si.ClientID == scope {
				owned[si.ID] = true
			}
		}
		filtered := recentCalls[:0]
		for _, c := range recentCalls {
			if owned[c.SessionID] {
				filtered = append(filtered, c)
			}
		}
		recentCalls = filtered
	}
	sessions := s.sessions.infos()
	if scope != "" {
		filtered := sessions[:0]
		for _, si := range sessions {
			if si.ClientID == scope {
				filtered = append(filtered, si)
			}
		}
		sessions = filtered
	}
	writeJSON(w, http.StatusOK, dashboardResponse{
		Stats:       stats,
		RecentCalls: recentCalls,
		Sessions:    sessions,
	})
}

func (s *server) getDashboardStats(ctx context.Context, clientID string) (dashboardStats, error) {
	var st dashboardStats
	db := s.sessions.store.db
	rdb := s.sessions.recStore.db

	if clientID == "" {
		_ = db.QueryRowContext(ctx, `SELECT COUNT(*) FROM sessions`).Scan(&st.TotalSessions)
		_ = db.QueryRowContext(ctx, `SELECT COUNT(*) FROM users`).Scan(&st.TotalUsers)
	} else {
		_ = db.QueryRowContext(ctx, `SELECT COUNT(*) FROM sessions WHERE client_id = $1`, clientID).Scan(&st.TotalSessions)
		_ = rdb.QueryRowContext(ctx,
			`SELECT COUNT(DISTINCT session_id) FROM recordings WHERE client_id = $1`, clientID).Scan(&st.TotalUsers)
	}
	if clientID == "" {
		_ = rdb.QueryRowContext(ctx,
			`SELECT COUNT(*), COALESCE(SUM(file_size),0), COALESCE(SUM(duration),0) FROM recordings`,
		).Scan(&st.TotalRecordings, &st.TotalRecSize, &st.TotalRecordingDur)
	} else {
		_ = rdb.QueryRowContext(ctx,
			`SELECT COUNT(*), COALESCE(SUM(file_size),0), COALESCE(SUM(duration),0) FROM recordings WHERE client_id = $1`, clientID,
		).Scan(&st.TotalRecordings, &st.TotalRecSize, &st.TotalRecordingDur)
	}

	for _, si := range s.sessions.infos() {
		if clientID != "" && si.ClientID != clientID {
			continue
		}
		if si.Paired {
			st.ActiveSessions++
		}
	}

	owned := map[string]bool{}
	if clientID != "" {
		for _, si := range s.sessions.infos() {
			if si.ClientID == clientID {
				owned[si.ID] = true
			}
		}
	}
	s.broker.mu.RLock()
	for _, c := range s.broker.history {
		if clientID != "" && !owned[c.SessionID] {
			continue
		}
		st.TotalCallsAll++
		if c.EndedAt != nil && c.StartedAt > 0 {
			st.AvgDuration += float64(*c.EndedAt-c.StartedAt) / 1000.0
		}
	}
	for _, c := range s.broker.calls {
		if clientID != "" && !owned[c.SessionID] {
			continue
		}
		st.ActiveCalls++
		if c.Direction == "inbound" {
			st.InboundCalls++
		} else {
			st.OutboundCalls++
		}
	}
	s.broker.mu.RUnlock()

	if st.TotalCallsAll > 0 {
		st.AvgDuration /= float64(st.TotalCallsAll)
	}

	_ = ctx
	_ = time.Now()
	return st, nil
}
