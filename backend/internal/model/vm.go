package model

import "time"

type VM struct {
	ID           string      `json:"id"`
	Name         string      `json:"name"`
	TenantID     string      `json:"tenant_id,omitempty"`
	ProjectID    string      `json:"project_id,omitempty"`
	Region       string      `json:"region,omitempty"`
	Zone         string      `json:"zone,omitempty"`
	PrivateIP    string      `json:"private_ip,omitempty"`
	PublicIP     string      `json:"public_ip,omitempty"`
	NetworkID    string      `json:"network_id,omitempty"`
	SubnetID     string      `json:"subnet_id,omitempty"`
	MACAddress   string      `json:"mac_address,omitempty"`
	HostID       string      `json:"host_id,omitempty"`
	Role         string      `json:"role,omitempty"`
	Environment  string      `json:"environment,omitempty"`
	Owner        string      `json:"owner,omitempty"`
	DiscoveredBy string      `json:"discovered_by"`
	AgentID      string      `json:"agent_id,omitempty"`
	MachineID    string      `json:"machine_id,omitempty"`
	Status       string      `json:"status"`
	FirstSeen    time.Time   `json:"first_seen"`
	LastSeen     time.Time   `json:"last_seen"`
	CreatedAt    time.Time   `json:"created_at"`
	Interfaces   []Interface `json:"interfaces,omitempty"`
}
