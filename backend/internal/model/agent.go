package model

import "time"

type AgentRegistration struct {
	AgentID      string      `json:"agent_id"`
	Hostname     string      `json:"hostname"`
	MachineID    string      `json:"machine_id,omitempty"`
	TenantID     string      `json:"tenant_id,omitempty"`
	PrivateIPs   []string    `json:"private_ips,omitempty"`
	PublicIP     *string     `json:"public_ip,omitempty"`
	MACAddresses []string    `json:"mac_addresses,omitempty"`
	Interfaces   []Interface `json:"interfaces,omitempty"`
	OS           string      `json:"os,omitempty"`
	Kernel       string      `json:"kernel,omitempty"`
	AgentVersion string      `json:"agent_version,omitempty"`
	Environment  string      `json:"environment,omitempty"`
}

type AgentHeartbeat struct {
	AgentID   string    `json:"agent_id"`
	Status    string    `json:"status"`
	Timestamp time.Time `json:"timestamp"`
}

type Agent struct {
	ID           string    `json:"id"`
	VMID         string    `json:"vm_id,omitempty"`
	Hostname     string    `json:"hostname"`
	MachineID    string    `json:"machine_id,omitempty"`
	OS           string    `json:"os,omitempty"`
	Kernel       string    `json:"kernel,omitempty"`
	AgentVersion string    `json:"agent_version,omitempty"`
	Environment  string    `json:"environment,omitempty"`
	Status       string    `json:"status"`
	FirstSeen    time.Time `json:"first_seen"`
	LastSeen     time.Time `json:"last_seen"`
}

type RegistrationResult struct {
	AgentID string `json:"agent_id"`
	VMID    string `json:"vm_id"`
	Status  string `json:"status"`
}
