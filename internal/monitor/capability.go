package monitor

// MetricCapability describes whether a canonical metric belongs to the
// controller profile. It deliberately contains no physical register/address.
type MetricCapability struct {
	Key                MetricKey `json:"key"`
	DisplayName        string    `json:"displayName"`
	Kind               ValueKind `json:"kind"`
	Unit               string    `json:"unit,omitempty"`
	Required           bool      `json:"required"`
	StaleAfterSeconds  int       `json:"staleAfterSeconds"`
	RapidChannelNumber int       `json:"rapidChannelNumber,omitempty"`
}

// GeneratorCapabilities is the read-only semantic contract consumed by the
// HMI. RemoteControl remains false until the separate command-plane gates are
// implemented and validated.
type GeneratorCapabilities struct {
	GeneratorID   string             `json:"generatorId"`
	ProfileID     string             `json:"profileId"`
	ProfileStatus string             `json:"profileStatus"`
	Telemetry     bool               `json:"telemetry"`
	Alarms        bool               `json:"alarms"`
	Events        bool               `json:"events"`
	Maintenance   bool               `json:"maintenance"`
	RemoteControl bool               `json:"remoteControl"`
	Metrics       []MetricCapability `json:"metrics"`
}
