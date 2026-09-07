package control

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"golang.org/x/crypto/bcrypt"
)

const stateSchema = 1

type stateFile struct {
	Schema         int                               `json:"schema"`
	Users          []User                            `json:"users"`
	Sites          []Site                            `json:"sites"`
	Commissionings []Commissioning                   `json:"commissionings"`
	Audit          []AuditEvent                      `json:"audit"`
	ProfileStates  map[string]ProfileLifecycleRecord `json:"profileStates,omitempty"`
	Settings       SystemSettings                    `json:"settings"`
}

type Store struct {
	mu    sync.Mutex
	path  string
	state stateFile
}

func NewStore(path, bootstrapUser, bootstrapPassword string) (*Store, error) {
	if strings.TrimSpace(path) == "" {
		return nil, errors.New("state path is required")
	}
	s := &Store{path: path, state: stateFile{Schema: stateSchema}}
	if data, err := os.ReadFile(path); err == nil {
		if err := json.Unmarshal(data, &s.state); err != nil {
			return nil, fmt.Errorf("decode state: %w", err)
		}
		if s.state.Schema != stateSchema {
			return nil, fmt.Errorf("unsupported state schema %d", s.state.Schema)
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}
	if s.state.ProfileStates == nil {
		s.state.ProfileStates = map[string]ProfileLifecycleRecord{}
	}
	if s.state.Settings.DefaultLanguage == "" {
		s.state.Settings.DefaultLanguage = "pt-BR"
	}
	if s.state.Settings.DefaultTimeZone == "" {
		s.state.Settings.DefaultTimeZone = "America/Sao_Paulo"
	}
	if s.state.Settings.DefaultFleetView == "" {
		s.state.Settings.DefaultFleetView = "vertical"
	}
	if len(s.state.Users) == 0 {
		bootstrapUser = strings.TrimSpace(strings.ToLower(bootstrapUser))
		if bootstrapUser == "" || len(bootstrapPassword) < 12 {
			return nil, errors.New("empty state requires RC_ADMIN_BOOTSTRAP_USER and RC_ADMIN_BOOTSTRAP_PASSWORD with at least 12 characters")
		}
		hash, err := bcrypt.GenerateFromPassword([]byte(bootstrapPassword), 12)
		if err != nil {
			return nil, err
		}
		now := time.Now().UTC()
		u := User{ID: newID("usr"), Username: bootstrapUser, DisplayName: "Bootstrap Administrator", Role: RoleAdministrator, SiteScopes: []string{"*"}, Active: true, PasswordHash: string(hash), MustChangePassword: true, MFARequired: true, CreatedAt: now, UpdatedAt: now, SessionVersion: 1, Preferences: defaultPreferences(s.state.Settings)}
		s.state.Users = []User{u}
		s.auditLocked(u, "bootstrap.admin.created", "user", u.ID, "success", nil)
		if err := s.saveLocked(); err != nil {
			return nil, err
		}
	}
	return s, nil
}

func newID(prefix string) string {
	b := make([]byte, 12)
	if _, err := rand.Read(b); err != nil {
		panic(err)
	}
	return prefix + "_" + hex.EncodeToString(b)
}
func randomPassword() string {
	b := make([]byte, 18)
	if _, err := rand.Read(b); err != nil {
		panic(err)
	}
	return hex.EncodeToString(b)
}

func (s *Store) saveLocked() error {
	if err := os.MkdirAll(filepath.Dir(s.path), 0700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(s.state, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	tmp, err := os.CreateTemp(filepath.Dir(s.path), ".state-*.json")
	if err != nil {
		return err
	}
	name := tmp.Name()
	defer os.Remove(name)
	if err := tmp.Chmod(0600); err != nil {
		tmp.Close()
		return err
	}
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(name, s.path)
}

func normalizeUsername(v string) string { return strings.ToLower(strings.TrimSpace(v)) }
func validUsername(v string) bool {
	if len(v) < 3 || len(v) > 64 {
		return false
	}
	for _, r := range v {
		if !(r >= 'a' && r <= 'z' || r >= '0' && r <= '9' || r == '.' || r == '_' || r == '-' || r == '@') {
			return false
		}
	}
	return true
}

func (s *Store) Authenticate(username, password string) (User, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	username = normalizeUsername(username)
	now := time.Now().UTC()
	for i := range s.state.Users {
		u := &s.state.Users[i]
		if u.Username != username {
			continue
		}
		if !u.Active {
			return User{}, errors.New("account disabled")
		}
		if u.LockedUntil != nil && now.Before(*u.LockedUntil) {
			return User{}, errors.New("account temporarily locked")
		}
		if bcrypt.CompareHashAndPassword([]byte(u.PasswordHash), []byte(password)) != nil {
			u.FailedLogins++
			if u.FailedLogins >= 5 {
				locked := now.Add(15 * time.Minute)
				u.LockedUntil = &locked
				u.FailedLogins = 0
			}
			u.UpdatedAt = now
			s.auditLocked(*u, "auth.login", "user", u.ID, "failure", map[string]string{"reason": "invalid_credentials"})
			_ = s.saveLocked()
			return User{}, errors.New("invalid credentials")
		}
		u.FailedLogins = 0
		u.LockedUntil = nil
		u.UpdatedAt = now
		if err := s.saveLocked(); err != nil {
			return User{}, err
		}
		return *u, nil
	}
	return User{}, errors.New("invalid credentials")
}

func (s *Store) RecordLoginSuccess(id string) (User, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now().UTC()
	for i := range s.state.Users {
		u := &s.state.Users[i]
		if u.ID != id {
			continue
		}
		u.LastLoginAt = &now
		u.UpdatedAt = now
		s.auditLocked(*u, "auth.login", "user", u.ID, "success", nil)
		if err := s.saveLocked(); err != nil {
			return User{}, err
		}
		return *u, nil
	}
	return User{}, errors.New("user not found")
}
func (s *Store) RecordLoginFailure(user User, reason string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.auditLocked(user, "auth.login", "user", user.ID, "failure", map[string]string{"reason": reason})
	_ = s.saveLocked()
}

func (s *Store) UserByID(id string) (User, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, u := range s.state.Users {
		if u.ID == id {
			return u, true
		}
	}
	return User{}, false
}
func (s *Store) ListUsers() []PublicUser {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]PublicUser, 0, len(s.state.Users))
	for _, u := range s.state.Users {
		out = append(out, u.Public())
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Username < out[j].Username })
	return out
}

func (s *Store) siteExistsLocked(ref string) bool {
	ref = strings.TrimSpace(ref)
	if ref == "" {
		return false
	}
	for _, site := range s.state.Sites {
		if site.ID == ref || strings.EqualFold(site.Code, ref) {
			return true
		}
	}
	return false
}

func (s *Store) validateSiteScopesLocked(scopes []string) error {
	for _, scope := range uniqueStrings(scopes) {
		if scope == "*" {
			continue
		}
		if !s.siteExistsLocked(scope) {
			return fmt.Errorf("unknown site scope %q", scope)
		}
	}
	return nil
}

type CreateUserInput struct {
	Username          string   `json:"username"`
	DisplayName       string   `json:"displayName"`
	Role              Role     `json:"role"`
	SiteScopes        []string `json:"siteScopes"`
	TemporaryPassword string   `json:"temporaryPassword"`
	MFARequired       bool     `json:"mfaRequired"`
}

func (s *Store) CreateUser(actor User, in CreateUserInput) (PublicUser, string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	in.Username = normalizeUsername(in.Username)
	if !validUsername(in.Username) {
		return PublicUser{}, "", errors.New("invalid username")
	}
	if !ValidRole(in.Role) {
		return PublicUser{}, "", errors.New("invalid role")
	}
	if err := s.validateSiteScopesLocked(in.SiteScopes); err != nil {
		return PublicUser{}, "", err
	}
	for _, u := range s.state.Users {
		if u.Username == in.Username {
			return PublicUser{}, "", errors.New("username already exists")
		}
	}
	pwd := in.TemporaryPassword
	if pwd == "" {
		pwd = randomPassword()
	}
	if len(pwd) < 12 {
		return PublicUser{}, "", errors.New("temporary password must have at least 12 characters")
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(pwd), 12)
	if err != nil {
		return PublicUser{}, "", err
	}
	now := time.Now().UTC()
	u := User{ID: newID("usr"), Username: in.Username, DisplayName: strings.TrimSpace(in.DisplayName), Role: in.Role, SiteScopes: uniqueStrings(in.SiteScopes), Active: true, PasswordHash: string(hash), MustChangePassword: true, MFARequired: in.MFARequired, CreatedAt: now, UpdatedAt: now, SessionVersion: 1, Preferences: defaultPreferences(s.state.Settings)}
	if u.Role == RoleAdministrator || u.Role == RoleCommissioningEngineer {
		u.MFARequired = true
	}
	if u.DisplayName == "" {
		u.DisplayName = u.Username
	}
	s.state.Users = append(s.state.Users, u)
	s.auditLocked(actor, "admin.user.create", "user", u.ID, "success", map[string]string{"role": string(u.Role)})
	if err := s.saveLocked(); err != nil {
		return PublicUser{}, "", err
	}
	return u.Public(), pwd, nil
}

type UpdateUserInput struct {
	DisplayName *string   `json:"displayName"`
	Role        *Role     `json:"role"`
	SiteScopes  *[]string `json:"siteScopes"`
	Active      *bool     `json:"active"`
	MFARequired *bool     `json:"mfaRequired"`
}

func (s *Store) UpdateUser(actor User, id string, in UpdateUserInput) (PublicUser, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := range s.state.Users {
		u := &s.state.Users[i]
		if u.ID != id {
			continue
		}
		proposedRole := u.Role
		proposedActive := u.Active
		if in.Role != nil {
			if !ValidRole(*in.Role) {
				return PublicUser{}, errors.New("invalid role")
			}
			proposedRole = *in.Role
		}
		if in.Active != nil {
			proposedActive = *in.Active
		}
		if u.Role == RoleAdministrator && u.Active && (proposedRole != RoleAdministrator || !proposedActive) {
			activeAdmins := 0
			for j := range s.state.Users {
				candidate := s.state.Users[j]
				if candidate.Active && candidate.Role == RoleAdministrator {
					activeAdmins++
				}
			}
			if activeAdmins <= 1 {
				return PublicUser{}, errors.New("cannot remove the last active administrator")
			}
		}
		if in.Role != nil {
			u.Role = proposedRole
			if u.Role == RoleAdministrator || u.Role == RoleCommissioningEngineer {
				u.MFARequired = true
			}
			u.SessionVersion++
		}
		if in.DisplayName != nil {
			u.DisplayName = strings.TrimSpace(*in.DisplayName)
			if u.DisplayName == "" {
				return PublicUser{}, errors.New("display name is required")
			}
		}
		if in.SiteScopes != nil {
			if err := s.validateSiteScopesLocked(*in.SiteScopes); err != nil {
				return PublicUser{}, err
			}
			u.SiteScopes = uniqueStrings(*in.SiteScopes)
		}
		if in.Active != nil {
			u.Active = proposedActive
			if !u.Active {
				u.SessionVersion++
			}
		}
		if in.MFARequired != nil {
			u.MFARequired = *in.MFARequired
		}
		if u.Role == RoleAdministrator || u.Role == RoleCommissioningEngineer {
			u.MFARequired = true
		}
		u.UpdatedAt = time.Now().UTC()
		s.auditLocked(actor, "admin.user.update", "user", u.ID, "success", map[string]string{"role": string(u.Role), "active": strconv.FormatBool(u.Active)})
		if err := s.saveLocked(); err != nil {
			return PublicUser{}, err
		}
		return u.Public(), nil
	}
	return PublicUser{}, errors.New("user not found")
}

func (s *Store) ResetPassword(actor User, id string) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := range s.state.Users {
		u := &s.state.Users[i]
		if u.ID != id {
			continue
		}
		pwd := randomPassword()
		hash, err := bcrypt.GenerateFromPassword([]byte(pwd), 12)
		if err != nil {
			return "", err
		}
		u.PasswordHash = string(hash)
		u.MustChangePassword = true
		u.SessionVersion++
		u.UpdatedAt = time.Now().UTC()
		s.auditLocked(actor, "admin.user.reset_password", "user", u.ID, "success", nil)
		if err := s.saveLocked(); err != nil {
			return "", err
		}
		return pwd, nil
	}
	return "", errors.New("user not found")
}
func (s *Store) ChangePassword(actor User, oldPassword, newPassword string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(newPassword) < 12 {
		return errors.New("new password must have at least 12 characters")
	}
	for i := range s.state.Users {
		u := &s.state.Users[i]
		if u.ID != actor.ID {
			continue
		}
		if bcrypt.CompareHashAndPassword([]byte(u.PasswordHash), []byte(oldPassword)) != nil {
			return errors.New("current password is invalid")
		}
		hash, err := bcrypt.GenerateFromPassword([]byte(newPassword), 12)
		if err != nil {
			return err
		}
		u.PasswordHash = string(hash)
		u.MustChangePassword = false
		u.SessionVersion++
		u.UpdatedAt = time.Now().UTC()
		s.auditLocked(*u, "auth.password.change", "user", u.ID, "success", nil)
		return s.saveLocked()
	}
	return errors.New("user not found")
}
func (s *Store) RevokeSessions(actor User, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := range s.state.Users {
		u := &s.state.Users[i]
		if u.ID == id {
			u.SessionVersion++
			u.UpdatedAt = time.Now().UTC()
			s.auditLocked(actor, "admin.user.revoke_sessions", "user", u.ID, "success", nil)
			return s.saveLocked()
		}
	}
	return errors.New("user not found")
}

func (s *Store) RevokeOwnSessions(actor User) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := range s.state.Users {
		u := &s.state.Users[i]
		if u.ID != actor.ID {
			continue
		}
		u.SessionVersion++
		u.UpdatedAt = time.Now().UTC()
		s.auditLocked(actor, "auth.logout_all", "user", u.ID, "success", nil)
		return s.saveLocked()
	}
	return errors.New("user not found")
}

func uniqueStrings(in []string) []string {
	seen := map[string]struct{}{}
	out := []string{}
	for _, v := range in {
		v = strings.TrimSpace(v)
		if v == "" {
			continue
		}
		if _, ok := seen[v]; ok {
			continue
		}
		seen[v] = struct{}{}
		out = append(out, v)
	}
	sort.Strings(out)
	return out
}

func (s *Store) ListSites() []Site {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := append([]Site(nil), s.state.Sites...)
	sort.Slice(out, func(i, j int) bool { return out[i].Code < out[j].Code })
	return out
}

type SiteInput struct {
	ID       string `json:"id,omitempty"`
	Code     string `json:"code"`
	Name     string `json:"name"`
	TimeZone string `json:"timeZone"`
	Active   *bool  `json:"active"`
}

func validSiteID(v string) bool {
	if len(v) < 1 || len(v) > 80 {
		return false
	}
	for _, r := range v {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '.' || r == '_' || r == '-' {
			continue
		}
		return false
	}
	return true
}

func (s *Store) CreateSite(actor User, in SiteInput) (Site, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	in.ID = strings.TrimSpace(in.ID)
	in.Code = strings.TrimSpace(in.Code)
	in.Name = strings.TrimSpace(in.Name)
	if in.Code == "" || in.Name == "" {
		return Site{}, errors.New("code and name are required")
	}
	if in.ID != "" && !validSiteID(in.ID) {
		return Site{}, errors.New("invalid site integration id")
	}
	for _, x := range s.state.Sites {
		if strings.EqualFold(x.Code, in.Code) {
			return Site{}, errors.New("site code already exists")
		}
		if in.ID != "" && x.ID == in.ID {
			return Site{}, errors.New("site integration id already exists")
		}
	}
	now := time.Now().UTC()
	active := true
	if in.Active != nil {
		active = *in.Active
	}
	siteID := in.ID
	if siteID == "" {
		siteID = newID("site")
	}
	site := Site{ID: siteID, Code: in.Code, Name: in.Name, TimeZone: strings.TrimSpace(in.TimeZone), Active: active, CreatedAt: now, UpdatedAt: now}
	if site.TimeZone == "" {
		site.TimeZone = "UTC"
	}
	if _, err := time.LoadLocation(site.TimeZone); err != nil {
		return Site{}, errors.New("invalid time zone")
	}
	s.state.Sites = append(s.state.Sites, site)
	s.auditLocked(actor, "admin.site.create", "site", site.ID, "success", nil)
	if err := s.saveLocked(); err != nil {
		return Site{}, err
	}
	return site, nil
}
func (s *Store) UpdateSite(actor User, id string, in SiteInput) (Site, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := range s.state.Sites {
		x := &s.state.Sites[i]
		if x.ID != id {
			continue
		}
		if strings.TrimSpace(in.Code) != "" {
			candidateCode := strings.TrimSpace(in.Code)
			for j := range s.state.Sites {
				other := s.state.Sites[j]
				if other.ID != x.ID && strings.EqualFold(other.Code, candidateCode) {
					return Site{}, errors.New("site code already exists")
				}
			}
			x.Code = candidateCode
		}
		if strings.TrimSpace(in.Name) != "" {
			x.Name = strings.TrimSpace(in.Name)
		}
		if strings.TrimSpace(in.TimeZone) != "" {
			if _, err := time.LoadLocation(in.TimeZone); err != nil {
				return Site{}, errors.New("invalid time zone")
			}
			x.TimeZone = in.TimeZone
		}
		if in.Active != nil {
			x.Active = *in.Active
		}
		x.UpdatedAt = time.Now().UTC()
		s.auditLocked(actor, "admin.site.update", "site", x.ID, "success", nil)
		if err := s.saveLocked(); err != nil {
			return Site{}, err
		}
		return *x, nil
	}
	return Site{}, errors.New("site not found")
}

func (s *Store) OperationalGeneratorIDs(actor User) map[string]struct{} {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := map[string]struct{}{}
	for _, c := range s.state.Commissionings {
		if c.Lifecycle != "COMMISSIONED" || strings.TrimSpace(c.MonitorGeneratorID) == "" || !allowedSite(actor, c.SiteID) {
			continue
		}
		out[c.MonitorGeneratorID] = struct{}{}
	}
	return out
}

func (s *Store) ListCommissionings(actor User) []Commissioning {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := []Commissioning{}
	for _, c := range s.state.Commissionings {
		if allowedSite(actor, c.SiteID) {
			out = append(out, c)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].UpdatedAt.After(out[j].UpdatedAt) })
	return out
}
func allowedSite(u User, site string) bool {
	for _, s := range u.SiteScopes {
		if s == "*" || s == site {
			return true
		}
	}
	return false
}

type CommissioningInput struct {
	Tag                string             `json:"tag"`
	Name               string             `json:"name"`
	SiteID             string             `json:"siteId"`
	Asset              AssetSpec          `json:"asset"`
	Controller         ControllerIdentity `json:"controller"`
	ECU                ECUIdentity        `json:"ecu"`
	Transport          TransportConfig    `json:"transport"`
	RapidPlanHash      string             `json:"rapidPlanHash"`
	RapidPlanVersion   string             `json:"rapidPlanVersion"`
	MonitorGeneratorID string             `json:"monitorGeneratorId"`
	ChangeReason       string             `json:"changeReason"`
}

func (s *Store) CreateCommissioning(actor User, in CommissioningInput) (Commissioning, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	in.Tag = strings.TrimSpace(in.Tag)
	in.Name = strings.TrimSpace(in.Name)
	if in.Tag == "" || in.Name == "" || strings.TrimSpace(in.SiteID) == "" {
		return Commissioning{}, errors.New("tag, name and siteId are required")
	}
	if !s.siteExistsLocked(in.SiteID) {
		return Commissioning{}, errors.New("site not found")
	}
	if !allowedSite(actor, in.SiteID) {
		return Commissioning{}, errors.New("site outside user scope")
	}
	now := time.Now().UTC()
	gates := map[string]GateResult{}
	for _, stage := range CommissioningStages {
		gates[stage] = GateResult{Status: GatePending, UpdatedAt: now}
	}
	c := Commissioning{ID: newID("com"), Tag: in.Tag, Name: in.Name, SiteID: in.SiteID, Lifecycle: "DRAFT", Stage: "IDENTITY", Asset: in.Asset, Controller: in.Controller, ECU: in.ECU, Transport: in.Transport, RapidPlanHash: in.RapidPlanHash, RapidPlanVersion: in.RapidPlanVersion, MonitorGeneratorID: strings.TrimSpace(in.MonitorGeneratorID), ChangeReason: strings.TrimSpace(in.ChangeReason), Revision: 1, Gates: gates, CreatedBy: actor.ID, CreatedAt: now, UpdatedAt: now}
	s.state.Commissionings = append(s.state.Commissionings, c)
	s.auditLocked(actor, "commissioning.create", "commissioning", c.ID, "success", map[string]string{"siteId": c.SiteID})
	if err := s.saveLocked(); err != nil {
		return Commissioning{}, err
	}
	return c, nil
}
func (s *Store) UpdateCommissioning(actor User, id string, in CommissioningInput) (Commissioning, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := range s.state.Commissionings {
		c := &s.state.Commissionings[i]
		if c.ID != id {
			continue
		}
		if !allowedSite(actor, c.SiteID) {
			return Commissioning{}, errors.New("site outside user scope")
		}
		if c.Lifecycle == "COMMISSIONED" || c.Lifecycle == "RETIRED" {
			return Commissioning{}, errors.New("commissioned/retired assets require a new change commissioning")
		}
		if strings.TrimSpace(in.Tag) != "" {
			c.Tag = strings.TrimSpace(in.Tag)
		}
		if strings.TrimSpace(in.Name) != "" {
			c.Name = strings.TrimSpace(in.Name)
		}
		if strings.TrimSpace(in.SiteID) != "" {
			if !s.siteExistsLocked(in.SiteID) {
				return Commissioning{}, errors.New("site not found")
			}
			if !allowedSite(actor, in.SiteID) {
				return Commissioning{}, errors.New("site outside user scope")
			}
			c.SiteID = in.SiteID
		}
		c.Asset = in.Asset
		c.Controller = in.Controller
		c.ECU = in.ECU
		c.Transport = in.Transport
		c.RapidPlanHash = in.RapidPlanHash
		c.RapidPlanVersion = in.RapidPlanVersion
		if strings.TrimSpace(in.MonitorGeneratorID) != "" {
			c.MonitorGeneratorID = strings.TrimSpace(in.MonitorGeneratorID)
		}
		if strings.TrimSpace(in.ChangeReason) != "" {
			c.ChangeReason = strings.TrimSpace(in.ChangeReason)
		}
		c.UpdatedAt = time.Now().UTC()
		s.auditLocked(actor, "commissioning.update", "commissioning", c.ID, "success", nil)
		if err := s.saveLocked(); err != nil {
			return Commissioning{}, err
		}
		return *c, nil
	}
	return Commissioning{}, errors.New("commissioning not found")
}

type GateInput struct {
	Status   GateStatus `json:"status"`
	Evidence string     `json:"evidence"`
}

func (s *Store) SetGate(actor User, id, stage string, in GateInput) (Commissioning, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if in.Status != GatePass && in.Status != GateFail && in.Status != GateBlocked && in.Status != GatePending {
		return Commissioning{}, errors.New("invalid gate status")
	}
	for i := range s.state.Commissionings {
		c := &s.state.Commissionings[i]
		if c.ID != id {
			continue
		}
		if !allowedSite(actor, c.SiteID) {
			return Commissioning{}, errors.New("site outside user scope")
		}
		found := false
		for _, st := range CommissioningStages {
			if st == stage {
				found = true
				break
			}
		}
		if !found {
			return Commissioning{}, errors.New("invalid commissioning stage")
		}
		now := time.Now().UTC()
		c.Gates[stage] = GateResult{Status: in.Status, Evidence: strings.TrimSpace(in.Evidence), UpdatedAt: now, UpdatedBy: actor.ID}
		c.Lifecycle = "COMMISSIONING"
		if in.Status == GatePass && c.Stage == stage {
			c.Stage = nextStage(stage)
		} else {
			c.Stage = stage
		}
		c.UpdatedAt = now
		s.auditLocked(actor, "commissioning.gate", "commissioning", c.ID, "success", map[string]string{"stage": stage, "status": string(in.Status)})
		if err := s.saveLocked(); err != nil {
			return Commissioning{}, err
		}
		return *c, nil
	}
	return Commissioning{}, errors.New("commissioning not found")
}
func (s *Store) PromoteCommissioning(actor User, id string) (Commissioning, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := range s.state.Commissionings {
		c := &s.state.Commissionings[i]
		if c.ID != id {
			continue
		}
		if !allowedSite(actor, c.SiteID) {
			return Commissioning{}, errors.New("site outside user scope")
		}
		for _, stage := range CommissioningStages {
			if c.Gates[stage].Status != GatePass {
				return Commissioning{}, fmt.Errorf("gate %s is not PASS", stage)
			}
		}
		now := time.Now().UTC()
		if c.ParentCommissioningID != "" {
			foundParent := false
			for j := range s.state.Commissionings {
				parent := &s.state.Commissionings[j]
				if parent.ID != c.ParentCommissioningID {
					continue
				}
				if parent.SiteID != c.SiteID {
					return Commissioning{}, errors.New("parent commissioning site mismatch")
				}
				if parent.Lifecycle != "COMMISSIONED" && parent.Lifecycle != "SUSPENDED" {
					return Commissioning{}, errors.New("parent commissioning is not an active revision")
				}
				parent.Lifecycle = "SUPERSEDED"
				parent.UpdatedAt = now
				s.auditLocked(actor, "commissioning.supersede", "commissioning", parent.ID, "success", map[string]string{"replacement": c.ID})
				foundParent = true
				break
			}
			if !foundParent {
				return Commissioning{}, errors.New("parent commissioning not found")
			}
		}
		c.Lifecycle = "COMMISSIONED"
		c.Stage = "EVIDENCE"
		c.CommissionedAt = &now
		c.UpdatedAt = now
		s.auditLocked(actor, "commissioning.promote", "commissioning", c.ID, "success", nil)
		if err := s.saveLocked(); err != nil {
			return Commissioning{}, err
		}
		return *c, nil
	}
	return Commissioning{}, errors.New("commissioning not found")
}
func (s *Store) SetLifecycle(actor User, id, lifecycle string) (Commissioning, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if lifecycle != "SUSPENDED" && lifecycle != "RETIRED" {
		return Commissioning{}, errors.New("invalid lifecycle transition")
	}
	for i := range s.state.Commissionings {
		c := &s.state.Commissionings[i]
		if c.ID == id {
			if !allowedSite(actor, c.SiteID) {
				return Commissioning{}, errors.New("site outside user scope")
			}
			c.Lifecycle = lifecycle
			c.UpdatedAt = time.Now().UTC()
			s.auditLocked(actor, "commissioning.lifecycle", "commissioning", c.ID, "success", map[string]string{"lifecycle": lifecycle})
			if err := s.saveLocked(); err != nil {
				return Commissioning{}, err
			}
			return *c, nil
		}
	}
	return Commissioning{}, errors.New("commissioning not found")
}

func (s *Store) EnableMFA(actor User, secret string) (PublicUser, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if strings.TrimSpace(secret) == "" {
		return PublicUser{}, errors.New("MFA secret is required")
	}
	for i := range s.state.Users {
		u := &s.state.Users[i]
		if u.ID != actor.ID {
			continue
		}
		u.MFASecret = secret
		u.MFAEnabled = true
		u.MFARequired = true
		u.UpdatedAt = time.Now().UTC()
		s.auditLocked(*u, "auth.mfa.enable", "user", u.ID, "success", nil)
		if err := s.saveLocked(); err != nil {
			return PublicUser{}, err
		}
		return u.Public(), nil
	}
	return PublicUser{}, errors.New("user not found")
}

func (s *Store) ResetMFA(actor User, id string) (PublicUser, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := range s.state.Users {
		u := &s.state.Users[i]
		if u.ID != id {
			continue
		}
		u.MFAEnabled = false
		u.MFASecret = ""
		if u.Role == RoleAdministrator || u.Role == RoleCommissioningEngineer {
			u.MFARequired = true
		}
		u.SessionVersion++
		u.UpdatedAt = time.Now().UTC()
		s.auditLocked(actor, "admin.user.mfa.reset", "user", u.ID, "success", nil)
		if err := s.saveLocked(); err != nil {
			return PublicUser{}, err
		}
		return u.Public(), nil
	}
	return PublicUser{}, errors.New("user not found")
}

func (s *Store) DisableMFA(actor User, id string) (PublicUser, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := range s.state.Users {
		u := &s.state.Users[i]
		if u.ID != id {
			continue
		}
		if u.Role == RoleAdministrator || u.Role == RoleCommissioningEngineer {
			return PublicUser{}, errors.New("MFA is mandatory for this role")
		}
		u.MFAEnabled = false
		u.MFASecret = ""
		u.MFARequired = false
		u.SessionVersion++
		u.UpdatedAt = time.Now().UTC()
		s.auditLocked(actor, "admin.user.mfa.disable", "user", u.ID, "success", nil)
		if err := s.saveLocked(); err != nil {
			return PublicUser{}, err
		}
		return u.Public(), nil
	}
	return PublicUser{}, errors.New("user not found")
}

func (s *Store) Audit(limit int) []AuditEvent {
	s.mu.Lock()
	defer s.mu.Unlock()
	if limit <= 0 || limit > 1000 {
		limit = 200
	}
	n := len(s.state.Audit)
	start := n - limit
	if start < 0 {
		start = 0
	}
	out := append([]AuditEvent(nil), s.state.Audit[start:]...)
	sort.Slice(out, func(i, j int) bool { return out[i].At.After(out[j].At) })
	return out
}
func (s *Store) auditLocked(actor User, action, objType, objID, result string, details map[string]string) {
	s.state.Audit = append(s.state.Audit, AuditEvent{ID: newID("aud"), At: time.Now().UTC(), ActorUserID: actor.ID, ActorUsername: actor.Username, Action: action, ObjectType: objType, ObjectID: objID, Result: result, Details: details, CorrelationID: newID("corr"), Source: "rc-admin"})
	if len(s.state.Audit) > 5000 {
		s.state.Audit = s.state.Audit[len(s.state.Audit)-5000:]
	}
}

func defaultPreferences(settings SystemSettings) UserPreferences {
	return UserPreferences{Language: settings.DefaultLanguage, TimeZone: settings.DefaultTimeZone, UnitSystem: "metric", FleetView: settings.DefaultFleetView}
}

func validatePreferences(p UserPreferences) error {
	if p.Language != "pt-BR" {
		return errors.New("unsupported language")
	}
	if _, err := time.LoadLocation(p.TimeZone); err != nil {
		return errors.New("invalid time zone")
	}
	if p.UnitSystem != "metric" {
		return errors.New("invalid unit system")
	}
	if p.FleetView != "vertical" && p.FleetView != "compact" && p.FleetView != "list" {
		return errors.New("invalid fleet view")
	}
	return nil
}

func (s *Store) Preferences(actor User) UserPreferences {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, u := range s.state.Users {
		if u.ID == actor.ID {
			p := u.Preferences
			if p.Language == "" {
				p = defaultPreferences(s.state.Settings)
			}
			return p
		}
	}
	return defaultPreferences(s.state.Settings)
}
func (s *Store) UpdatePreferences(actor User, p UserPreferences) (UserPreferences, error) {
	if err := validatePreferences(p); err != nil {
		return UserPreferences{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := range s.state.Users {
		u := &s.state.Users[i]
		if u.ID == actor.ID {
			u.Preferences = p
			u.UpdatedAt = time.Now().UTC()
			s.auditLocked(*u, "auth.preferences.update", "user", u.ID, "success", nil)
			return p, s.saveLocked()
		}
	}
	return UserPreferences{}, errors.New("user not found")
}
func (s *Store) Settings() SystemSettings { s.mu.Lock(); defer s.mu.Unlock(); return s.state.Settings }
func (s *Store) UpdateSettings(actor User, in SystemSettings) (SystemSettings, error) {
	if in.DefaultLanguage != "pt-BR" {
		return SystemSettings{}, errors.New("unsupported language")
	}
	if _, err := time.LoadLocation(in.DefaultTimeZone); err != nil {
		return SystemSettings{}, errors.New("invalid time zone")
	}
	if in.DefaultFleetView != "vertical" && in.DefaultFleetView != "compact" && in.DefaultFleetView != "list" {
		return SystemSettings{}, errors.New("invalid fleet view")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.state.Settings = in
	s.auditLocked(actor, "admin.settings.update", "settings", "global", "success", nil)
	return in, s.saveLocked()
}
func (s *Store) ProfileStates() []ProfileLifecycleRecord {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]ProfileLifecycleRecord, 0, len(s.state.ProfileStates))
	for _, v := range s.state.ProfileStates {
		out = append(out, v)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ProfileID < out[j].ProfileID })
	return out
}
func validProfileStatus(status string) bool {
	switch status {
	case "DRAFT", "LAB", "HIL_VALIDATED", "HOMOLOGATED", "DEPRECATED":
		return true
	}
	return false
}
func (s *Store) SetProfileState(actor User, profileID, status, version, evidence string) (ProfileLifecycleRecord, error) {
	profileID = strings.TrimSpace(profileID)
	status = strings.TrimSpace(status)
	evidence = strings.TrimSpace(evidence)
	if profileID == "" || !validProfileStatus(status) {
		return ProfileLifecycleRecord{}, errors.New("invalid profile state")
	}
	if (status == "HIL_VALIDATED" || status == "HOMOLOGATED") && evidence == "" {
		return ProfileLifecycleRecord{}, errors.New("HIL/homologation status requires evidence")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	rec := ProfileLifecycleRecord{ProfileID: profileID, Status: status, Version: strings.TrimSpace(version), Evidence: evidence, UpdatedAt: time.Now().UTC(), UpdatedBy: actor.ID}
	s.state.ProfileStates[profileID] = rec
	s.auditLocked(actor, "engineering.profile.status", "profile", profileID, "success", map[string]string{"status": status, "version": rec.Version})
	return rec, s.saveLocked()
}
func (s *Store) CommissioningByID(actor User, id string) (Commissioning, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, c := range s.state.Commissionings {
		if c.ID == id && allowedSite(actor, c.SiteID) {
			return c, true
		}
	}
	return Commissioning{}, false
}
func (s *Store) CreateChangeCommissioning(actor User, id, reason string) (Commissioning, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	reason = strings.TrimSpace(reason)
	if reason == "" {
		return Commissioning{}, errors.New("change reason is required")
	}
	for i := range s.state.Commissionings {
		parent := s.state.Commissionings[i]
		if parent.ID != id {
			continue
		}
		if !allowedSite(actor, parent.SiteID) {
			return Commissioning{}, errors.New("site outside user scope")
		}
		if parent.Lifecycle != "COMMISSIONED" && parent.Lifecycle != "SUSPENDED" {
			return Commissioning{}, errors.New("change commissioning requires commissioned or suspended asset")
		}
		for _, existing := range s.state.Commissionings {
			if existing.ParentCommissioningID == parent.ID && existing.Lifecycle == "DRAFT" {
				return Commissioning{}, errors.New("an open change commissioning already exists for this asset")
			}
		}
		now := time.Now().UTC()
		gates := map[string]GateResult{}
		for _, stage := range CommissioningStages {
			gates[stage] = GateResult{Status: GatePending, UpdatedAt: now}
		}
		child := parent
		child.ID = newID("com")
		child.ParentCommissioningID = parent.ID
		child.Lifecycle = "DRAFT"
		child.Stage = "IDENTITY"
		child.Gates = gates
		child.CreatedBy = actor.ID
		child.CreatedAt = now
		child.UpdatedAt = now
		child.CommissionedAt = nil
		child.Revision = parent.Revision + 1
		if child.Revision < 2 {
			child.Revision = 2
		}
		child.ChangeReason = reason
		s.state.Commissionings = append(s.state.Commissionings, child)
		s.auditLocked(actor, "commissioning.change.create", "commissioning", child.ID, "success", map[string]string{"parent": parent.ID, "reason": child.ChangeReason})
		if err := s.saveLocked(); err != nil {
			return Commissioning{}, err
		}
		return child, nil
	}
	return Commissioning{}, errors.New("commissioning not found")
}
