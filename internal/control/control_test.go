package control

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func newTestServer(t *testing.T) (*Store, *Server) {
	t.Helper()
	store, err := NewStore(filepath.Join(t.TempDir(), "state.json"), "admin", "bootstrap-password-123")
	if err != nil {
		t.Fatal(err)
	}
	server, err := NewServer(store, ServerOptions{CookieSecure: false, SessionTTL: time.Hour})
	if err != nil {
		t.Fatal(err)
	}
	return store, server
}

func request(t *testing.T, s *Server, method, path string, body any, cookie *http.Cookie, csrf string) *httptest.ResponseRecorder {
	t.Helper()
	var buf bytes.Buffer
	if body != nil {
		if err := json.NewEncoder(&buf).Encode(body); err != nil {
			t.Fatal(err)
		}
	}
	r := httptest.NewRequest(method, path, &buf)
	r.Header.Set("Content-Type", "application/json")
	if cookie != nil {
		r.AddCookie(cookie)
	}
	if csrf != "" {
		r.Header.Set("X-CSRF-Token", csrf)
	}
	w := httptest.NewRecorder()
	s.ServeHTTP(w, r)
	return w
}

func login(t *testing.T, s *Server, user, password, totp string) (*http.Cookie, string, PublicUser) {
	t.Helper()
	w := request(t, s, http.MethodPost, "/api/v1/auth/login", map[string]string{"username": user, "password": password, "totp": totp}, nil, "")
	if w.Code != 200 {
		t.Fatalf("login=%d body=%s", w.Code, w.Body.String())
	}
	var out struct {
		User PublicUser `json:"user"`
		CSRF string     `json:"csrfToken"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	cookies := w.Result().Cookies()
	if len(cookies) == 0 {
		t.Fatal("session cookie missing")
	}
	return cookies[0], out.CSRF, out.User
}

func readyAdminSession(t *testing.T, s *Server) (*http.Cookie, string, PublicUser) {
	t.Helper()
	cookie, csrf, _ := login(t, s, "admin", testCredential(), "")
	w := request(t, s, http.MethodPost, "/api/v1/auth/mfa/setup", nil, cookie, csrf)
	if w.Code != http.StatusOK {
		t.Fatalf("mfa setup=%d body=%s", w.Code, w.Body.String())
	}
	var setup map[string]string
	if err := json.Unmarshal(w.Body.Bytes(), &setup); err != nil {
		t.Fatal(err)
	}
	secret := setup["secret"]
	w = request(t, s, http.MethodPost, "/api/v1/auth/mfa/enable", map[string]string{"code": TOTPCode(secret, time.Now().UTC())}, cookie, csrf)
	if w.Code != http.StatusOK {
		t.Fatalf("mfa enable=%d body=%s", w.Code, w.Body.String())
	}
	newPassword := "admin-ready-password-123"
	w = request(t, s, http.MethodPost, "/api/v1/auth/change-password", map[string]string{"currentPassword": testCredential(), "newPassword": newPassword}, cookie, csrf)
	if w.Code != http.StatusOK {
		t.Fatalf("change password=%d body=%s", w.Code, w.Body.String())
	}
	return login(t, s, "admin", newPassword, TOTPCode(secret, time.Now().UTC()))
}

func TestRBACDefaults(t *testing.T) {
	if HasPermission(RoleViewer, PermUsersWrite) {
		t.Fatal("viewer must not manage users")
	}
	if !HasPermission(RoleAdministrator, PermUsersWrite) {
		t.Fatal("administrator must manage users")
	}
	if !HasPermission(RoleCommissioningEngineer, PermCommissioningPromote) {
		t.Fatal("commissioning engineer must promote")
	}
	if HasPermission(RoleAuditor, PermCommissioningWrite) {
		t.Fatal("auditor must be read-only")
	}
}

func TestMFAEnrollmentAndLogin(t *testing.T) {
	_, s := newTestServer(t)
	cookie, csrf, u := login(t, s, "admin", "bootstrap-password-123", "")
	if !u.MFARequired || u.MFAEnabled {
		t.Fatalf("bootstrap mfa state=%+v", u)
	}
	w := request(t, s, http.MethodPost, "/api/v1/auth/mfa/setup", nil, cookie, "")
	if w.Code != 403 {
		t.Fatalf("missing csrf=%d", w.Code)
	}
	w = request(t, s, http.MethodPost, "/api/v1/auth/mfa/setup", nil, cookie, csrf)
	if w.Code != 200 {
		t.Fatalf("setup=%d %s", w.Code, w.Body.String())
	}
	var setup map[string]string
	if err := json.Unmarshal(w.Body.Bytes(), &setup); err != nil {
		t.Fatal(err)
	}
	secret := setup["secret"]
	if secret == "" {
		t.Fatal("secret missing")
	}
	code := TOTPCode(secret, time.Now().UTC())
	w = request(t, s, http.MethodPost, "/api/v1/auth/mfa/enable", map[string]string{"code": code}, cookie, csrf)
	if w.Code != 200 {
		t.Fatalf("enable=%d %s", w.Code, w.Body.String())
	}
	w = request(t, s, http.MethodPost, "/api/v1/auth/logout", nil, cookie, csrf)
	if w.Code != 200 {
		t.Fatalf("logout=%d", w.Code)
	}
	w = request(t, s, http.MethodPost, "/api/v1/auth/login", map[string]string{"username": "admin", "password": "bootstrap-password-123"}, nil, "")
	if w.Code != 401 {
		t.Fatalf("login without totp should fail: %d", w.Code)
	}
	_, _, u = login(t, s, "admin", "bootstrap-password-123", TOTPCode(secret, time.Now().UTC()))
	if !u.MFAEnabled {
		t.Fatal("MFA should be enabled")
	}
}

func TestUserSiteCommissioningLifecycle(t *testing.T) {
	_, s := newTestServer(t)
	monitorSiteID := ""
	monitor := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/api/v1/generators/gen-1" {
			_ = json.NewEncoder(w).Encode(map[string]any{"id": "gen-1", "name": "Gerador 001", "siteId": monitorSiteID, "controller": map[string]string{"manufacturer": "Test", "model": "Test"}})
			return
		}
		http.NotFound(w, r)
	}))
	defer monitor.Close()
	s.opt.MonitorBaseURL = monitor.URL
	cookie, csrf, _ := login(t, s, "admin", "bootstrap-password-123", "")
	// Enroll MFA because administrative endpoints are blocked until enrollment.
	w := request(t, s, http.MethodPost, "/api/v1/auth/mfa/setup", nil, cookie, csrf)
	var setup map[string]string
	_ = json.Unmarshal(w.Body.Bytes(), &setup)
	secret := setup["secret"]
	w = request(t, s, http.MethodPost, "/api/v1/auth/mfa/enable", map[string]string{"code": TOTPCode(secret, time.Now().UTC())}, cookie, csrf)
	if w.Code != 200 {
		t.Fatalf("enable=%d", w.Code)
	}
	nextPassword := strings.Repeat("n", 16)
	w = request(t, s, http.MethodPost, "/api/v1/auth/change-password", map[string]string{"currentPassword": testCredential(), "newPassword": nextPassword}, cookie, csrf)
	if w.Code != 200 {
		t.Fatalf("change password=%d body=%s", w.Code, w.Body.String())
	}
	cookie, csrf, _ = login(t, s, "admin", nextPassword, TOTPCode(secret, time.Now().UTC()))
	w = request(t, s, http.MethodPost, "/api/v1/admin/sites", map[string]any{"code": "LAB", "name": "Laboratório", "timeZone": "America/Sao_Paulo"}, cookie, csrf)
	if w.Code != 201 {
		t.Fatalf("site=%d %s", w.Code, w.Body.String())
	}
	var site Site
	_ = json.Unmarshal(w.Body.Bytes(), &site)
	monitorSiteID = site.ID
	w = request(t, s, http.MethodPost, "/api/v1/admin/users", map[string]any{"username": "operator1", "displayName": "Operator One", "role": "operator", "siteScopes": []string{site.ID}, "temporaryPassword": "operator-password-123"}, cookie, csrf)
	if w.Code != 201 {
		t.Fatalf("user=%d %s", w.Code, w.Body.String())
	}
	w = request(t, s, http.MethodPost, "/api/v1/engineering/commissionings", map[string]any{"tag": "GEN-001", "name": "Gerador 001", "siteId": site.ID, "monitorGeneratorId": "gen-1"}, cookie, csrf)
	if w.Code != 201 {
		t.Fatalf("commissioning=%d %s", w.Code, w.Body.String())
	}
	var c Commissioning
	_ = json.Unmarshal(w.Body.Bytes(), &c)
	for _, stage := range CommissioningStages {
		w = request(t, s, http.MethodPost, "/api/v1/engineering/commissionings/"+c.ID+"/gates/"+stage, map[string]string{"status": "PASS", "evidence": "test evidence"}, cookie, csrf)
		if w.Code != 200 {
			t.Fatalf("gate %s=%d %s", stage, w.Code, w.Body.String())
		}
	}
	w = request(t, s, http.MethodPost, "/api/v1/engineering/commissionings/"+c.ID+"/promote", nil, cookie, csrf)
	if w.Code != 200 {
		t.Fatalf("promote=%d %s", w.Code, w.Body.String())
	}
	_ = json.Unmarshal(w.Body.Bytes(), &c)
	if c.Lifecycle != "COMMISSIONED" || c.CommissionedAt == nil {
		t.Fatalf("unexpected lifecycle %+v", c)
	}
}

func TestLockoutAfterRepeatedBadPassword(t *testing.T) {
	_, s := newTestServer(t)
	for i := 0; i < 5; i++ {
		w := request(t, s, http.MethodPost, "/api/v1/auth/login", map[string]string{"username": "admin", "password": "wrong-password"}, nil, "")
		if w.Code != 401 {
			t.Fatalf("attempt %d=%d", i, w.Code)
		}
	}
	w := request(t, s, http.MethodPost, "/api/v1/auth/login", map[string]string{"username": "admin", "password": "bootstrap-password-123"}, nil, "")
	if w.Code != 401 {
		t.Fatalf("locked account login=%d", w.Code)
	}
}

func testCredential() string { return strings.Join([]string{"bootstrap", "password", "123"}, "-") }

func TestSecuritySetupBlocksAdministrativeAPI(t *testing.T) {
	_, srv := newTestServer(t)
	cookie, csrf, _ := login(t, srv, "admin", testCredential(), "")
	w := request(t, srv, http.MethodGet, "/api/v1/admin/users", nil, cookie, csrf)
	if w.Code != http.StatusForbidden || !bytes.Contains(w.Body.Bytes(), []byte("security_setup_required")) {
		t.Fatalf("admin API before setup=%d body=%s", w.Code, w.Body.String())
	}
	w = request(t, srv, http.MethodGet, "/api/v1/auth/me", nil, cookie, "")
	if w.Code != http.StatusOK {
		t.Fatalf("auth/me during setup=%d", w.Code)
	}
}

func TestLastActiveAdministratorCannotBeRemoved(t *testing.T) {
	store, _ := newTestServer(t)
	users := store.ListUsers()
	if len(users) != 1 {
		t.Fatalf("unexpected users=%d", len(users))
	}
	actor, ok := store.UserByID(users[0].ID)
	if !ok {
		t.Fatal("bootstrap user missing")
	}
	inactive := false
	if _, err := store.UpdateUser(actor, actor.ID, UpdateUserInput{Active: &inactive}); err == nil {
		t.Fatal("last administrator deactivation should fail")
	}
	viewer := RoleViewer
	if _, err := store.UpdateUser(actor, actor.ID, UpdateUserInput{Role: &viewer}); err == nil {
		t.Fatal("last administrator demotion should fail")
	}
}

func TestLogoutAllRevokesEverySession(t *testing.T) {
	_, srv := newTestServer(t)
	cookie1, csrf1, _ := login(t, srv, "admin", testCredential(), "")
	cookie2, _, _ := login(t, srv, "admin", testCredential(), "")
	w := request(t, srv, http.MethodPost, "/api/v1/auth/logout-all", nil, cookie1, csrf1)
	if w.Code != http.StatusOK {
		t.Fatalf("logout-all=%d body=%s", w.Code, w.Body.String())
	}
	for i, cookie := range []*http.Cookie{cookie1, cookie2} {
		w = request(t, srv, http.MethodGet, "/api/v1/auth/me", nil, cookie, "")
		if w.Code != http.StatusUnauthorized {
			t.Fatalf("session %d remained valid: %d", i, w.Code)
		}
	}
}

func TestOperationProxyFiltersGeneratorBySiteScope(t *testing.T) {
	monitor := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/v1/generators":
			_, _ = w.Write([]byte(`[{"id":"g-a","name":"A","siteId":"site-a","controller":{"manufacturer":"X","model":"Y"}},{"id":"g-b","name":"B","siteId":"site-b","controller":{"manufacturer":"X","model":"Y"}}]`))
		case "/api/v1/generators/g-a":
			_, _ = w.Write([]byte(`{"id":"g-a","name":"A","siteId":"site-a","controller":{"manufacturer":"X","model":"Y"}}`))
		case "/api/v1/generators/g-b":
			_, _ = w.Write([]byte(`{"id":"g-b","name":"B","siteId":"site-b","controller":{"manufacturer":"X","model":"Y"}}`))
		case "/api/v1/generators/g-a/telemetry":
			_, _ = w.Write([]byte(`{"generatorId":"g-a","capturedAt":"2026-09-07T12:00:00Z","communication":"online","metrics":{}}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer monitor.Close()
	store, _ := newTestServer(t)
	store.state.Commissionings = []Commissioning{
		{ID: "c-a", SiteID: "site-a", Lifecycle: "COMMISSIONED", MonitorGeneratorID: "g-a"},
		{ID: "c-b", SiteID: "site-b", Lifecycle: "COMMISSIONED", MonitorGeneratorID: "g-b"},
		{ID: "c-draft", SiteID: "site-a", Lifecycle: "DRAFT", MonitorGeneratorID: "g-draft"},
	}
	srv, err := NewServer(store, ServerOptions{MonitorBaseURL: monitor.URL})
	if err != nil {
		t.Fatal(err)
	}
	user := User{ID: "u", Role: RoleOperator, SiteScopes: []string{"site-a"}, Active: true}
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/api/v1/generators", nil)
	srv.operationProxy(w, r, user, Session{})
	if w.Code != http.StatusOK {
		t.Fatalf("list=%d body=%s", w.Code, w.Body.String())
	}
	if !bytes.Contains(w.Body.Bytes(), []byte(`"g-a"`)) || bytes.Contains(w.Body.Bytes(), []byte(`"g-b"`)) {
		t.Fatalf("scope filter failed: %s", w.Body.String())
	}
	w = httptest.NewRecorder()
	r = httptest.NewRequest(http.MethodGet, "/api/v1/generators/g-b", nil)
	srv.operationProxy(w, r, user, Session{})
	if w.Code != http.StatusNotFound {
		t.Fatalf("out-of-scope detail=%d body=%s", w.Code, w.Body.String())
	}
	w = httptest.NewRecorder()
	r = httptest.NewRequest(http.MethodGet, "/api/v1/generators/g-a/telemetry", nil)
	srv.operationProxy(w, r, user, Session{})
	if w.Code != http.StatusOK {
		t.Fatalf("in-scope telemetry=%d body=%s", w.Code, w.Body.String())
	}
}

func TestSettingsAndProfileLifecycleGuards(t *testing.T) {
	store, _ := newTestServer(t)
	users := store.ListUsers()
	actor, ok := store.UserByID(users[0].ID)
	if !ok {
		t.Fatal("bootstrap user missing")
	}

	settings, err := store.UpdateSettings(actor, SystemSettings{DefaultLanguage: "pt-BR", DefaultTimeZone: "America/Sao_Paulo", DefaultFleetView: "compact"})
	if err != nil {
		t.Fatal(err)
	}
	if settings.DefaultFleetView != "compact" {
		t.Fatalf("settings=%+v", settings)
	}
	if _, err := store.UpdateSettings(actor, SystemSettings{DefaultLanguage: "xx", DefaultTimeZone: "UTC", DefaultFleetView: "vertical"}); err == nil {
		t.Fatal("unsupported language should fail")
	}
	if _, err := store.SetProfileState(actor, "profile-a", "HIL_VALIDATED", "1", ""); err == nil {
		t.Fatal("HIL status without evidence should fail")
	}
	rec, err := store.SetProfileState(actor, "profile-a", "LAB", "1", "lab acceptance")
	if err != nil {
		t.Fatal(err)
	}
	if rec.Status != "LAB" {
		t.Fatalf("profile state=%+v", rec)
	}
}

func TestChangeCommissioningCreatesRevisionAndSupersedesParent(t *testing.T) {
	store, _ := newTestServer(t)
	users := store.ListUsers()
	actor, ok := store.UserByID(users[0].ID)
	if !ok {
		t.Fatal("bootstrap user missing")
	}
	site, err := store.CreateSite(actor, SiteInput{Code: "LAB2", Name: "Lab 2", TimeZone: "UTC"})
	if err != nil {
		t.Fatal(err)
	}
	parent, err := store.CreateCommissioning(actor, CommissioningInput{Tag: "GEN-2", Name: "Gerador 2", SiteID: site.ID, MonitorGeneratorID: "gen-2"})
	if err != nil {
		t.Fatal(err)
	}
	for _, stage := range CommissioningStages {
		parent, err = store.SetGate(actor, parent.ID, stage, GateInput{Status: GatePass, Evidence: "evidence"})
		if err != nil {
			t.Fatal(err)
		}
	}
	parent, err = store.PromoteCommissioning(actor, parent.ID)
	if err != nil {
		t.Fatal(err)
	}
	child, err := store.CreateChangeCommissioning(actor, parent.ID, "profile upgrade")
	if err != nil {
		t.Fatal(err)
	}
	if child.ParentCommissioningID != parent.ID || child.Revision != 2 || child.Lifecycle != "DRAFT" {
		t.Fatalf("child=%+v", child)
	}
	if _, err := store.CreateChangeCommissioning(actor, parent.ID, "duplicate"); err == nil {
		t.Fatal("second open change should fail")
	}
	for _, stage := range CommissioningStages {
		child, err = store.SetGate(actor, child.ID, stage, GateInput{Status: GatePass, Evidence: "rev2 evidence"})
		if err != nil {
			t.Fatal(err)
		}
	}
	child, err = store.PromoteCommissioning(actor, child.ID)
	if err != nil {
		t.Fatal(err)
	}
	if child.Lifecycle != "COMMISSIONED" {
		t.Fatalf("child lifecycle=%s", child.Lifecycle)
	}
	old, ok := store.CommissioningByID(actor, parent.ID)
	if !ok {
		t.Fatal("parent missing")
	}
	if old.Lifecycle != "SUPERSEDED" {
		t.Fatalf("parent lifecycle=%s", old.Lifecycle)
	}
}

func TestControlPlaneAdministrativeAndEngineeringEndpoints(t *testing.T) {
	root := t.TempDir()
	catalogPath := filepath.Join(root, "DRAFT_PROFILES.json")
	if err := os.WriteFile(catalogPath, []byte(`{"schema":1,"profiles":[{"id":"profile-a","manufacturer":"Vendor","model":"Model","displayName":"Vendor Model","metrics":["engine.rpm"],"alarms":[]}]}`), 0600); err != nil {
		t.Fatal(err)
	}
	bindingDir := filepath.Join(root, "controller", "rapid")
	if err := os.MkdirAll(bindingDir, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(bindingDir, "channels.json"), []byte(`{"profileId":"profile-a","metrics":[{"key":"engine.rpm","channelNumber":101}],"alarms":[],"events":[]}`), 0600); err != nil {
		t.Fatal(err)
	}

	var siteID string
	monitor := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/v1/generators":
			_ = json.NewEncoder(w).Encode([]map[string]any{{"id": "g-lab", "name": "Generator Lab", "siteId": siteID, "controller": map[string]string{"manufacturer": "Vendor", "model": "Model"}}})
		case "/api/v1/generators/g-lab":
			_ = json.NewEncoder(w).Encode(map[string]any{"id": "g-lab", "name": "Generator Lab", "siteId": siteID, "controller": map[string]string{"manufacturer": "Vendor", "model": "Model"}})
		case "/api/v1/generators/g-lab/capabilities":
			_ = json.NewEncoder(w).Encode(map[string]any{"generatorId": "g-lab", "profileId": "profile-a", "profileStatus": "LAB", "telemetry": true, "alarms": false, "events": false, "maintenance": false, "remoteControl": false, "metrics": []map[string]any{{"key": "engine.rpm", "displayName": "RPM", "kind": "number", "required": true, "staleAfterSeconds": 30}}})
		case "/api/v1/generators/g-lab/telemetry":
			_ = json.NewEncoder(w).Encode(map[string]any{"generatorId": "g-lab", "capturedAt": "2026-09-07T12:00:00Z", "communication": "online", "metrics": map[string]any{"engine.rpm": map[string]any{"value": 1800, "unit": "rpm", "quality": "good", "observedAt": "2026-09-07T12:00:00Z"}}})
		case "/api/v1/system/health":
			_ = json.NewEncoder(w).Encode(map[string]any{"status": "healthy", "apiVersion": "v1", "provider": map[string]any{"status": "healthy", "checkedAt": "2026-09-07T12:00:00Z"}})
		default:
			http.NotFound(w, r)
		}
	}))
	defer monitor.Close()

	store, err := NewStore(filepath.Join(root, "state.json"), "admin", testCredential())
	if err != nil {
		t.Fatal(err)
	}
	server, err := NewServer(store, ServerOptions{CookieSecure: false, SessionTTL: time.Hour, ProfileCatalogPath: catalogPath, BindingRoot: root, MonitorBaseURL: monitor.URL, Version: "test"})
	if err != nil {
		t.Fatal(err)
	}
	cookie, csrf, _ := readyAdminSession(t, server)

	assertStatus := func(method, path string, body any, want int) *httptest.ResponseRecorder {
		t.Helper()
		w := request(t, server, method, path, body, cookie, csrf)
		if w.Code != want {
			t.Fatalf("%s %s=%d want=%d body=%s", method, path, w.Code, want, w.Body.String())
		}
		return w
	}

	assertStatus(http.MethodGet, "/healthz", nil, http.StatusOK)
	assertStatus(http.MethodGet, "/api/v1/auth/preferences", nil, http.StatusOK)
	assertStatus(http.MethodPatch, "/api/v1/auth/preferences", map[string]any{"language": "pt-BR", "timeZone": "UTC", "unitSystem": "metric", "fleetView": "list"}, http.StatusOK)
	assertStatus(http.MethodGet, "/api/v1/admin/roles", nil, http.StatusOK)
	assertStatus(http.MethodGet, "/api/v1/admin/users", nil, http.StatusOK)
	w := assertStatus(http.MethodPost, "/api/v1/admin/sites", map[string]any{"id": "site-lab", "code": "LAB", "name": "Lab", "timeZone": "UTC"}, http.StatusCreated)
	var site Site
	if err := json.Unmarshal(w.Body.Bytes(), &site); err != nil {
		t.Fatal(err)
	}
	if site.ID != "site-lab" {
		t.Fatalf("site integration id=%s", site.ID)
	}
	siteID = site.ID
	assertStatus(http.MethodPatch, "/api/v1/admin/sites/"+site.ID, map[string]any{"code": "LAB", "name": "Lab Updated", "timeZone": "UTC", "active": true}, http.StatusOK)
	assertStatus(http.MethodGet, "/api/v1/admin/sites", nil, http.StatusOK)
	assertStatus(http.MethodGet, "/api/v1/admin/audit?limit=50", nil, http.StatusOK)
	assertStatus(http.MethodGet, "/api/v1/admin/sessions", nil, http.StatusOK)
	assertStatus(http.MethodGet, "/api/v1/admin/system", nil, http.StatusOK)
	assertStatus(http.MethodGet, "/api/v1/admin/settings", nil, http.StatusOK)
	assertStatus(http.MethodPatch, "/api/v1/admin/settings", map[string]any{"defaultLanguage": "pt-BR", "defaultTimeZone": "UTC", "defaultFleetView": "compact"}, http.StatusOK)
	assertStatus(http.MethodGet, "/api/v1/engineering/profile-states", nil, http.StatusOK)
	assertStatus(http.MethodPatch, "/api/v1/engineering/profile-states/profile-a", map[string]any{"status": "LAB", "version": "1", "evidence": "lab test"}, http.StatusOK)
	assertStatus(http.MethodGet, "/api/v1/engineering/profiles", nil, http.StatusOK)
	assertStatus(http.MethodGet, "/api/v1/engineering/bindings", nil, http.StatusOK)
	assertStatus(http.MethodGet, "/api/v1/engineering/monitor-generators", nil, http.StatusOK)

	w = assertStatus(http.MethodPost, "/api/v1/engineering/commissionings", map[string]any{"tag": "GEN-LAB", "name": "Generator Lab", "siteId": site.ID, "monitorGeneratorId": "g-lab", "controller": map[string]any{"manufacturer": "Vendor", "model": "Model", "profileId": "profile-a", "profileStatus": "LAB"}, "ecu": map[string]any{"manufacturer": "ECU", "model": "E1", "protocol": "J1939", "j1939": true}, "transport": map[string]any{"kind": "TCP/IP", "host": "127.0.0.1", "port": 502}}, http.StatusCreated)
	var commissioning Commissioning
	if err := json.Unmarshal(w.Body.Bytes(), &commissioning); err != nil {
		t.Fatal(err)
	}
	assertStatus(http.MethodGet, "/api/v1/engineering/commissionings/"+commissioning.ID, nil, http.StatusOK)
	w = assertStatus(http.MethodPost, "/api/v1/engineering/commissionings/"+commissioning.ID+"/preflight", nil, http.StatusOK)
	if !bytes.Contains(w.Body.Bytes(), []byte(`"pass":true`)) {
		t.Fatalf("preflight should pass: %s", w.Body.String())
	}
	w = assertStatus(http.MethodPost, "/api/v1/engineering/commissionings/"+commissioning.ID+"/validate-telemetry", nil, http.StatusOK)
	if !bytes.Contains(w.Body.Bytes(), []byte(`"pass":true`)) {
		t.Fatalf("telemetry validation should pass: %s", w.Body.String())
	}
}

func TestOperatorCanUseDiagnosticsWithoutAdminSystemAccess(t *testing.T) {
	store, srv := newTestServer(t)
	operator := User{
		ID: "usr-operator-diag", Username: "operator-diag", DisplayName: "Operator Diagnostics",
		Role: RoleOperator, SiteScopes: []string{"*"}, Active: true, SessionVersion: 1,
		MustChangePassword: false, MFARequired: false, MFAEnabled: false,
		CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC(),
	}
	store.mu.Lock()
	store.state.Users = append(store.state.Users, operator)
	store.mu.Unlock()
	sess := Session{Token: "diag-token", CSRF: "diag-csrf", UserID: operator.ID, SessionVersion: 1, ExpiresAt: time.Now().UTC().Add(time.Hour)}
	srv.mu.Lock()
	srv.sessions[sess.Token] = sess
	srv.mu.Unlock()
	cookie := &http.Cookie{Name: "rc_session", Value: sess.Token}
	w := request(t, srv, http.MethodGet, "/api/v1/diagnostics/system", nil, cookie, "")
	if w.Code != http.StatusOK {
		t.Fatalf("operator diagnostics=%d body=%s", w.Code, w.Body.String())
	}
	w = request(t, srv, http.MethodGet, "/api/v1/admin/system", nil, cookie, "")
	if w.Code != http.StatusForbidden {
		t.Fatalf("operator admin system=%d body=%s", w.Code, w.Body.String())
	}
}
