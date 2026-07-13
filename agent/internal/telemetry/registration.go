package telemetry

type Interface struct {
	Name       string `json:"name"`
	IPAddress  string `json:"ip_address,omitempty"`
	MACAddress string `json:"mac_address,omitempty"`
}

type Registration struct {
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
	AgentVersion string      `json:"agent_version"`
	Environment  string      `json:"environment,omitempty"`
}

type RegistrationResult struct {
	AgentID string `json:"agent_id"`
	VMID    string `json:"vm_id"`
	Status  string `json:"status"`
}

type Heartbeat struct {
	AgentID   string `json:"agent_id"`
	Status    string `json:"status"`
	Timestamp string `json:"timestamp"`
}
