package httpserver

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"math/big"
	"net"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/linkasu/linka.looks-metric/internal/config"
	"github.com/linkasu/linka.looks-metric/internal/db"
	"github.com/linkasu/linka.looks-metric/internal/events"
	"github.com/linkasu/linka.looks-metric/internal/mail"
	"github.com/linkasu/linka.looks-metric/internal/pages"
	"github.com/linkasu/linka.looks-metric/internal/ratelimit"
)

const (
	jsonContentType         = "application/json; charset=utf-8"
	maxPublicBody           = 2048
	maxEventBody            = 16 * 1024
	currentConsentPolicy    = "technical-events"
	currentConsentPolicyVer = 1
)

var (
	emailRegex   = regexp.MustCompile(`^[^\s@]+@[^\s@]+\.[^\s@]+$`)
	versionRegex = regexp.MustCompile(`looks/(\S+)`)
)

type Server struct {
	cfg     config.Config
	store   *db.Store
	mailer  mail.Mailer
	logger  *slog.Logger
	pages   *pages.Renderer
	limiter *ratelimit.Limiter
	started time.Time
}

type eventConsentProof struct {
	Policy  string `json:"policy"`
	Version int    `json:"version"`
	Granted bool   `json:"granted"`
}

func New(cfg config.Config, store *db.Store, mailer mail.Mailer, logger *slog.Logger) http.Handler {
	renderer, err := pages.New()
	if err != nil {
		panic(err)
	}
	server := &Server{
		cfg:     cfg,
		store:   store,
		mailer:  mailer,
		logger:  logger,
		pages:   renderer,
		limiter: ratelimit.New(),
		started: time.Now(),
	}
	go server.sweepLimits()
	return server.routes()
}

func (s *Server) routes() http.Handler {
	r := chi.NewRouter()
	r.Use(s.securityHeaders)
	r.Use(s.requestLogger)
	r.Options("/*", s.handleOptions)
	r.Get("/healthz", s.health)
	r.Get("/", s.index)
	r.Get("/privacy", s.privacy)
	r.Get("/stats", s.stats)
	r.Handle("/assets/*", http.StripPrefix("/assets/", http.FileServer(pages.Static())))
	r.Post("/requestActivation", s.requestActivation)
	r.Post("/activate", s.activate)
	r.Post("/registerEvent", s.registerEvent)
	r.NotFound(func(w http.ResponseWriter, r *http.Request) {
		s.pages.Render(w, "error.html", map[string]any{"Title": "Not found", "Status": http.StatusNotFound})
	})
	return r
}

func (s *Server) health(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()
	if err := s.store.Ping(ctx); err != nil {
		s.writeJSON(w, http.StatusServiceUnavailable, map[string]string{"status": "error"})
		return
	}
	s.writeJSON(w, http.StatusOK, map[string]any{"status": "ok", "uptimeSeconds": int(time.Since(s.started).Seconds())})
}

func (s *Server) index(w http.ResponseWriter, r *http.Request) {
	s.pages.Render(w, "index.html", map[string]any{"Title": "LINKa metric", "PublicBaseURL": s.cfg.PublicBaseURL})
}

func (s *Server) privacy(w http.ResponseWriter, r *http.Request) {
	s.pages.Render(w, "privacy.html", map[string]any{"Title": "Privacy"})
}

func (s *Server) stats(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()
	stats, err := s.store.LoadStats(ctx)
	if err != nil {
		s.logger.Error("load stats", "error", err)
		http.Error(w, "stats error", http.StatusInternalServerError)
		return
	}
	s.pages.Render(w, "stats.html", map[string]any{"Title": "Stats", "Stats": stats})
}

func (s *Server) requestActivation(w http.ResponseWriter, r *http.Request) {
	if !s.checkLimit(w, "activation-ip:"+clientIP(r), 20, time.Hour) {
		return
	}
	var body struct {
		Email string `json:"email"`
	}
	if !s.readJSON(w, r, maxPublicBody, &body) {
		return
	}
	email := normalizeEmail(body.Email)
	if !emailRegex.MatchString(email) {
		s.writeJSON(w, http.StatusBadRequest, map[string]string{"error": "Invalid email format."})
		return
	}
	if !s.checkLimit(w, "activation-email:"+email, 3, time.Hour) {
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 25*time.Second)
	defer cancel()
	user, err := s.store.FindOrCreateUser(ctx, email)
	if err != nil {
		s.logger.Error("find or create user", "error", err)
		s.badRequest(w)
		return
	}
	code, err := generateCode()
	if err != nil {
		s.logger.Error("generate code", "error", err)
		s.badRequest(w)
		return
	}
	if err := s.store.SaveActivationCode(ctx, user.ID, code); err != nil {
		s.logger.Error("save activation code", "error", err)
		s.badRequest(w)
		return
	}
	if err := s.mailer.SendActivationCode(ctx, email, code); err != nil {
		s.logger.Error("send activation mail", "error", err)
		s.badRequest(w)
		return
	}
	s.writeJSON(w, http.StatusOK, map[string]string{"message": "Activation email sent successfully."})
}

func (s *Server) activate(w http.ResponseWriter, r *http.Request) {
	if !s.checkLimit(w, "activate-ip:"+clientIP(r), 10, time.Hour) {
		return
	}
	var body struct {
		Email string `json:"email"`
		Code  string `json:"code"`
	}
	if !s.readJSON(w, r, maxPublicBody, &body) {
		return
	}
	email := normalizeEmail(body.Email)
	code := strings.TrimSpace(body.Code)
	if !emailRegex.MatchString(email) || len(code) != 6 {
		s.badRequest(w)
		return
	}
	if !s.checkLimit(w, "activate-email:"+email, 5, time.Hour) {
		return
	}
	hash := uuid.NewString()
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()
	if err := s.store.Activate(ctx, email, code, hash); err != nil {
		s.badRequest(w)
		return
	}
	s.writeJSON(w, http.StatusOK, map[string]string{"hash": hash, "message": "Account activated successfully."})
}

func (s *Server) registerEvent(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Hash            string             `json:"hash"`
		EventName       string             `json:"eventName"`
		LegacyEventData json.RawMessage    `json:"eventData"`
		Consent         *eventConsentProof `json:"consent"`
	}
	if !s.readJSON(w, r, maxEventBody, &body) {
		return
	}
	if body.Consent == nil || body.Consent.Policy != currentConsentPolicy || body.Consent.Version != currentConsentPolicyVer || !body.Consent.Granted {
		s.writeJSON(w, http.StatusOK, map[string]string{"message": "Event ignored without current consent."})
		return
	}
	hash := strings.TrimSpace(body.Hash)
	if _, err := uuid.Parse(hash); err != nil {
		s.badRequest(w)
		return
	}
	if !s.checkLimit(w, "event-hash:"+hash, 120, time.Minute) || !s.checkLimit(w, "event-ip:"+clientIP(r), 300, time.Minute) {
		return
	}
	eventName := strings.TrimSpace(body.EventName)
	if !events.Allowed(eventName) {
		s.badRequest(w)
		return
	}
	version := parseVersion(r.UserAgent())
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()
	if err := s.store.RegisterEvent(ctx, hash, eventName, version); err != nil {
		s.badRequest(w)
		return
	}
	s.writeJSON(w, http.StatusOK, map[string]string{"message": "Event registered successfully."})
}

func (s *Server) readJSON(w http.ResponseWriter, r *http.Request, limit int64, dst any) bool {
	defer r.Body.Close()
	decoder := json.NewDecoder(io.LimitReader(r.Body, limit+1))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(dst); err != nil {
		s.badRequest(w)
		return false
	}
	var trailing json.RawMessage
	if err := decoder.Decode(&trailing); err != io.EOF {
		s.badRequest(w)
		return false
	}
	return true
}

func (s *Server) writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", jsonContentType)
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(value)
}

func (s *Server) badRequest(w http.ResponseWriter) {
	s.writeJSON(w, http.StatusBadRequest, map[string]string{"error": "Bad request"})
}

func (s *Server) checkLimit(w http.ResponseWriter, key string, limit int, window time.Duration) bool {
	if s.limiter.Allow(key, limit, window) {
		return true
	}
	s.writeJSON(w, http.StatusTooManyRequests, map[string]string{"error": "Too many requests"})
	return false
}

func (s *Server) handleOptions(w http.ResponseWriter, r *http.Request) {
	setCORS(w)
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		setCORS(w)
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		next.ServeHTTP(w, r)
	})
}

func (s *Server) requestLogger(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		s.logger.Info("request", "method", r.Method, "path", r.URL.Path, "durationMs", time.Since(start).Milliseconds())
	})
}

func (s *Server) sweepLimits() {
	ticker := time.NewTicker(10 * time.Minute)
	defer ticker.Stop()
	for range ticker.C {
		s.limiter.Sweep()
	}
}

func setCORS(w http.ResponseWriter) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type, User-Agent")
}

func normalizeEmail(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}

func parseVersion(userAgent string) string {
	match := versionRegex.FindStringSubmatch(userAgent)
	if len(match) != 2 {
		return ""
	}
	version := match[1]
	if len(version) > 32 {
		return version[:32]
	}
	return version
}

func clientIP(r *http.Request) string {
	forwarded := strings.TrimSpace(strings.Split(r.Header.Get("X-Forwarded-For"), ",")[0])
	if forwarded != "" {
		return forwarded
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

func generateCode() (string, error) {
	var result strings.Builder
	for i := 0; i < 6; i++ {
		value, err := rand.Int(rand.Reader, big.NewInt(10))
		if err != nil {
			return "", err
		}
		result.WriteString(fmt.Sprint(value.Int64()))
	}
	return result.String(), nil
}
