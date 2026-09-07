package control

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

const APIVersion = "v1"

type Session struct {
	Token            string
	CSRF             string
	UserID           string
	SessionVersion   uint64
	ExpiresAt        time.Time
	PendingMFASecret string
}
type ServerOptions struct {
	CookieSecure       bool
	SessionTTL         time.Duration
	ProfileCatalogPath string
	BindingRoot        string
	MonitorBaseURL     string
	Version            string
}
type Server struct {
	store    *Store
	opt      ServerOptions
	client   *http.Client
	mu       sync.Mutex
	sessions map[string]Session
	handler  http.Handler
}

func NewServer(store *Store, opt ServerOptions) (*Server, error) {
	if store == nil {
		return nil, errors.New("store is required")
	}
	if opt.SessionTTL <= 0 {
		opt.SessionTTL = 8 * time.Hour
	}
	if opt.ProfileCatalogPath == "" {
		opt.ProfileCatalogPath = "/opt/rc-gateway/current/controllers/DRAFT_PROFILES.json"
	}
	if opt.BindingRoot == "" {
		opt.BindingRoot = "/opt/rc-gateway/current/controllers"
	}
	if opt.MonitorBaseURL == "" {
		opt.MonitorBaseURL = "http://127.0.0.1:18100"
	}
	if err := validateMonitorBaseURL(opt.MonitorBaseURL); err != nil {
		return nil, fmt.Errorf("invalid monitor base URL: %w", err)
	}
	s := &Server{store: store, opt: opt, client: &http.Client{Timeout: 12 * time.Second}, sessions: map[string]Session{}}
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", s.health)
	mux.HandleFunc("/readyz", s.health)
	mux.HandleFunc("/api/v1/auth/login", s.login)
	mux.HandleFunc("/api/v1/auth/logout", s.auth(PermFleetRead, s.logout))
	mux.HandleFunc("/api/v1/auth/logout-all", s.auth(PermFleetRead, s.logoutAll))
	mux.HandleFunc("/api/v1/auth/me", s.auth(PermFleetRead, s.me))
	mux.HandleFunc("/api/v1/auth/change-password", s.auth(PermFleetRead, s.changePassword))
	mux.HandleFunc("/api/v1/auth/mfa/setup", s.auth(PermFleetRead, s.mfaSetup))
	mux.HandleFunc("/api/v1/auth/mfa/enable", s.auth(PermFleetRead, s.mfaEnable))
	mux.HandleFunc("/api/v1/auth/preferences", s.auth(PermFleetRead, s.preferences))
	mux.HandleFunc("/api/v1/system/health", s.auth(PermFleetRead, s.operationProxy))
	mux.HandleFunc("/api/v1/diagnostics/system", s.auth(PermDiagnosticsRead, s.diagnosticsSystem))
	mux.HandleFunc("/api/v1/generators", s.auth(PermFleetRead, s.operationProxy))
	mux.HandleFunc("/api/v1/generators/", s.auth(PermFleetRead, s.operationProxy))
	mux.HandleFunc("/api/v1/admin/roles", s.auth(PermUsersRead, s.roles))
	mux.HandleFunc("/api/v1/admin/users", s.auth(PermUsersRead, s.users))
	mux.HandleFunc("/api/v1/admin/users/", s.auth(PermUsersWrite, s.userResource))
	mux.HandleFunc("/api/v1/admin/sites", s.auth(PermSitesRead, s.sites))
	mux.HandleFunc("/api/v1/admin/sites/", s.auth(PermSitesWrite, s.siteResource))
	mux.HandleFunc("/api/v1/admin/audit", s.auth(PermAuditRead, s.audit))
	mux.HandleFunc("/api/v1/admin/sessions", s.auth(PermUsersRead, s.sessionList))
	mux.HandleFunc("/api/v1/admin/system", s.auth(PermSystemRead, s.systemInfo))
	mux.HandleFunc("/api/v1/admin/settings", s.auth(PermSettingsRead, s.settings))
	mux.HandleFunc("/api/v1/engineering/monitor-generators", s.auth(PermCommissioningRead, s.engineeringMonitorGenerators))
	mux.HandleFunc("/api/v1/engineering/profiles", s.auth(PermCommissioningRead, s.profiles))
	mux.HandleFunc("/api/v1/engineering/bindings", s.auth(PermCommissioningRead, s.bindings))
	mux.HandleFunc("/api/v1/engineering/profile-states", s.auth(PermCommissioningRead, s.profileStates))
	mux.HandleFunc("/api/v1/engineering/profile-states/", s.auth(PermCommissioningRead, s.profileStateResource))
	mux.HandleFunc("/api/v1/engineering/commissionings", s.auth(PermCommissioningRead, s.commissionings))
	mux.HandleFunc("/api/v1/engineering/commissionings/", s.auth(PermCommissioningRead, s.commissioningResource))
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) { writeError(w, 404, "not_found", "resource not found") })
	s.handler = securityHeaders(mux)
	return s, nil
}
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) { s.handler.ServeHTTP(w, r) }
func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		next.ServeHTTP(w, r)
	})
}

func validateMonitorBaseURL(raw string) error {
	u, err := url.Parse(raw)
	if err != nil {
		return err
	}
	if u.Scheme != "http" {
		return errors.New("monitor URL must use http over loopback")
	}
	if u.Path != "" && u.Path != "/" {
		return errors.New("monitor URL must not contain a path")
	}
	if u.RawQuery != "" || u.Fragment != "" || u.User != nil {
		return errors.New("monitor URL must not contain credentials, query or fragment")
	}
	host := u.Hostname()
	if strings.EqualFold(host, "localhost") {
		return nil
	}
	ip := net.ParseIP(host)
	if ip == nil || !ip.IsLoopback() {
		return errors.New("monitor host must be loopback")
	}
	return nil
}

type monitorGeneratorMeta struct {
	ID     string `json:"id"`
	SiteID string `json:"siteId"`
}

func (s *Server) monitorGET(ctx context.Context, path string) (int, []byte, string, error) {
	base := strings.TrimRight(s.opt.MonitorBaseURL, "/")
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, base+path, nil)
	if err != nil {
		return 0, nil, "", err
	}
	req.Header.Set("Accept", "application/json")
	resp, err := s.client.Do(req)
	if err != nil {
		return 0, nil, "", err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return 0, nil, "", err
	}
	return resp.StatusCode, body, resp.Header.Get("Content-Type"), nil
}

func (s *Server) operationProxy(w http.ResponseWriter, r *http.Request, u User, sess Session) {
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", http.MethodGet)
		writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "only GET is allowed")
		return
	}
	path := r.URL.Path
	if path == "/api/v1/system/health" {
		s.forwardMonitorJSON(w, r, path)
		return
	}
	if path == "/api/v1/generators" {
		status, body, _, err := s.monitorGET(r.Context(), path)
		if err != nil {
			writeError(w, http.StatusBadGateway, "monitor_unavailable", "RC Monitor unavailable")
			return
		}
		if status != http.StatusOK {
			writeRawJSON(w, status, body)
			return
		}
		var items []json.RawMessage
		if err := json.Unmarshal(body, &items); err != nil {
			writeError(w, http.StatusBadGateway, "monitor_invalid_response", "RC Monitor returned invalid generator list")
			return
		}
		eligible := s.store.OperationalGeneratorIDs(u)
		filtered := make([]json.RawMessage, 0, len(items))
		for _, raw := range items {
			var meta monitorGeneratorMeta
			if json.Unmarshal(raw, &meta) != nil || meta.ID == "" {
				continue
			}
			_, commissioned := eligible[meta.ID]
			if allowedSite(u, meta.SiteID) && commissioned {
				filtered = append(filtered, raw)
			}
		}
		writeJSON(w, http.StatusOK, filtered)
		return
	}
	if strings.HasPrefix(path, "/api/v1/generators/") {
		remainder := strings.Trim(strings.TrimPrefix(path, "/api/v1/generators/"), "/")
		parts := strings.Split(remainder, "/")
		if len(parts) < 1 || len(parts) > 2 || strings.TrimSpace(parts[0]) == "" {
			writeError(w, 404, "not_found", "resource not found")
			return
		}
		generatorPath := "/api/v1/generators/" + url.PathEscape(parts[0])
		status, body, _, err := s.monitorGET(r.Context(), generatorPath)
		if err != nil {
			writeError(w, http.StatusBadGateway, "monitor_unavailable", "RC Monitor unavailable")
			return
		}
		if status != http.StatusOK {
			writeRawJSON(w, status, body)
			return
		}
		var meta monitorGeneratorMeta
		if json.Unmarshal(body, &meta) != nil || meta.ID == "" {
			writeError(w, http.StatusBadGateway, "monitor_invalid_response", "RC Monitor returned invalid generator metadata")
			return
		}
		_, commissioned := s.store.OperationalGeneratorIDs(u)[meta.ID]
		if !allowedSite(u, meta.SiteID) || !commissioned {
			writeError(w, http.StatusNotFound, "generator_not_found", "generator not found")
			return
		}
		s.forwardMonitorJSON(w, r, path)
		return
	}
	writeError(w, http.StatusNotFound, "not_found", "resource not found")
}

func (s *Server) forwardMonitorJSON(w http.ResponseWriter, r *http.Request, path string) {
	status, body, contentType, err := s.monitorGET(r.Context(), path)
	if err != nil {
		writeError(w, http.StatusBadGateway, "monitor_unavailable", "RC Monitor unavailable")
		return
	}
	if contentType != "" {
		w.Header().Set("Content-Type", contentType)
	}
	writeRawJSON(w, status, body)
}

func writeRawJSON(w http.ResponseWriter, status int, body []byte) {
	if w.Header().Get("Content-Type") == "" {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
	}
	w.WriteHeader(status)
	_, _ = w.Write(body)
}

func (s *Server) health(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, 405, "method_not_allowed", "only GET is allowed")
		return
	}
	writeJSON(w, 200, map[string]any{"status": "ready", "apiVersion": APIVersion})
}

func token(prefix string) string {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		panic(err)
	}
	return prefix + hex.EncodeToString(b)
}
func (s *Server) login(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, 405, "method_not_allowed", "only POST is allowed")
		return
	}
	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
		TOTP     string `json:"totp,omitempty"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	u, err := s.store.Authenticate(req.Username, req.Password)
	if err != nil {
		writeError(w, 401, "invalid_credentials", "invalid username or password")
		return
	}
	if u.MFAEnabled && !ValidateTOTP(u.MFASecret, req.TOTP, time.Now().UTC()) {
		s.store.RecordLoginFailure(u, "mfa_required")
		writeError(w, http.StatusUnauthorized, "mfa_required", "valid TOTP code required")
		return
	}
	u, err = s.store.RecordLoginSuccess(u.ID)
	if err != nil {
		writeError(w, 500, "login_state_failed", "unable to persist login state")
		return
	}
	sess := Session{Token: token("s_"), CSRF: token("c_"), UserID: u.ID, SessionVersion: u.SessionVersion, ExpiresAt: time.Now().UTC().Add(s.opt.SessionTTL)}
	s.mu.Lock()
	s.sessions[sess.Token] = sess
	s.mu.Unlock()
	http.SetCookie(w, &http.Cookie{Name: "rc_session", Value: sess.Token, Path: "/", HttpOnly: true, Secure: s.opt.CookieSecure, SameSite: http.SameSiteStrictMode, MaxAge: int(s.opt.SessionTTL.Seconds())})
	writeJSON(w, 200, map[string]any{"user": u.Public(), "csrfToken": sess.CSRF, "expiresAt": sess.ExpiresAt, "mfaEnrollmentRequired": u.MFARequired && !u.MFAEnabled})
}
func (s *Server) current(r *http.Request) (User, Session, bool) {
	c, err := r.Cookie("rc_session")
	if err != nil {
		return User{}, Session{}, false
	}
	s.mu.Lock()
	sess, ok := s.sessions[c.Value]
	if ok && time.Now().UTC().After(sess.ExpiresAt) {
		delete(s.sessions, c.Value)
		ok = false
	}
	s.mu.Unlock()
	if !ok {
		return User{}, Session{}, false
	}
	u, ok := s.store.UserByID(sess.UserID)
	if !ok || !u.Active || u.SessionVersion != sess.SessionVersion {
		return User{}, Session{}, false
	}
	return u, sess, true
}
func (s *Server) auth(permission Permission, next func(http.ResponseWriter, *http.Request, User, Session)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		u, sess, ok := s.current(r)
		if !ok {
			writeError(w, 401, "unauthorized", "authentication required")
			return
		}
		if !HasPermission(u.Role, permission) {
			writeError(w, 403, "forbidden", "permission denied")
			return
		}
		securityPending := u.MustChangePassword || (u.MFARequired && !u.MFAEnabled)
		if securityPending && !strings.HasPrefix(r.URL.Path, "/api/v1/auth/") {
			writeError(w, 403, "security_setup_required", "password/MFA enrollment must be completed before accessing this resource")
			return
		}
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			provided := r.Header.Get("X-CSRF-Token")
			if provided == "" || subtle.ConstantTimeCompare([]byte(provided), []byte(sess.CSRF)) != 1 {
				writeError(w, 403, "csrf_failed", "CSRF token missing or invalid")
				return
			}
		}
		next(w, r, u, sess)
	}
}
func (s *Server) logout(w http.ResponseWriter, r *http.Request, u User, sess Session) {
	if r.Method != http.MethodPost {
		writeError(w, 405, "method_not_allowed", "only POST is allowed")
		return
	}
	s.mu.Lock()
	delete(s.sessions, sess.Token)
	s.mu.Unlock()
	http.SetCookie(w, &http.Cookie{Name: "rc_session", Value: "", Path: "/", HttpOnly: true, Secure: s.opt.CookieSecure, SameSite: http.SameSiteStrictMode, MaxAge: -1})
	writeJSON(w, 200, map[string]string{"status": "logged_out"})
}
func (s *Server) logoutAll(w http.ResponseWriter, r *http.Request, u User, sess Session) {
	if r.Method != http.MethodPost {
		writeError(w, 405, "method_not_allowed", "only POST is allowed")
		return
	}
	if err := s.store.RevokeOwnSessions(u); err != nil {
		writeError(w, 500, "session_revoke_failed", "unable to revoke sessions")
		return
	}
	s.mu.Lock()
	for token, candidate := range s.sessions {
		if candidate.UserID == u.ID {
			delete(s.sessions, token)
		}
	}
	s.mu.Unlock()
	http.SetCookie(w, &http.Cookie{Name: "rc_session", Value: "", Path: "/", HttpOnly: true, Secure: s.opt.CookieSecure, SameSite: http.SameSiteStrictMode, MaxAge: -1})
	writeJSON(w, 200, map[string]string{"status": "all_sessions_revoked"})
}

func (s *Server) me(w http.ResponseWriter, r *http.Request, u User, sess Session) {
	if r.Method != http.MethodGet {
		writeError(w, 405, "method_not_allowed", "only GET is allowed")
		return
	}
	writeJSON(w, 200, map[string]any{"user": u.Public(), "csrfToken": sess.CSRF, "expiresAt": sess.ExpiresAt, "mfaEnrollmentRequired": u.MFARequired && !u.MFAEnabled})
}
func (s *Server) changePassword(w http.ResponseWriter, r *http.Request, u User, sess Session) {
	if r.Method != http.MethodPost {
		writeError(w, 405, "method_not_allowed", "only POST is allowed")
		return
	}
	var req struct {
		CurrentPassword string `json:"currentPassword"`
		NewPassword     string `json:"newPassword"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	if err := s.store.ChangePassword(u, req.CurrentPassword, req.NewPassword); err != nil {
		writeError(w, 400, "password_change_failed", err.Error())
		return
	}
	s.mu.Lock()
	delete(s.sessions, sess.Token)
	s.mu.Unlock()
	http.SetCookie(w, &http.Cookie{Name: "rc_session", Value: "", Path: "/", MaxAge: -1, HttpOnly: true, Secure: s.opt.CookieSecure, SameSite: http.SameSiteStrictMode})
	writeJSON(w, 200, map[string]string{"status": "password_changed_relogin_required"})
}
func (s *Server) mfaSetup(w http.ResponseWriter, r *http.Request, u User, sess Session) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "only POST is allowed")
		return
	}
	secret, err := NewTOTPSecret()
	if err != nil {
		writeError(w, 500, "mfa_setup_failed", "unable to generate MFA secret")
		return
	}
	s.mu.Lock()
	current := s.sessions[sess.Token]
	current.PendingMFASecret = secret
	s.sessions[sess.Token] = current
	s.mu.Unlock()
	writeJSON(w, 200, map[string]string{"secret": secret, "otpauthUri": TOTPURI("RC Monitor", u.Username, secret)})
}
func (s *Server) mfaEnable(w http.ResponseWriter, r *http.Request, u User, sess Session) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "only POST is allowed")
		return
	}
	var req struct {
		Code string `json:"code"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	s.mu.Lock()
	current := s.sessions[sess.Token]
	secret := current.PendingMFASecret
	s.mu.Unlock()
	if secret == "" || !ValidateTOTP(secret, req.Code, time.Now().UTC()) {
		writeError(w, 400, "mfa_invalid_code", "invalid TOTP code")
		return
	}
	out, err := s.store.EnableMFA(u, secret)
	if err != nil {
		writeError(w, 400, "mfa_enable_failed", err.Error())
		return
	}
	s.mu.Lock()
	current.PendingMFASecret = ""
	s.sessions[sess.Token] = current
	s.mu.Unlock()
	writeJSON(w, 200, out)
}

func (s *Server) preferences(w http.ResponseWriter, r *http.Request, u User, sess Session) {
	switch r.Method {
	case http.MethodGet:
		writeJSON(w, 200, s.store.Preferences(u))
	case http.MethodPatch:
		var in UserPreferences
		if !decodeJSON(w, r, &in) {
			return
		}
		out, err := s.store.UpdatePreferences(u, in)
		if err != nil {
			writeError(w, 400, "preferences_update_failed", err.Error())
			return
		}
		writeJSON(w, 200, out)
	default:
		writeError(w, 405, "method_not_allowed", "GET or PATCH required")
	}
}

func (s *Server) roles(w http.ResponseWriter, r *http.Request, u User, sess Session) {
	if r.Method != http.MethodGet {
		writeError(w, 405, "method_not_allowed", "only GET is allowed")
		return
	}
	roles := []map[string]any{}
	for _, role := range []Role{RoleViewer, RoleOperator, RoleTechnician, RoleCommissioningEngineer, RoleAdministrator, RoleAuditor} {
		roles = append(roles, map[string]any{"id": role, "permissions": PermissionsForRole(role)})
	}
	writeJSON(w, 200, roles)
}

func (s *Server) users(w http.ResponseWriter, r *http.Request, u User, sess Session) {
	switch r.Method {
	case http.MethodGet:
		writeJSON(w, 200, s.store.ListUsers())
	case http.MethodPost:
		if !HasPermission(u.Role, PermUsersWrite) {
			writeError(w, 403, "forbidden", "permission denied")
			return
		}
		var in CreateUserInput
		if !decodeJSON(w, r, &in) {
			return
		}
		created, pwd, err := s.store.CreateUser(u, in)
		if err != nil {
			writeError(w, 400, "user_create_failed", err.Error())
			return
		}
		writeJSON(w, 201, map[string]any{"user": created, "temporaryPassword": pwd})
	default:
		writeError(w, 405, "method_not_allowed", "GET or POST required")
	}
}
func (s *Server) userResource(w http.ResponseWriter, r *http.Request, u User, sess Session) {
	parts := splitAfter(r.URL.Path, "/api/v1/admin/users/")
	if len(parts) < 1 {
		writeError(w, 404, "not_found", "resource not found")
		return
	}
	id := parts[0]
	if len(parts) == 1 && r.Method == http.MethodGet {
		out, ok := s.store.CommissioningByID(u, id)
		if !ok {
			writeError(w, 404, "not_found", "commissioning not found")
			return
		}
		writeJSON(w, 200, out)
		return
	}
	if len(parts) == 1 && r.Method == http.MethodPatch {
		var in UpdateUserInput
		if !decodeJSON(w, r, &in) {
			return
		}
		out, err := s.store.UpdateUser(u, id, in)
		if err != nil {
			writeError(w, 400, "user_update_failed", err.Error())
			return
		}
		writeJSON(w, 200, out)
		return
	}
	if len(parts) == 2 && parts[1] == "reset-password" && r.Method == http.MethodPost {
		pwd, err := s.store.ResetPassword(u, id)
		if err != nil {
			writeError(w, 400, "password_reset_failed", err.Error())
			return
		}
		writeJSON(w, 200, map[string]string{"temporaryPassword": pwd})
		return
	}
	if len(parts) == 2 && parts[1] == "reset-mfa" && r.Method == http.MethodPost {
		out, err := s.store.ResetMFA(u, id)
		if err != nil {
			writeError(w, 400, "mfa_reset_failed", err.Error())
			return
		}
		writeJSON(w, 200, out)
		return
	}
	if len(parts) == 2 && parts[1] == "disable-mfa" && r.Method == http.MethodPost {
		out, err := s.store.DisableMFA(u, id)
		if err != nil {
			writeError(w, 400, "mfa_disable_failed", err.Error())
			return
		}
		writeJSON(w, 200, out)
		return
	}
	if len(parts) == 2 && parts[1] == "revoke-sessions" && r.Method == http.MethodPost {
		if err := s.store.RevokeSessions(u, id); err != nil {
			writeError(w, 400, "session_revoke_failed", err.Error())
			return
		}
		writeJSON(w, 200, map[string]string{"status": "revoked"})
		return
	}
	writeError(w, 405, "method_not_allowed", "unsupported user operation")
}
func (s *Server) sites(w http.ResponseWriter, r *http.Request, u User, sess Session) {
	switch r.Method {
	case http.MethodGet:
		sites := s.store.ListSites()
		filtered := []Site{}
		for _, site := range sites {
			if allowedSite(u, site.ID) || allowedSite(u, site.Code) {
				filtered = append(filtered, site)
			}
		}
		writeJSON(w, 200, filtered)
	case http.MethodPost:
		if !HasPermission(u.Role, PermSitesWrite) {
			writeError(w, 403, "forbidden", "permission denied")
			return
		}
		var in SiteInput
		if !decodeJSON(w, r, &in) {
			return
		}
		out, err := s.store.CreateSite(u, in)
		if err != nil {
			writeError(w, 400, "site_create_failed", err.Error())
			return
		}
		writeJSON(w, 201, out)
	default:
		writeError(w, 405, "method_not_allowed", "GET or POST required")
	}
}
func (s *Server) siteResource(w http.ResponseWriter, r *http.Request, u User, sess Session) {
	parts := splitAfter(r.URL.Path, "/api/v1/admin/sites/")
	if len(parts) != 1 || r.Method != http.MethodPatch {
		writeError(w, 405, "method_not_allowed", "PATCH required")
		return
	}
	var in SiteInput
	if !decodeJSON(w, r, &in) {
		return
	}
	out, err := s.store.UpdateSite(u, parts[0], in)
	if err != nil {
		writeError(w, 400, "site_update_failed", err.Error())
		return
	}
	writeJSON(w, 200, out)
}
func (s *Server) audit(w http.ResponseWriter, r *http.Request, u User, sess Session) {
	if r.Method != http.MethodGet {
		writeError(w, 405, "method_not_allowed", "GET required")
		return
	}
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	writeJSON(w, 200, s.store.Audit(limit))
}
func (s *Server) sessionList(w http.ResponseWriter, r *http.Request, u User, current Session) {
	if r.Method != http.MethodGet {
		writeError(w, 405, "method_not_allowed", "GET required")
		return
	}
	type item struct {
		UserID    string    `json:"userId"`
		ExpiresAt time.Time `json:"expiresAt"`
		Current   bool      `json:"current"`
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now().UTC()
	out := []item{}
	for token, sess := range s.sessions {
		if now.After(sess.ExpiresAt) {
			delete(s.sessions, token)
			continue
		}
		out = append(out, item{UserID: sess.UserID, ExpiresAt: sess.ExpiresAt, Current: sess.Token == current.Token})
	}
	writeJSON(w, 200, out)
}
func readManifestKV(path string) map[string]string {
	data, err := os.ReadFile(path)
	if err != nil {
		return map[string]string{}
	}
	out := map[string]string{}
	for _, line := range strings.Split(string(data), "\n") {
		parts := strings.SplitN(strings.TrimSpace(line), "=", 2)
		if len(parts) == 2 {
			out[parts[0]] = parts[1]
		}
	}
	return out
}
func rapidPackageVersion() string {
	data, err := os.ReadFile("/var/lib/dpkg/status")
	if err != nil {
		return ""
	}
	for _, block := range strings.Split(string(data), "\n\n") {
		if strings.Contains(block, "Package: rapidscada\n") || strings.HasPrefix(block, "Package: rapidscada\n") {
			for _, line := range strings.Split(block, "\n") {
				if strings.HasPrefix(line, "Version: ") {
					return strings.TrimSpace(strings.TrimPrefix(line, "Version: "))
				}
			}
		}
	}
	return ""
}
func (s *Server) gatewayStatus(ctx context.Context) any {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://127.0.0.1:18080/status", nil)
	if err != nil {
		return nil
	}
	resp, err := s.client.Do(req)
	if err != nil {
		return nil
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil
	}
	var payload any
	if json.Unmarshal(body, &payload) != nil {
		return nil
	}
	return payload
}
func (s *Server) diagnosticsSystem(w http.ResponseWriter, r *http.Request, u User, sess Session) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "GET required")
		return
	}
	monitorStatus, monitorBody, _, monitorErr := s.monitorGET(r.Context(), "/api/v1/system/health")
	gateway := s.gatewayStatus(r.Context())
	var monitor any
	if monitorErr == nil && monitorStatus == http.StatusOK {
		_ = json.Unmarshal(monitorBody, &monitor)
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"gateway": gateway,
		"monitor": monitor,
	})
}

func (s *Server) systemInfo(w http.ResponseWriter, r *http.Request, u User, sess Session) {
	if r.Method != http.MethodGet {
		writeError(w, 405, "method_not_allowed", "GET required")
		return
	}
	manifest := readManifestKV("/opt/rc-gateway/current/MANIFEST")
	monitorStatus, monitorBody, _, monitorErr := s.monitorGET(r.Context(), "/api/v1/system/health")
	gateway := s.gatewayStatus(r.Context())
	var monitor any
	if monitorErr == nil && monitorStatus == http.StatusOK {
		_ = json.Unmarshal(monitorBody, &monitor)
	}
	writeJSON(w, 200, map[string]any{
		"service": "rc-admin", "version": s.opt.Version, "apiVersion": APIVersion,
		"cookieSecure": s.opt.CookieSecure, "sessionTtlSeconds": int64(s.opt.SessionTTL.Seconds()),
		"profileCatalog": s.opt.ProfileCatalogPath, "bindingRoot": s.opt.BindingRoot,
		"releaseVersion": manifest["version"], "commit": manifest["commit"], "buildDate": manifest["buildDate"],
		"nodeVersion": manifest["node"], "goVersion": manifest["go"], "rapidScadaVersion": rapidPackageVersion(),
		"gatewayVersion": manifest["version"], "monitorVersion": manifest["version"], "adminVersion": s.opt.Version, "frontendVersion": manifest["version"],
		"gateway": gateway, "monitor": monitor,
	})
}

func (s *Server) settings(w http.ResponseWriter, r *http.Request, u User, sess Session) {
	switch r.Method {
	case http.MethodGet:
		writeJSON(w, 200, s.store.Settings())
	case http.MethodPatch:
		if !HasPermission(u.Role, PermSettingsWrite) {
			writeError(w, 403, "forbidden", "permission denied")
			return
		}
		var in SystemSettings
		if !decodeJSON(w, r, &in) {
			return
		}
		out, err := s.store.UpdateSettings(u, in)
		if err != nil {
			writeError(w, 400, "settings_update_failed", err.Error())
			return
		}
		writeJSON(w, 200, out)
	default:
		writeError(w, 405, "method_not_allowed", "GET or PATCH required")
	}
}
func (s *Server) profileStates(w http.ResponseWriter, r *http.Request, u User, sess Session) {
	if r.Method != http.MethodGet {
		writeError(w, 405, "method_not_allowed", "GET required")
		return
	}
	writeJSON(w, 200, s.store.ProfileStates())
}
func (s *Server) profileStateResource(w http.ResponseWriter, r *http.Request, u User, sess Session) {
	if r.Method != http.MethodPatch {
		writeError(w, 405, "method_not_allowed", "PATCH required")
		return
	}
	if !HasPermission(u.Role, PermCommissioningWrite) {
		writeError(w, 403, "forbidden", "permission denied")
		return
	}
	parts := splitAfter(r.URL.Path, "/api/v1/engineering/profile-states/")
	if len(parts) != 1 {
		writeError(w, 404, "not_found", "profile not found")
		return
	}
	var in struct {
		Status   string `json:"status"`
		Version  string `json:"version"`
		Evidence string `json:"evidence"`
	}
	if !decodeJSON(w, r, &in) {
		return
	}
	out, err := s.store.SetProfileState(u, parts[0], in.Status, in.Version, in.Evidence)
	if err != nil {
		writeError(w, 400, "profile_state_failed", err.Error())
		return
	}
	writeJSON(w, 200, out)
}

type bindingMetricView struct {
	Key                string `json:"key"`
	RapidChannelNumber int    `json:"rapidChannelNumber"`
}
type bindingAlarmView struct {
	Code               string `json:"code"`
	RapidChannelNumber int    `json:"rapidChannelNumber"`
}
type bindingEventView struct {
	Type               string `json:"type"`
	RapidChannelNumber int    `json:"rapidChannelNumber"`
}
type bindingCatalogEntry struct {
	ProfileID string              `json:"profileId"`
	File      string              `json:"file"`
	Metrics   []bindingMetricView `json:"metrics"`
	Alarms    []bindingAlarmView  `json:"alarms"`
	Events    []bindingEventView  `json:"events"`
}

func (s *Server) bindings(w http.ResponseWriter, r *http.Request, u User, sess Session) {
	if r.Method != http.MethodGet {
		writeError(w, 405, "method_not_allowed", "GET required")
		return
	}
	root := filepath.Clean(s.opt.BindingRoot)
	entries := []bindingCatalogEntry{}
	err := filepath.Walk(root, func(path string, info os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if info.IsDir() {
			return nil
		}
		base := filepath.Base(path)
		if !strings.HasPrefix(base, "channels") || filepath.Ext(base) != ".json" || filepath.Base(filepath.Dir(path)) != "rapid" {
			return nil
		}
		if info.Size() > 1<<20 {
			return fmt.Errorf("binding file too large: %s", base)
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		var raw struct {
			ProfileID string `json:"profileId"`
			Metrics   []struct {
				Key           string `json:"key"`
				ChannelNumber int    `json:"channelNumber"`
			} `json:"metrics"`
			Alarms []struct {
				Code          string `json:"code"`
				ChannelNumber int    `json:"channelNumber"`
			} `json:"alarms"`
			Events []struct {
				Type          string `json:"type"`
				ChannelNumber int    `json:"channelNumber"`
			} `json:"events"`
		}
		if err := json.Unmarshal(data, &raw); err != nil {
			return fmt.Errorf("decode binding %s: %w", base, err)
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		entry := bindingCatalogEntry{ProfileID: raw.ProfileID, File: filepath.ToSlash(rel), Metrics: []bindingMetricView{}, Alarms: []bindingAlarmView{}, Events: []bindingEventView{}}
		for _, m := range raw.Metrics {
			if m.Key != "" && m.ChannelNumber > 0 {
				entry.Metrics = append(entry.Metrics, bindingMetricView{Key: m.Key, RapidChannelNumber: m.ChannelNumber})
			}
		}
		for _, a := range raw.Alarms {
			if a.Code != "" && a.ChannelNumber > 0 {
				entry.Alarms = append(entry.Alarms, bindingAlarmView{Code: a.Code, RapidChannelNumber: a.ChannelNumber})
			}
		}
		for _, e := range raw.Events {
			if e.Type != "" && e.ChannelNumber > 0 {
				entry.Events = append(entry.Events, bindingEventView{Type: e.Type, RapidChannelNumber: e.ChannelNumber})
			}
		}
		entries = append(entries, entry)
		if len(entries) > 200 {
			return errors.New("too many binding files")
		}
		return nil
	})
	if err != nil {
		writeError(w, 503, "binding_catalog_unavailable", err.Error())
		return
	}
	writeJSON(w, 200, entries)
}

func (s *Server) engineeringMonitorGenerators(w http.ResponseWriter, r *http.Request, u User, sess Session) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "GET required")
		return
	}
	status, body, _, err := s.monitorGET(r.Context(), "/api/v1/generators")
	if err != nil {
		writeError(w, http.StatusBadGateway, "monitor_unavailable", "RC Monitor unavailable")
		return
	}
	if status != http.StatusOK {
		writeRawJSON(w, status, body)
		return
	}
	var items []json.RawMessage
	if json.Unmarshal(body, &items) != nil {
		writeError(w, http.StatusBadGateway, "monitor_invalid_response", "RC Monitor returned invalid generator list")
		return
	}
	filtered := make([]json.RawMessage, 0, len(items))
	for _, raw := range items {
		var meta monitorGeneratorMeta
		if json.Unmarshal(raw, &meta) == nil && meta.ID != "" && allowedSite(u, meta.SiteID) {
			filtered = append(filtered, raw)
		}
	}
	writeJSON(w, http.StatusOK, filtered)
}

func (s *Server) profiles(w http.ResponseWriter, r *http.Request, u User, sess Session) {
	if r.Method != http.MethodGet {
		writeError(w, 405, "method_not_allowed", "GET required")
		return
	}
	data, err := os.ReadFile(s.opt.ProfileCatalogPath)
	if err != nil {
		writeError(w, 503, "profile_catalog_unavailable", "profile catalog unavailable")
		return
	}
	var payload any
	if err := json.Unmarshal(data, &payload); err != nil {
		writeError(w, 500, "profile_catalog_invalid", "profile catalog is invalid")
		return
	}
	writeJSON(w, 200, payload)
}

type preflightCheck struct {
	ID      string `json:"id"`
	Status  string `json:"status"`
	Message string `json:"message"`
}
type commissioningPreflight struct {
	CommissioningID string           `json:"commissioningId"`
	Checks          []preflightCheck `json:"checks"`
	Pass            bool             `json:"pass"`
}

func (s *Server) runPreflight(c Commissioning) commissioningPreflight {
	checks := []preflightCheck{}
	add := func(id string, ok bool, msg string) {
		status := "FAIL"
		if ok {
			status = "PASS"
		}
		checks = append(checks, preflightCheck{ID: id, Status: status, Message: msg})
	}
	add("identity", strings.TrimSpace(c.Tag) != "" && strings.TrimSpace(c.Name) != "", "tag/name present")
	siteOK := false
	for _, site := range s.store.ListSites() {
		if site.ID == c.SiteID && site.Active {
			siteOK = true
			break
		}
	}
	add("site", siteOK, "active site exists")
	transportOK := strings.TrimSpace(c.Transport.Kind) != ""
	if strings.EqualFold(c.Transport.Kind, "TCP/IP") {
		transportOK = transportOK && strings.TrimSpace(c.Transport.Host) != "" && c.Transport.Port > 0 && c.Transport.Port <= 65535
	}
	add("transport", transportOK, "transport fields structurally valid")
	profileOK := strings.TrimSpace(c.Controller.ProfileID) != ""
	add("profile", profileOK, "controller profile selected")
	rapidOK := c.RapidPlanHash == "" || len(c.RapidPlanHash) == 64
	add("rapid_plan_hash", rapidOK, "Rapid plan SHA-256 is empty/pending or 64 hex chars")
	monitorOK := strings.TrimSpace(c.MonitorGeneratorID) != ""
	add("monitor_link", monitorOK, "operational RC Monitor generatorId linked")
	pass := true
	for _, x := range checks {
		if x.Status != "PASS" {
			pass = false
		}
	}
	return commissioningPreflight{CommissioningID: c.ID, Checks: checks, Pass: pass}
}

type monitorCapabilityMetric struct {
	Key      string `json:"key"`
	Required bool   `json:"required"`
}
type monitorCapabilities struct {
	GeneratorID string                    `json:"generatorId"`
	Metrics     []monitorCapabilityMetric `json:"metrics"`
}
type monitorMetric struct {
	Quality string `json:"quality"`
}
type monitorTelemetry struct {
	GeneratorID   string                   `json:"generatorId"`
	Communication string                   `json:"communication"`
	Metrics       map[string]monitorMetric `json:"metrics"`
}

func (s *Server) validateCommissioningTelemetry(ctx context.Context, actor User, c Commissioning) (map[string]any, error) {
	if strings.TrimSpace(c.MonitorGeneratorID) == "" {
		return nil, errors.New("monitorGeneratorId is required")
	}
	base := "/api/v1/generators/" + url.PathEscape(c.MonitorGeneratorID)
	status, body, _, err := s.monitorGET(ctx, base+"/capabilities")
	if err != nil || status != 200 {
		return nil, errors.New("capabilities unavailable")
	}
	var caps monitorCapabilities
	if json.Unmarshal(body, &caps) != nil {
		return nil, errors.New("invalid capabilities response")
	}
	status, body, _, err = s.monitorGET(ctx, base+"/telemetry")
	if err != nil || status != 200 {
		return nil, errors.New("telemetry unavailable")
	}
	var tel monitorTelemetry
	if json.Unmarshal(body, &tel) != nil {
		return nil, errors.New("invalid telemetry response")
	}
	results := map[string]string{}
	pass := tel.Communication == "online"
	for _, m := range caps.Metrics {
		if !m.Required {
			continue
		}
		sample, ok := tel.Metrics[m.Key]
		if !ok {
			results[m.Key] = "ABSENT"
			pass = false
			continue
		}
		results[m.Key] = strings.ToUpper(sample.Quality)
		if sample.Quality != "good" {
			pass = false
		}
	}
	statusWord := "FAIL"
	if pass {
		statusWord = "PASS"
	}
	evidenceBytes, _ := json.Marshal(results)
	_, gateErr := s.store.SetGate(actor, c.ID, "TELEMETRY_VALIDATION", GateInput{Status: GateStatus(statusWord), Evidence: "automated required-metric validation: " + string(evidenceBytes)})
	if gateErr != nil {
		return nil, gateErr
	}
	return map[string]any{"pass": pass, "communication": tel.Communication, "requiredMetrics": results}, nil
}

func (s *Server) commissionings(w http.ResponseWriter, r *http.Request, u User, sess Session) {
	switch r.Method {
	case http.MethodGet:
		writeJSON(w, 200, s.store.ListCommissionings(u))
	case http.MethodPost:
		if !HasPermission(u.Role, PermCommissioningWrite) {
			writeError(w, 403, "forbidden", "permission denied")
			return
		}
		var in CommissioningInput
		if !decodeJSON(w, r, &in) {
			return
		}
		out, err := s.store.CreateCommissioning(u, in)
		if err != nil {
			writeError(w, 400, "commissioning_create_failed", err.Error())
			return
		}
		writeJSON(w, 201, out)
	default:
		writeError(w, 405, "method_not_allowed", "GET or POST required")
	}
}
func (s *Server) commissioningResource(w http.ResponseWriter, r *http.Request, u User, sess Session) {
	parts := splitAfter(r.URL.Path, "/api/v1/engineering/commissionings/")
	if len(parts) < 1 {
		writeError(w, 404, "not_found", "resource not found")
		return
	}
	id := parts[0]
	if len(parts) == 1 && r.Method == http.MethodGet {
		out, ok := s.store.CommissioningByID(u, id)
		if !ok {
			writeError(w, http.StatusNotFound, "not_found", "commissioning not found")
			return
		}
		writeJSON(w, http.StatusOK, out)
		return
	}
	if len(parts) == 1 && r.Method == http.MethodPatch {
		if !HasPermission(u.Role, PermCommissioningWrite) {
			writeError(w, 403, "forbidden", "permission denied")
			return
		}
		var in CommissioningInput
		if !decodeJSON(w, r, &in) {
			return
		}
		out, err := s.store.UpdateCommissioning(u, id, in)
		if err != nil {
			writeError(w, 400, "commissioning_update_failed", err.Error())
			return
		}
		writeJSON(w, 200, out)
		return
	}
	if len(parts) == 3 && parts[1] == "gates" && r.Method == http.MethodPost {
		if !HasPermission(u.Role, PermCommissioningWrite) {
			writeError(w, 403, "forbidden", "permission denied")
			return
		}
		var in GateInput
		if !decodeJSON(w, r, &in) {
			return
		}
		out, err := s.store.SetGate(u, id, strings.ToUpper(parts[2]), in)
		if err != nil {
			writeError(w, 400, "gate_update_failed", err.Error())
			return
		}
		writeJSON(w, 200, out)
		return
	}
	if len(parts) == 2 && parts[1] == "preflight" && r.Method == http.MethodPost {
		if !HasPermission(u.Role, PermCommissioningWrite) {
			writeError(w, 403, "forbidden", "permission denied")
			return
		}
		c, ok := s.store.CommissioningByID(u, id)
		if !ok {
			writeError(w, 404, "not_found", "commissioning not found")
			return
		}
		writeJSON(w, 200, s.runPreflight(c))
		return
	}
	if len(parts) == 2 && parts[1] == "validate-telemetry" && r.Method == http.MethodPost {
		if !HasPermission(u.Role, PermCommissioningWrite) {
			writeError(w, 403, "forbidden", "permission denied")
			return
		}
		c, ok := s.store.CommissioningByID(u, id)
		if !ok {
			writeError(w, 404, "not_found", "commissioning not found")
			return
		}
		out, err := s.validateCommissioningTelemetry(r.Context(), u, c)
		if err != nil {
			writeError(w, 409, "telemetry_validation_failed", err.Error())
			return
		}
		writeJSON(w, 200, out)
		return
	}
	if len(parts) == 2 && parts[1] == "change" && r.Method == http.MethodPost {
		if !HasPermission(u.Role, PermCommissioningWrite) {
			writeError(w, 403, "forbidden", "permission denied")
			return
		}
		var in struct {
			Reason string `json:"reason"`
		}
		if !decodeJSON(w, r, &in) {
			return
		}
		out, err := s.store.CreateChangeCommissioning(u, id, in.Reason)
		if err != nil {
			writeError(w, 409, "change_commissioning_failed", err.Error())
			return
		}
		writeJSON(w, 201, out)
		return
	}
	if len(parts) == 2 && parts[1] == "promote" && r.Method == http.MethodPost {
		if !HasPermission(u.Role, PermCommissioningPromote) {
			writeError(w, 403, "forbidden", "permission denied")
			return
		}
		candidate, ok := s.store.CommissioningByID(u, id)
		if !ok {
			writeError(w, 404, "not_found", "commissioning not found")
			return
		}
		if strings.TrimSpace(candidate.MonitorGeneratorID) == "" {
			writeError(w, 409, "commissioning_promote_failed", "monitorGeneratorId is required before promotion")
			return
		}
		status, body, _, monitorErr := s.monitorGET(r.Context(), "/api/v1/generators/"+url.PathEscape(candidate.MonitorGeneratorID))
		if monitorErr != nil || status != http.StatusOK {
			writeError(w, 409, "commissioning_promote_failed", "linked operational generator is not available in RC Monitor")
			return
		}
		var meta monitorGeneratorMeta
		if json.Unmarshal(body, &meta) != nil || meta.SiteID != candidate.SiteID {
			writeError(w, 409, "commissioning_promote_failed", "linked generator site does not match commissioning site")
			return
		}
		out, err := s.store.PromoteCommissioning(u, id)
		if err != nil {
			writeError(w, 409, "commissioning_promote_failed", err.Error())
			return
		}
		writeJSON(w, 200, out)
		return
	}
	if len(parts) == 2 && (parts[1] == "suspend" || parts[1] == "retire") && r.Method == http.MethodPost {
		if !HasPermission(u.Role, PermCommissioningWrite) {
			writeError(w, 403, "forbidden", "permission denied")
			return
		}
		life := "SUSPENDED"
		if parts[1] == "retire" {
			life = "RETIRED"
		}
		out, err := s.store.SetLifecycle(u, id, life)
		if err != nil {
			writeError(w, 400, "lifecycle_update_failed", err.Error())
			return
		}
		writeJSON(w, 200, out)
		return
	}
	writeError(w, 405, "method_not_allowed", "unsupported commissioning operation")
}

func splitAfter(path, prefix string) []string {
	rem := strings.Trim(strings.TrimPrefix(path, prefix), "/")
	if rem == "" {
		return nil
	}
	return strings.Split(rem, "/")
}
func decodeJSON(w http.ResponseWriter, r *http.Request, out any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(out); err != nil {
		writeError(w, 400, "invalid_json", "invalid JSON request")
		return false
	}
	return true
}
func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
func writeError(w http.ResponseWriter, status int, code, message string) {
	writeJSON(w, status, map[string]any{"error": map[string]string{"code": code, "message": message}})
}
