package service

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"net/netip"
	"os"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
)

// VMInventory reads operator-managed VM assignments once at backend startup.
// The first seven fields are shared with the tunnel script; the remaining
// fields are optional backend/cloud metadata used to avoid scattering local lab
// constants through the codebase.
type VMInventory struct {
	path      string
	entries   []VMInventoryEntry
	entriesIP map[string]VMInventoryEntry
}

type VMInventoryEntry struct {
	Alias          string
	Host           string
	SSHUser        string
	SSHKey         string
	RemoteBackend  string
	LocalBackend   string
	ProxyJump      string
	Role           string
	Type           string
	Environment    string
	Owner          string
	TenantID       string
	ProjectID      string
	Region         string
	Zone           string
	NetworkID      string
	SubnetID       string
	PublicIP       string
	ProviderID     string
	ProbeProtocol  string
	ProbePort      int
	CaptureIface   string
	IgnorePorts    []int
	IgnoreIPs      []string
	FlowAllowCIDRs []string
	FlowDenyCIDRs  []string
	Notes          string
}

func NewVMInventory(path string) *VMInventory {
	inventory := &VMInventory{path: strings.TrimSpace(path), entriesIP: map[string]VMInventoryEntry{}}
	_ = inventory.Load()
	return inventory
}

func (i *VMInventory) Load() error {
	entries, err := i.LoadEntries()
	if err != nil {
		return err
	}
	i.entries = entries
	i.entriesIP = map[string]VMInventoryEntry{}
	for _, entry := range entries {
		if addr, err := netip.ParseAddr(entry.Host); err == nil {
			i.entriesIP[addr.String()] = entry
		}
	}
	return nil
}

func (i *VMInventory) Entries() []VMInventoryEntry {
	if i == nil {
		return nil
	}
	out := make([]VMInventoryEntry, len(i.entries))
	copy(out, i.entries)
	return out
}

func (i *VMInventory) AliasForIP(ip string) string {
	entry, ok := i.EntryForIP(ip)
	if !ok {
		return ""
	}
	return entry.Alias
}

func (i *VMInventory) EntryForIP(ip string) (VMInventoryEntry, bool) {
	if i == nil {
		return VMInventoryEntry{}, false
	}
	entry, ok := i.entriesIP[strings.TrimSpace(ip)]
	return entry, ok
}

func (i *VMInventory) ProbeForIP(ip string, fallbackProtocol string, fallbackPort int) (string, int) {
	entry, ok := i.EntryForIP(ip)
	if !ok {
		return fallbackProtocol, fallbackPort
	}
	protocol := firstNonEmpty([]string{entry.ProbeProtocol}, []string{fallbackProtocol})
	port := fallbackPort
	if entry.ProbePort > 0 {
		port = entry.ProbePort
	}
	return protocol, port
}

func (i *VMInventory) IgnoredPorts() []int {
	if i == nil {
		return nil
	}
	seen := map[int]bool{}
	var out []int
	for _, entry := range i.entries {
		if entry.ProbePort > 0 && !seen[entry.ProbePort] {
			seen[entry.ProbePort] = true
			out = append(out, entry.ProbePort)
		}
		for _, port := range entry.IgnorePorts {
			if port > 0 && !seen[port] {
				seen[port] = true
				out = append(out, port)
			}
		}
	}
	return out
}

func (i *VMInventory) IgnoredIPs() []string {
	if i == nil {
		return nil
	}
	seen := map[string]bool{}
	var out []string
	for _, entry := range i.entries {
		for _, ip := range entry.IgnoreIPs {
			if ip != "" && !seen[ip] {
				seen[ip] = true
				out = append(out, ip)
			}
		}
	}
	return out
}

func (i *VMInventory) LoadAliases() (map[string]string, error) {
	entries, err := i.LoadEntries()
	if err != nil {
		return nil, err
	}
	aliases := map[string]string{}
	for _, entry := range entries {
		if addr, err := netip.ParseAddr(entry.Host); err == nil {
			aliases[addr.String()] = entry.Alias
		}
	}
	return aliases, nil
}

func (i *VMInventory) LoadEntries() ([]VMInventoryEntry, error) {
	entries := []VMInventoryEntry{}
	if i == nil || i.path == "" {
		return entries, nil
	}
	file, err := os.Open(i.path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return entries, nil
		}
		return nil, err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		entry, ok, err := parseVMInventoryEntry(line)
		if err != nil {
			return nil, err
		}
		if !ok {
			continue
		}
		entries = append(entries, entry)
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return entries, nil
}

func (i *VMInventory) ApplyToDatabase(ctx context.Context, pool *pgxpool.Pool) error {
	if pool == nil {
		return nil
	}
	for _, entry := range i.Entries() {
		if _, err := netip.ParseAddr(entry.Host); err != nil {
			continue
		}
		if _, err := pool.Exec(ctx, `
			UPDATE vms SET
				name = $1,
				tenant_id = COALESCE($2, tenant_id),
				project_id = COALESCE($3, project_id),
				region = COALESCE($4, region),
				zone = COALESCE($5, zone),
				network_id = COALESCE($6, network_id),
				subnet_id = COALESCE($7, subnet_id),
				public_ip = COALESCE($8::inet, public_ip),
				host_id = COALESCE($9, host_id),
				role = COALESCE($10, role),
				host_type = COALESCE($11, host_type),
				environment = COALESCE($12, environment),
				owner = COALESCE($13, owner)
			WHERE private_ip = $14::inet`,
			entry.Alias, nullIfEmpty(entry.TenantID), nullIfEmpty(entry.ProjectID), nullIfEmpty(entry.Region),
			nullIfEmpty(entry.Zone), nullIfEmpty(entry.NetworkID), nullIfEmpty(entry.SubnetID), nullIfEmpty(entry.PublicIP),
			nullIfEmpty(entry.ProviderID), nullIfEmpty(entry.Role), nullIfEmpty(entry.Type), nullIfEmpty(entry.Environment),
			nullIfEmpty(entry.Owner), entry.Host); err != nil {
			return fmt.Errorf("apply VM inventory assignment %s=%s: %w", entry.Host, entry.Alias, err)
		}
	}
	return nil
}

func parseVMInventoryEntry(line string) (VMInventoryEntry, bool, error) {
	fields := strings.Split(line, "|")
	if len(fields) < 2 {
		return VMInventoryEntry{}, false, nil
	}
	entry := VMInventoryEntry{
		Alias:          inventoryField(fields, 0),
		Host:           inventoryField(fields, 1),
		SSHUser:        inventoryField(fields, 2),
		SSHKey:         inventoryField(fields, 3),
		RemoteBackend:  inventoryField(fields, 4),
		LocalBackend:   inventoryField(fields, 5),
		ProxyJump:      inventoryField(fields, 6),
		Role:           inventoryField(fields, 7),
		Type:           inventoryField(fields, 8),
		Environment:    inventoryField(fields, 9),
		Owner:          inventoryField(fields, 10),
		TenantID:       inventoryField(fields, 11),
		ProjectID:      inventoryField(fields, 12),
		Region:         inventoryField(fields, 13),
		Zone:           inventoryField(fields, 14),
		NetworkID:      inventoryField(fields, 15),
		SubnetID:       inventoryField(fields, 16),
		PublicIP:       inventoryField(fields, 17),
		ProviderID:     inventoryField(fields, 18),
		ProbeProtocol:  strings.ToLower(inventoryField(fields, 19)),
		ProbePort:      inventoryPort(fields, 20),
		CaptureIface:   inventoryField(fields, 21),
		IgnorePorts:    inventoryPorts(fields, 22),
		IgnoreIPs:      inventoryCSV(fields, 23),
		FlowAllowCIDRs: inventoryCSV(fields, 24),
		FlowDenyCIDRs:  inventoryCSV(fields, 25),
		Notes:          inventoryField(fields, 26),
	}
	if entry.Alias == "" || entry.Host == "" {
		return VMInventoryEntry{}, false, nil
	}
	if entry.ProbeProtocol != "" && entry.ProbeProtocol != "tcp" && entry.ProbeProtocol != "udp" && entry.ProbeProtocol != "icmp" {
		return VMInventoryEntry{}, false, fmt.Errorf("inventory %s: unsupported probe protocol %q", entry.Alias, entry.ProbeProtocol)
	}
	if entry.PublicIP != "" {
		if _, err := netip.ParseAddr(entry.PublicIP); err != nil {
			return VMInventoryEntry{}, false, fmt.Errorf("inventory %s: invalid public_ip %q", entry.Alias, entry.PublicIP)
		}
	}
	return entry, true, nil
}

func inventoryField(fields []string, index int) string {
	if index >= len(fields) {
		return ""
	}
	return cleanInventoryField(fields[index])
}

func cleanInventoryField(value string) string {
	value = strings.TrimSpace(value)
	if value == "-" {
		return ""
	}
	return value
}

func inventoryCSV(fields []string, index int) []string {
	value := inventoryField(fields, index)
	if value == "" {
		return nil
	}
	parts := strings.Split(value, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		if item := cleanInventoryField(part); item != "" {
			out = append(out, item)
		}
	}
	return out
}

func inventoryPorts(fields []string, index int) []int {
	values := inventoryCSV(fields, index)
	out := make([]int, 0, len(values))
	for _, value := range values {
		port, err := strconv.Atoi(value)
		if err == nil && port > 0 && port <= 65535 {
			out = append(out, port)
		}
	}
	return out
}

func inventoryPort(fields []string, index int) int {
	value := inventoryField(fields, index)
	if value == "" {
		return 0
	}
	port, err := strconv.Atoi(value)
	if err != nil || port < 1 || port > 65535 {
		return 0
	}
	return port
}
