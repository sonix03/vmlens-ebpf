package metadata

import (
	"bufio"
	"net"
	"os"
	"runtime"
	"strings"

	"golang.org/x/sys/unix"

	"github.com/vmlens/vmlens-ebpf/internal/config"
)

type Interface struct {
	Name      string   `json:"name"`
	Addresses []string `json:"addresses"`
}

type VM struct {
	TenantID      string      `json:"tenant_id,omitempty"`
	UserID        string      `json:"user_id,omitempty"`
	VMID          string      `json:"vm_id"`
	Hostname      string      `json:"hostname"`
	PrivateIP     string      `json:"private_ip,omitempty"`
	PublicIP      string      `json:"public_ip,omitempty"`
	Region        string      `json:"region,omitempty"`
	OS            string      `json:"os"`
	KernelVersion string      `json:"kernel_version"`
	BootID        string      `json:"boot_id"`
	Interfaces    []Interface `json:"network_interfaces"`
}

func Collect(cfg config.VMConfig) VM {
	hostname := cfg.Hostname
	if hostname == "" {
		hostname, _ = os.Hostname()
	}
	return VM{
		TenantID: cfg.TenantID, UserID: cfg.UserID, VMID: cfg.VMID,
		Hostname: hostname, PrivateIP: cfg.PrivateIP, PublicIP: cfg.PublicIP,
		Region: cfg.Region, OS: operatingSystem(), KernelVersion: kernelVersion(),
		BootID: readTrimmed("/proc/sys/kernel/random/boot_id"), Interfaces: interfaces(),
	}
}

func operatingSystem() string {
	f, err := os.Open("/etc/os-release")
	if err != nil {
		return runtime.GOOS
	}
	defer f.Close()
	values := map[string]string{}
	s := bufio.NewScanner(f)
	for s.Scan() {
		key, value, ok := strings.Cut(s.Text(), "=")
		if ok {
			values[key] = strings.Trim(value, "\"")
		}
	}
	if values["PRETTY_NAME"] != "" {
		return values["PRETTY_NAME"]
	}
	return runtime.GOOS
}

func kernelVersion() string {
	var u unix.Utsname
	if unix.Uname(&u) != nil {
		return "unknown"
	}
	b := make([]byte, 0, len(u.Release))
	for _, c := range u.Release {
		if c == 0 {
			break
		}
		b = append(b, byte(c))
	}
	return string(b)
}

func interfaces() []Interface {
	list, _ := net.Interfaces()
	out := make([]Interface, 0, len(list))
	for _, iface := range list {
		addrs, _ := iface.Addrs()
		item := Interface{Name: iface.Name}
		for _, addr := range addrs {
			item.Addresses = append(item.Addresses, addr.String())
		}
		out = append(out, item)
	}
	return out
}

func readTrimmed(path string) string {
	b, err := os.ReadFile(path)
	if err != nil {
		return "unknown"
	}
	return strings.TrimSpace(string(b))
}

func (v VM) InterfaceForIP(ip string) string {
	parsed := net.ParseIP(ip)
	if parsed == nil {
		return "unknown"
	}
	for _, iface := range v.Interfaces {
		for _, raw := range iface.Addresses {
			addr, _, err := net.ParseCIDR(raw)
			if err == nil && addr.Equal(parsed) {
				return iface.Name
			}
		}
	}
	// A configured private IP can differ from the current development namespace
	// (for example, WSL or a container). Preserve a useful low-cardinality label.
	if ip == v.PrivateIP {
		for _, iface := range v.Interfaces {
			if iface.Name != "lo" {
				return iface.Name
			}
		}
	}
	return "unknown"
}
