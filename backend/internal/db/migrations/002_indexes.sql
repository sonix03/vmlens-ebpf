CREATE INDEX IF NOT EXISTS idx_agents_vm_id ON agents(vm_id);
CREATE INDEX IF NOT EXISTS idx_agents_machine_id ON agents(machine_id);
CREATE INDEX IF NOT EXISTS idx_agents_last_seen ON agents(last_seen);

CREATE INDEX IF NOT EXISTS idx_vms_agent_id ON vms(agent_id);
CREATE INDEX IF NOT EXISTS idx_vms_machine_id ON vms(machine_id);
CREATE INDEX IF NOT EXISTS idx_vms_private_ip ON vms(private_ip);
CREATE INDEX IF NOT EXISTS idx_vms_public_ip ON vms(public_ip);
CREATE INDEX IF NOT EXISTS idx_vms_status ON vms(status);

CREATE INDEX IF NOT EXISTS idx_vm_interfaces_vm_id ON vm_interfaces(vm_id);
CREATE INDEX IF NOT EXISTS idx_vm_interfaces_ip_address ON vm_interfaces(ip_address);
CREATE INDEX IF NOT EXISTS idx_vm_interfaces_mac_address ON vm_interfaces(mac_address);

CREATE INDEX IF NOT EXISTS idx_network_flows_agent_id ON network_flows(agent_id);
CREATE INDEX IF NOT EXISTS idx_network_flows_src_ip ON network_flows(src_ip);
CREATE INDEX IF NOT EXISTS idx_network_flows_dst_ip ON network_flows(dst_ip);
CREATE INDEX IF NOT EXISTS idx_network_flows_src_vm_id ON network_flows(src_vm_id);
CREATE INDEX IF NOT EXISTS idx_network_flows_dst_vm_id ON network_flows(dst_vm_id);
CREATE INDEX IF NOT EXISTS idx_network_flows_scope ON network_flows(scope);
CREATE INDEX IF NOT EXISTS idx_network_flows_protocol ON network_flows(protocol);
CREATE INDEX IF NOT EXISTS idx_network_flows_last_seen ON network_flows(last_seen DESC);

