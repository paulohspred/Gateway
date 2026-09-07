package control

import "time"

type Role string
type Permission string

const (
	RoleViewer                Role = "viewer"
	RoleOperator              Role = "operator"
	RoleTechnician            Role = "technician"
	RoleCommissioningEngineer Role = "commissioning_engineer"
	RoleAdministrator         Role = "administrator"
	RoleAuditor               Role = "auditor"
)

const (
	PermFleetRead            Permission = "fleet.read"
	PermAlarmRead            Permission = "alarms.read"
	PermEventRead            Permission = "events.read"
	PermDiagnosticsRead      Permission = "diagnostics.read"
	PermCommissioningRead    Permission = "commissioning.read"
	PermCommissioningWrite   Permission = "commissioning.write"
	PermCommissioningPromote Permission = "commissioning.promote"
	PermUsersRead            Permission = "users.read"
	PermUsersWrite           Permission = "users.write"
	PermSitesRead            Permission = "sites.read"
	PermSitesWrite           Permission = "sites.write"
	PermAuditRead            Permission = "audit.read"
	PermSystemRead           Permission = "system.read"
	PermSettingsRead         Permission = "settings.read"
	PermSettingsWrite        Permission = "settings.write"
)

var rolePermissions = map[Role][]Permission{
	RoleViewer:                {PermFleetRead, PermAlarmRead, PermEventRead},
	RoleOperator:              {PermFleetRead, PermAlarmRead, PermEventRead, PermDiagnosticsRead},
	RoleTechnician:            {PermFleetRead, PermAlarmRead, PermEventRead, PermDiagnosticsRead, PermCommissioningRead, PermSystemRead, PermSettingsRead},
	RoleCommissioningEngineer: {PermFleetRead, PermAlarmRead, PermEventRead, PermDiagnosticsRead, PermCommissioningRead, PermCommissioningWrite, PermCommissioningPromote, PermSitesRead, PermSystemRead, PermSettingsRead},
	RoleAdministrator:         {PermFleetRead, PermAlarmRead, PermEventRead, PermDiagnosticsRead, PermCommissioningRead, PermCommissioningWrite, PermCommissioningPromote, PermUsersRead, PermUsersWrite, PermSitesRead, PermSitesWrite, PermAuditRead, PermSystemRead, PermSettingsRead, PermSettingsWrite},
	RoleAuditor:               {PermFleetRead, PermAlarmRead, PermEventRead, PermCommissioningRead, PermAuditRead, PermSystemRead, PermSettingsRead},
}

func PermissionsForRole(role Role) []Permission {
	p := rolePermissions[role]
	out := make([]Permission, len(p))
	copy(out, p)
	return out
}

func ValidRole(role Role) bool { _, ok := rolePermissions[role]; return ok }
func HasPermission(role Role, permission Permission) bool {
	for _, p := range rolePermissions[role] {
		if p == permission {
			return true
		}
	}
	return false
}

type UserPreferences struct {
	Language   string `json:"language"`
	TimeZone   string `json:"timeZone"`
	UnitSystem string `json:"unitSystem"`
	FleetView  string `json:"fleetView"`
}

type SystemSettings struct {
	DefaultLanguage  string `json:"defaultLanguage"`
	DefaultTimeZone  string `json:"defaultTimeZone"`
	DefaultFleetView string `json:"defaultFleetView"`
}

type ProfileLifecycleRecord struct {
	ProfileID string    `json:"profileId"`
	Status    string    `json:"status"`
	Version   string    `json:"version,omitempty"`
	Evidence  string    `json:"evidence,omitempty"`
	UpdatedAt time.Time `json:"updatedAt"`
	UpdatedBy string    `json:"updatedBy"`
}

type User struct {
	ID                 string          `json:"id"`
	Username           string          `json:"username"`
	DisplayName        string          `json:"displayName"`
	Role               Role            `json:"role"`
	SiteScopes         []string        `json:"siteScopes"`
	Active             bool            `json:"active"`
	PasswordHash       string          `json:"passwordHash"`
	MustChangePassword bool            `json:"mustChangePassword"`
	MFARequired        bool            `json:"mfaRequired"`
	MFAEnabled         bool            `json:"mfaEnabled"`
	MFASecret          string          `json:"mfaSecret"`
	CreatedAt          time.Time       `json:"createdAt"`
	UpdatedAt          time.Time       `json:"updatedAt"`
	LastLoginAt        *time.Time      `json:"lastLoginAt,omitempty"`
	FailedLogins       int             `json:"failedLogins"`
	LockedUntil        *time.Time      `json:"lockedUntil,omitempty"`
	SessionVersion     uint64          `json:"sessionVersion"`
	Preferences        UserPreferences `json:"preferences"`
}

type PublicUser struct {
	ID                 string          `json:"id"`
	Username           string          `json:"username"`
	DisplayName        string          `json:"displayName"`
	Role               Role            `json:"role"`
	Permissions        []Permission    `json:"permissions"`
	SiteScopes         []string        `json:"siteScopes"`
	Active             bool            `json:"active"`
	MustChangePassword bool            `json:"mustChangePassword"`
	MFARequired        bool            `json:"mfaRequired"`
	MFAEnabled         bool            `json:"mfaEnabled"`
	CreatedAt          time.Time       `json:"createdAt"`
	UpdatedAt          time.Time       `json:"updatedAt"`
	LastLoginAt        *time.Time      `json:"lastLoginAt,omitempty"`
	LockedUntil        *time.Time      `json:"lockedUntil,omitempty"`
	Preferences        UserPreferences `json:"preferences"`
}

func (u User) Public() PublicUser {
	return PublicUser{ID: u.ID, Username: u.Username, DisplayName: u.DisplayName, Role: u.Role, Permissions: PermissionsForRole(u.Role), SiteScopes: append([]string(nil), u.SiteScopes...), Active: u.Active, MustChangePassword: u.MustChangePassword, MFARequired: u.MFARequired, MFAEnabled: u.MFAEnabled, CreatedAt: u.CreatedAt, UpdatedAt: u.UpdatedAt, LastLoginAt: u.LastLoginAt, LockedUntil: u.LockedUntil, Preferences: u.Preferences}
}

type Site struct {
	ID        string    `json:"id"`
	Code      string    `json:"code"`
	Name      string    `json:"name"`
	TimeZone  string    `json:"timeZone"`
	Active    bool      `json:"active"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

type GateStatus string

const (
	GatePending GateStatus = "PENDING"
	GatePass    GateStatus = "PASS"
	GateFail    GateStatus = "FAIL"
	GateBlocked GateStatus = "BLOCKED"
)

type GateResult struct {
	Status    GateStatus `json:"status"`
	Evidence  string     `json:"evidence,omitempty"`
	UpdatedAt time.Time  `json:"updatedAt"`
	UpdatedBy string     `json:"updatedBy"`
}

type AssetSpec struct {
	RatedPowerKW     *float64 `json:"ratedPowerKw,omitempty"`
	NominalVoltage   *float64 `json:"nominalVoltage,omitempty"`
	NominalFrequency *float64 `json:"nominalFrequency,omitempty"`
	PhaseCount       *int     `json:"phaseCount,omitempty"`
	NominalRPM       *float64 `json:"nominalRpm,omitempty"`
}

type ControllerIdentity struct {
	Manufacturer   string `json:"manufacturer,omitempty"`
	Model          string `json:"model,omitempty"`
	Firmware       string `json:"firmware,omitempty"`
	Hardware       string `json:"hardware,omitempty"`
	SerialNumber   string `json:"serialNumber,omitempty"`
	ProfileID      string `json:"profileId,omitempty"`
	ProfileVersion string `json:"profileVersion,omitempty"`
	ProfileStatus  string `json:"profileStatus,omitempty"`
}

type ECUIdentity struct {
	Manufacturer string `json:"manufacturer,omitempty"`
	Model        string `json:"model,omitempty"`
	SerialNumber string `json:"serialNumber,omitempty"`
	Protocol     string `json:"protocol,omitempty"`
	J1939        bool   `json:"j1939"`
}

type TransportConfig struct {
	Kind          string `json:"kind,omitempty"`
	Gateway       string `json:"gateway,omitempty"`
	Host          string `json:"host,omitempty"`
	Port          int    `json:"port,omitempty"`
	DeviceAddress string `json:"deviceAddress,omitempty"`
	Notes         string `json:"notes,omitempty"`
}

type Commissioning struct {
	ID                    string                `json:"id"`
	Tag                   string                `json:"tag"`
	Name                  string                `json:"name"`
	SiteID                string                `json:"siteId"`
	Lifecycle             string                `json:"lifecycle"`
	Stage                 string                `json:"stage"`
	Asset                 AssetSpec             `json:"asset"`
	Controller            ControllerIdentity    `json:"controller"`
	ECU                   ECUIdentity           `json:"ecu"`
	Transport             TransportConfig       `json:"transport"`
	RapidPlanHash         string                `json:"rapidPlanHash,omitempty"`
	RapidPlanVersion      string                `json:"rapidPlanVersion,omitempty"`
	Gates                 map[string]GateResult `json:"gates"`
	CreatedBy             string                `json:"createdBy"`
	CreatedAt             time.Time             `json:"createdAt"`
	UpdatedAt             time.Time             `json:"updatedAt"`
	CommissionedAt        *time.Time            `json:"commissionedAt,omitempty"`
	MonitorGeneratorID    string                `json:"monitorGeneratorId,omitempty"`
	ParentCommissioningID string                `json:"parentCommissioningId,omitempty"`
	Revision              int                   `json:"revision"`
	ChangeReason          string                `json:"changeReason,omitempty"`
}

var CommissioningStages = []string{"IDENTITY", "TRANSPORT", "CONTROLLER", "PROFILE", "RAPID_PLAN", "RAPID_APPLY", "TELEMETRY_VALIDATION", "EVIDENCE"}

func nextStage(current string) string {
	for i, s := range CommissioningStages {
		if s == current && i+1 < len(CommissioningStages) {
			return CommissioningStages[i+1]
		}
	}
	return current
}

type AuditEvent struct {
	ID            string            `json:"id"`
	At            time.Time         `json:"at"`
	ActorUserID   string            `json:"actorUserId,omitempty"`
	ActorUsername string            `json:"actorUsername,omitempty"`
	Action        string            `json:"action"`
	ObjectType    string            `json:"objectType"`
	ObjectID      string            `json:"objectId,omitempty"`
	Result        string            `json:"result"`
	Details       map[string]string `json:"details,omitempty"`
	CorrelationID string            `json:"correlationId"`
	Source        string            `json:"source"`
}
