package model

import "time"

type OwnershipScope struct {
	TenantID  string `json:"tenant_id,omitempty"`
	ProjectID string `json:"project_id,omitempty"`
}

type CloudContextFilter struct {
	TenantID  string
	ProjectID string
	VMID      string
}

type ConnectionViewFilter struct {
	TenantID  string
	ProjectID string
	VMID      string
	TimeRange time.Duration
}

type CloudProviderStatus struct {
	Name         string     `json:"name"`
	Mode         string     `json:"mode"`
	Connected    bool       `json:"connected"`
	Capabilities []string   `json:"capabilities"`
	LastSyncAt   *time.Time `json:"last_sync_at,omitempty"`
	Error        string     `json:"error,omitempty"`
}

type CloudHost struct {
	ID           string      `json:"id"`
	ProviderID   string      `json:"provider_id,omitempty"`
	Name         string      `json:"name"`
	Hostname     string      `json:"hostname,omitempty"`
	TenantID     string      `json:"tenant_id,omitempty"`
	ProjectID    string      `json:"project_id,omitempty"`
	Region       string      `json:"region,omitempty"`
	Zone         string      `json:"zone,omitempty"`
	Role         string      `json:"role,omitempty"`
	Type         string      `json:"type,omitempty"`
	Environment  string      `json:"environment,omitempty"`
	Owner        string      `json:"owner,omitempty"`
	PrivateIP    string      `json:"private_ip,omitempty"`
	PublicIP     string      `json:"public_ip,omitempty"`
	NetworkID    string      `json:"network_id,omitempty"`
	SubnetID     string      `json:"subnet_id,omitempty"`
	Status       string      `json:"status"`
	LastSeen     time.Time   `json:"last_seen"`
	Interfaces   []Interface `json:"interfaces,omitempty"`
	ObservedOnly bool        `json:"observed_only"`
}

type PublicIP struct {
	ID         string     `json:"id"`
	TenantID   string     `json:"tenant_id,omitempty"`
	ProjectID  string     `json:"project_id,omitempty"`
	VMID       string     `json:"vm_id,omitempty"`
	IPAddress  string     `json:"ip_address"`
	ProviderID string     `json:"provider_id,omitempty"`
	Status     string     `json:"status"`
	AssignedAt *time.Time `json:"assigned_at,omitempty"`
	LastSeenAt *time.Time `json:"last_seen_at,omitempty"`
}

type FirewallRule struct {
	ID           string     `json:"id"`
	TenantID     string     `json:"tenant_id,omitempty"`
	ProjectID    string     `json:"project_id,omitempty"`
	ProviderID   string     `json:"provider_id,omitempty"`
	Name         string     `json:"name,omitempty"`
	Direction    string     `json:"direction"`
	Protocol     string     `json:"protocol"`
	PortMin      int        `json:"port_min,omitempty"`
	PortMax      int        `json:"port_max,omitempty"`
	SourceCIDR   string     `json:"source_cidr,omitempty"`
	DestCIDR     string     `json:"dest_cidr,omitempty"`
	SourceVMID   string     `json:"source_vm_id,omitempty"`
	DestVMID     string     `json:"dest_vm_id,omitempty"`
	Action       string     `json:"action"`
	Scope        string     `json:"scope"`
	Description  string     `json:"description,omitempty"`
	LastSyncedAt *time.Time `json:"last_synced_at,omitempty"`
}

type NetworkRoute struct {
	ID           string     `json:"id"`
	TenantID     string     `json:"tenant_id,omitempty"`
	ProjectID    string     `json:"project_id,omitempty"`
	ProviderID   string     `json:"provider_id,omitempty"`
	NetworkID    string     `json:"network_id,omitempty"`
	SubnetID     string     `json:"subnet_id,omitempty"`
	Destination  string     `json:"destination"`
	NextHop      string     `json:"next_hop,omitempty"`
	RouteType    string     `json:"route_type"`
	Description  string     `json:"description,omitempty"`
	LastSyncedAt *time.Time `json:"last_synced_at,omitempty"`
}

type NetworkPolicy struct {
	ID           string     `json:"id"`
	TenantID     string     `json:"tenant_id,omitempty"`
	ProjectID    string     `json:"project_id,omitempty"`
	Name         string     `json:"name"`
	Zone         string     `json:"zone,omitempty"`
	SourceVMID   string     `json:"source_vm_id,omitempty"`
	DestVMID     string     `json:"dest_vm_id,omitempty"`
	Protocol     string     `json:"protocol,omitempty"`
	Port         int        `json:"port,omitempty"`
	Action       string     `json:"action"`
	Description  string     `json:"description,omitempty"`
	LastSyncedAt *time.Time `json:"last_synced_at,omitempty"`
}

type CloudContext struct {
	Provider        CloudProviderStatus `json:"provider"`
	Hosts           []CloudHost         `json:"hosts"`
	PublicIPs       []PublicIP          `json:"public_ips"`
	FirewallRules   []FirewallRule      `json:"firewall_rules"`
	Routes          []NetworkRoute      `json:"routes"`
	NetworkPolicies []NetworkPolicy     `json:"network_policies"`
}

type ConnectionIntent struct {
	ID         string    `json:"id"`
	TenantID   string    `json:"tenant_id,omitempty"`
	ProjectID  string    `json:"project_id,omitempty"`
	SourceVMID string    `json:"source_vm_id"`
	DestVMID   string    `json:"dest_vm_id"`
	Protocol   string    `json:"protocol"`
	Port       int       `json:"port"`
	Purpose    string    `json:"purpose,omitempty"`
	Exposure   string    `json:"exposure"`
	Required   bool      `json:"required"`
	Status     string    `json:"status"`
	CreatedBy  string    `json:"created_by,omitempty"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

type ConnectionConfiguration struct {
	ID             string     `json:"id"`
	TenantID       string     `json:"tenant_id,omitempty"`
	ProjectID      string     `json:"project_id,omitempty"`
	IntentID       string     `json:"intent_id,omitempty"`
	SourceVMID     string     `json:"source_vm_id"`
	DestVMID       string     `json:"dest_vm_id"`
	Protocol       string     `json:"protocol"`
	Port           int        `json:"port"`
	FirewallRuleID string     `json:"firewall_rule_id,omitempty"`
	RouteID        string     `json:"route_id,omitempty"`
	NetworkID      string     `json:"network_id,omitempty"`
	ConfigState    string     `json:"config_state"`
	SecurityState  string     `json:"security_state"`
	LastSyncedAt   *time.Time `json:"last_synced_at,omitempty"`
}

type ConnectionObservedState struct {
	SourceVMID     string     `json:"source_vm_id"`
	DestVMID       string     `json:"dest_vm_id"`
	Protocol       string     `json:"protocol"`
	Port           int        `json:"port"`
	Scope          string     `json:"scope"`
	Observed       bool       `json:"observed"`
	Reachable      bool       `json:"reachable"`
	Active         bool       `json:"active"`
	Failed         bool       `json:"failed"`
	RequestCount   int64      `json:"request_count"`
	ErrorCount     int64      `json:"error_count"`
	BytesTotal     int64      `json:"bytes_total"`
	AvgRTTMs       float64    `json:"avg_rtt_ms,omitempty"`
	LastObservedAt *time.Time `json:"last_observed_at,omitempty"`
	LastErrorAt    *time.Time `json:"last_error_at,omitempty"`
}

type ConnectionView struct {
	ID                 string                   `json:"id"`
	TenantID           string                   `json:"tenant_id,omitempty"`
	ProjectID          string                   `json:"project_id,omitempty"`
	SourceVMID         string                   `json:"source_vm_id"`
	SourceName         string                   `json:"source_name,omitempty"`
	SourceIP           string                   `json:"source_ip,omitempty"`
	DestVMID           string                   `json:"dest_vm_id"`
	DestName           string                   `json:"dest_name,omitempty"`
	DestIP             string                   `json:"dest_ip,omitempty"`
	Protocol           string                   `json:"protocol"`
	Port               int                      `json:"port"`
	Scope              string                   `json:"scope"`
	Intent             *ConnectionIntent        `json:"intent,omitempty"`
	Configuration      *ConnectionConfiguration `json:"configuration,omitempty"`
	Observed           ConnectionObservedState  `json:"observed"`
	IntendedState      string                   `json:"intended_state"`
	ConfigurationState string                   `json:"configuration_state"`
	ObservedState      string                   `json:"observed_state"`
	ValidationState    string                   `json:"validation_state"`
	HealthState        string                   `json:"health_state"`
	SecurityState      string                   `json:"security_state"`
	Description        string                   `json:"description"`
	LastChangedAt      *time.Time               `json:"last_changed_at,omitempty"`
}

type ConnectionChangeEvent struct {
	ID                 string    `json:"id"`
	TenantID           string    `json:"tenant_id,omitempty"`
	ProjectID          string    `json:"project_id,omitempty"`
	ConnectionID       string    `json:"connection_id,omitempty"`
	ChangeType         string    `json:"change_type"`
	Actor              string    `json:"actor,omitempty"`
	BeforeState        string    `json:"before_state,omitempty"`
	AfterState         string    `json:"after_state,omitempty"`
	AffectedSourceVMID string    `json:"affected_source_vm_id,omitempty"`
	AffectedDestVMID   string    `json:"affected_dest_vm_id,omitempty"`
	ValidationState    string    `json:"validation_state,omitempty"`
	CreatedAt          time.Time `json:"created_at"`
}
