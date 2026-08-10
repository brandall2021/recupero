package main

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"
)

type clientJSON struct {
	ID                string `json:"id"`
	Name              string `json:"name"`
	Slug              string `json:"slug"`
	Status            string `json:"status"`
	MaxSessions       int    `json:"maxSessions"`
	UsedSessions      int    `json:"usedSessions"`
	AvailableSessions int    `json:"availableSessions"`
	CreatedAt         string `json:"createdAt"`
	UpdatedAt         string `json:"updatedAt"`
}

func toClientJSON(c *clientRow) clientJSON {
	avail := c.MaxSessions - c.UsedSessions
	if avail < 0 {
		avail = 0
	}
	return clientJSON{
		ID:                c.ID,
		Name:              c.Name,
		Slug:              c.Slug,
		Status:            c.Status,
		MaxSessions:       c.MaxSessions,
		UsedSessions:      c.UsedSessions,
		AvailableSessions: avail,
		CreatedAt:         c.CreatedAt.Format("2006-01-02T15:04:05Z"),
		UpdatedAt:         c.UpdatedAt.Format("2006-01-02T15:04:05Z"),
	}
}

func (s *server) handlePlatformClientList(w http.ResponseWriter, r *http.Request) {
	clients, err := s.clients.list(r.Context())
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	out := make([]clientJSON, len(clients))
	for i, c := range clients {
		out[i] = toClientJSON(&c)
	}
	writeJSON(w, http.StatusOK, map[string]any{"clients": out})
}

func (s *server) handlePlatformClientGet(w http.ResponseWriter, r *http.Request) {
	cl, err := s.clients.get(r.Context(), r.PathValue("clientId"))
	if err != nil {
		writeJSON(w, clientNotFoundStatus(err), map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"client": toClientJSON(cl)})
}

func (s *server) handlePlatformClientCreate(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Name        string `json:"name"`
		Slug        string `json:"slug"`
		MaxSessions int    `json:"maxSessions"`
		Admin       struct {
			Name     string `json:"name"`
			Email    string `json:"email"`
			Password string `json:"password"`
		} `json:"admin"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid body"})
		return
	}
	name := strings.TrimSpace(body.Name)
	if name == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "name required"})
		return
	}
	email := strings.TrimSpace(body.Admin.Email)
	password := strings.TrimSpace(body.Admin.Password)
	if email == "" || password == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "admin email and password required"})
		return
	}
	maxSessions := body.MaxSessions
	if maxSessions < 0 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "maxSessions must be >= 0"})
		return
	}
	slug := strings.TrimSpace(body.Slug)
	if slug == "" {
		slug = slugify(name)
	}

	id, err := s.clients.createWithAdmin(r.Context(), name, slug, maxSessions, email, body.Admin.Name, password)
	if err != nil {
		code := http.StatusInternalServerError
		switch {
		case errors.Is(err, ErrSlugTaken):
			code = http.StatusConflict
		case errors.Is(err, ErrClientEmailTaken):
			code = http.StatusConflict
		}
		writeJSON(w, code, map[string]string{"error": err.Error()})
		return
	}
	cl, err := s.clients.get(r.Context(), id)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{
		"client": toClientJSON(cl),
		"admin": map[string]any{
			"email": strings.ToLower(strings.TrimSpace(email)),
			"name":  body.Admin.Name,
			"role":  roleClientAdmin,
		},
	})
}

func (s *server) handlePlatformClientUpdate(w http.ResponseWriter, r *http.Request) {
	clientID := r.PathValue("clientId")
	var body struct {
		Name        string `json:"name"`
		Slug        string `json:"slug"`
		MaxSessions *int   `json:"maxSessions"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid body"})
		return
	}
	cl, err := s.clients.get(r.Context(), clientID)
	if err != nil {
		writeJSON(w, clientNotFoundStatus(err), map[string]string{"error": err.Error()})
		return
	}
	name := strings.TrimSpace(body.Name)
	if name == "" {
		name = cl.Name
	}
	maxSessions := cl.MaxSessions
	if body.MaxSessions != nil {
		maxSessions = *body.MaxSessions
		if maxSessions < 0 {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "maxSessions must be >= 0"})
			return
		}
	}
	if err := s.clients.update(r.Context(), clientID, name, maxSessions); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	cl, err = s.clients.get(r.Context(), clientID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"client": toClientJSON(cl)})
}

func (s *server) handlePlatformClientDelete(w http.ResponseWriter, r *http.Request) {
	clientID := r.PathValue("clientId")
	for _, si := range s.sessions.infos() {
		if si.ClientID == clientID {
			_ = s.sessions.Delete(r.Context(), si.ID)
		}
	}
	if err := s.clients.delete(r.Context(), clientID); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *server) handlePlatformClientStatus(w http.ResponseWriter, r *http.Request) {
	clientID := r.PathValue("clientId")
	var body struct {
		Status string `json:"status"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid body"})
		return
	}
	switch body.Status {
	case "active", "suspended", "disabled":
	default:
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "status must be active, suspended or disabled"})
		return
	}
	if err := s.clients.setStatus(r.Context(), clientID, body.Status); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	cl, err := s.clients.get(r.Context(), clientID)
	if err != nil {
		writeJSON(w, clientNotFoundStatus(err), map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"client": toClientJSON(cl)})
}

func (s *server) handlePlatformClientLimits(w http.ResponseWriter, r *http.Request) {
	clientID := r.PathValue("clientId")
	var body struct {
		MaxSessions int `json:"maxSessions"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid body"})
		return
	}
	if body.MaxSessions < 0 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "maxSessions must be >= 0"})
		return
	}
	if err := s.clients.updateLimitsGuarded(r.Context(), clientID, body.MaxSessions); err != nil {
		if errors.Is(err, ErrLimitBelowUsage) {
			writeJSON(w, http.StatusUnprocessableEntity, map[string]string{
				"error": "limit_below_current_usage",
			})
			return
		}
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	cl, err := s.clients.get(r.Context(), clientID)
	if err != nil {
		writeJSON(w, clientNotFoundStatus(err), map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"client": toClientJSON(cl)})
}

func clientNotFoundStatus(err error) int {
	if errors.Is(err, ErrClientNotFound) {
		return http.StatusNotFound
	}
	return http.StatusInternalServerError
}
