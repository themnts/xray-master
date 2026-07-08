package api

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"github.com/thethoughtcriminal/xray-master/internal/config"
	"github.com/thethoughtcriminal/xray-master/internal/service"
)

type Server struct {
	cfg    *config.Config
	master *service.Master
	http   *http.Server
}

func New(cfg *config.Config, master *service.Master) *Server {
	s := &Server{cfg: cfg, master: master}
	r := chi.NewRouter()
	r.Use(middleware.RequestID, middleware.RealIP, middleware.Recoverer, middleware.Logger)

	r.Get("/healthz", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})

	r.Get("/sub/{token}", s.serveSubscription)

	r.Group(func(r chi.Router) {
		r.Use(s.adminAuth)
		r.Get("/nodes", s.listNodes)
		r.Post("/nodes", s.addNode)
		r.Delete("/nodes/{id}", s.deleteNode)
		r.Get("/users", s.listUsers)
		r.Post("/users", s.addUser)
		r.Post("/users/{id}/enable", s.enableUser)
		r.Post("/users/{id}/disable", s.disableUser)
		r.Delete("/users/{id}", s.deleteUser)
		r.Get("/users/{email}/stats", s.userStats)
		r.Post("/sync/users", s.syncUsers)
	})

	s.http = &http.Server{Addr: cfg.Server.Listen, Handler: r}
	return s
}

func (s *Server) ListenAndServe() error {
	return s.http.ListenAndServe()
}

func (s *Server) Shutdown(ctx context.Context) error {
	return s.http.Shutdown(ctx)
}

func (s *Server) adminAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Admin-Key") != s.cfg.Server.AdminKey {
			writeError(w, http.StatusUnauthorized, "invalid admin key")
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (s *Server) serveSubscription(w http.ResponseWriter, r *http.Request) {
	token := chi.URLParam(r, "token")
	result, err := s.master.BuildSubscription(token, r.UserAgent())
	if err != nil {
		writeServiceError(w, err)
		return
	}
	for k, v := range result.Headers {
		w.Header().Set(k, v)
	}
	if result.Format == "happ_json" {
		w.Header().Set("Content-Type", "application/json")
	} else {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	}
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(result.Body)
}

func (s *Server) listNodes(w http.ResponseWriter, r *http.Request) {
	items, err := s.master.ListNodes()
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, items)
}

type addNodeRequest struct {
	Name       string `json:"name"`
	IP         string `json:"ip"`
	APIURL     string `json:"api_url"`
	APIKey     string `json:"api_key"`
	PublicHost string `json:"public_host"`
}

func (s *Server) addNode(w http.ResponseWriter, r *http.Request) {
	var req addNodeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	node, err := s.master.AddNode(service.AddNodeInput{
		Name:       req.Name,
		IP:         req.IP,
		APIURL:     req.APIURL,
		APIKey:     req.APIKey,
		PublicHost: req.PublicHost,
	})
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, node)
}

func (s *Server) deleteNode(w http.ResponseWriter, r *http.Request) {
	if err := s.master.DeleteNode(chi.URLParam(r, "id")); err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

func (s *Server) listUsers(w http.ResponseWriter, r *http.Request) {
	items, err := s.master.ListUsers()
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, items)
}

type addUserRequest struct {
	Email      string `json:"email"`
	UUID       string `json:"uuid"`
	ExpiryTime int64  `json:"expiry_time"`
	TotalBytes int64  `json:"total_bytes"`
	Note       string `json:"note"`
}

func (s *Server) addUser(w http.ResponseWriter, r *http.Request) {
	var req addUserRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	result, err := s.master.AddUser(service.AddUserInput{
		Email:      req.Email,
		UUID:       req.UUID,
		ExpiryTime: req.ExpiryTime,
		TotalBytes: req.TotalBytes,
		Note:       req.Note,
	})
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, result)
}

func (s *Server) enableUser(w http.ResponseWriter, r *http.Request) {
	s.setUserEnabled(w, r, true)
}

func (s *Server) disableUser(w http.ResponseWriter, r *http.Request) {
	s.setUserEnabled(w, r, false)
}

func (s *Server) setUserEnabled(w http.ResponseWriter, r *http.Request, enabled bool) {
	if err := s.master.SetUserEnabled(chi.URLParam(r, "id"), enabled); err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"enabled": enabled})
}

func (s *Server) deleteUser(w http.ResponseWriter, r *http.Request) {
	if err := s.master.DeleteUser(chi.URLParam(r, "id")); err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

func (s *Server) userStats(w http.ResponseWriter, r *http.Request) {
	stats, err := s.master.UserStats(chi.URLParam(r, "email"))
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, stats)
}

func (s *Server) syncUsers(w http.ResponseWriter, r *http.Request) {
	result, err := s.master.SyncAllUsers()
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

func writeServiceError(w http.ResponseWriter, err error) {
	writeError(w, statusFromError(err), err.Error())
}
