package identity

import (
	"bufio"
	"crypto/sha256"
	"fmt"
	"net"
	"os"
	"runtime"
	"strings"

	"github.com/vmlens/vmlens/agent/internal/config"
	"github.com/vmlens/vmlens/agent/internal/model"
)

func Collect(cfg config.Config) (model.Registration, error) {
	hostname := strings.TrimSpace(cfg.Hostname)
	if hostname == "" {
		hostname, _ = os.Hostname()
	}
	machineID := strings.TrimSpace(cfg.MachineID)
	if machineID == "" {
		machineID = readTrimmed("/etc/machine-id")
	}
	interfaces, privateIPs, macs := networkIdentity()
	if len(cfg.PrivateIPs) > 0 {
		privateIPs = cfg.PrivateIPs
		interfaces = make([]model.Interface, 0, len(privateIPs))
		for index, ip := range privateIPs {
			iface := model.Interface{Name: fmt.Sprintf("mock%d", index), IPAddress: ip}
			if index < len(cfg.MACAddresses) {
				iface.MACAddress = cfg.MACAddresses[index]
			}
			interfaces = append(interfaces, iface)
		}
	}
	if len(cfg.MACAddresses) > 0 {
		macs = cfg.MACAddresses
	}
	agentID := strings.TrimSpace(cfg.AgentID)
	if agentID == "" {
		sum := sha256.Sum256([]byte(machineID + "|" + hostname))
		agentID = fmt.Sprintf("agent-%x", sum[:6])
	}
	var publicIP *string
	if cfg.PublicIP != "" {
		value := cfg.PublicIP
		publicIP = &value
	}
	return model.Registration{
		AgentID: agentID, Hostname: hostname, MachineID: machineID, TenantID: cfg.TenantID,
		PrivateIPs: privateIPs, PublicIP: publicIP, MACAddresses: macs, Interfaces: interfaces,
		OS: operatingSystem(), Kernel: readTrimmed("/proc/sys/kernel/osrelease"),
		AgentVersion: cfg.AgentVersion, Environment: cfg.Environment,
	}, nil
}

func networkIdentity() ([]model.Interface, []string, []string) {
	list, _ := net.Interfaces()
	interfaces := []model.Interface{}
	privateIPs := []string{}
	macs := []string{}
	for _, iface := range list {
		if iface.Flags&net.FlagLoopback != 0 {
			continue
		}
		mac := iface.HardwareAddr.String()
		if mac != "" {
			macs = append(macs, mac)
		}
		addresses, _ := iface.Addrs()
		if len(addresses) == 0 {
			interfaces = append(interfaces, model.Interface{Name: iface.Name, MACAddress: mac})
		}
		for _, address := range addresses {
			ip, _, err := net.ParseCIDR(address.String())
			if err != nil || ip == nil || ip.IsLoopback() {
				continue
			}
			value := ip.String()
			if ip.To4() != nil {
				privateIPs = append(privateIPs, value)
			}
			interfaces = append(interfaces, model.Interface{Name: iface.Name, IPAddress: value, MACAddress: mac})
		}
	}
	return interfaces, unique(privateIPs), unique(macs)
}

func operatingSystem() string {
	file, err := os.Open("/etc/os-release")
	if err != nil {
		return runtime.GOOS
	}
	defer file.Close()
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		key, value, ok := strings.Cut(scanner.Text(), "=")
		if ok && key == "ID" {
			return strings.Trim(value, "\"")
		}
	}
	return runtime.GOOS
}

func readTrimmed(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(data))
}

func unique(values []string) []string {
	seen := map[string]struct{}{}
	out := make([]string, 0, len(values))
	for _, value := range values {
		if _, ok := seen[value]; ok || value == "" {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	return out
}
