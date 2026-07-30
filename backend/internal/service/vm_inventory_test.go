package service

import (
	"os"
	"path/filepath"
	"testing"
)

func TestVMInventoryLoadAliases(t *testing.T) {
	path := filepath.Join(t.TempDir(), "vms.local")
	content := `# alias|host|ssh_user|ssh_key|remote_backend|local_backend|proxy_jump
testing-a-1sss|10.20.20.130|-|-|-|-|ubuntu@10.20.20.199
testing-a-2 | 10.20.20.199 | - | - | - | - | -
hostname-only|testing-a-4|-|-|-|-|-
broken-row
`
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}

	aliases, err := NewVMInventory(path).LoadAliases()
	if err != nil {
		t.Fatal(err)
	}
	if got := aliases["10.20.20.130"]; got != "testing-a-1sss" {
		t.Fatalf("alias for .130 = %q", got)
	}
	if got := aliases["10.20.20.199"]; got != "testing-a-2" {
		t.Fatalf("alias for .199 = %q", got)
	}
	if _, ok := aliases["testing-a-4"]; ok {
		t.Fatal("hostname inventory rows should not be synced into inet private_ip")
	}
}

func TestVMInventoryMissingFileIsEmpty(t *testing.T) {
	aliases, err := NewVMInventory(filepath.Join(t.TempDir(), "missing.local")).LoadAliases()
	if err != nil {
		t.Fatal(err)
	}
	if len(aliases) != 0 {
		t.Fatalf("expected no aliases, got %#v", aliases)
	}
}
