package collector

import (
	"testing"

	"github.com/vmlens/vmlens-ebpf/pkg/config"
)

func TestParseSSHLoginAndLogout(t *testing.T) {
	c := NewSSH(config.Default())
	c.parse("Jun 30 host sshd[1001]: Accepted publickey for ubuntu from 36.72.1.2 port 51233 ssh2")
	login := <-c.Events
	if login.EventType != "ssh_login" || login.User != "ubuntu" || login.RemoteIP != "36.72.1.2" || login.RemotePort != 51233 || login.AuthMethod != "publickey" || login.SSHPID != 1001 {
		t.Fatalf("unexpected login: %+v", login)
	}
	c.parse("Jun 30 host sshd[1001]: Disconnected from user ubuntu 36.72.1.2 port 51233")
	logout := <-c.Events
	if logout.EventType != "ssh_logout" || logout.User != "ubuntu" || logout.SSHPID != 1001 {
		t.Fatalf("unexpected logout: %+v", logout)
	}
}
