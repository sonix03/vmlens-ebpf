package service

import "fmt"

var tcpServices = map[int]string{
	20: "ftp-data", 21: "ftp", 22: "ssh", 25: "smtp", 53: "dns", 80: "http",
	110: "pop3", 143: "imap", 443: "https", 445: "smb", 465: "smtps", 587: "smtp",
	636: "ldaps", 873: "rsync", 1883: "mqtt", 2379: "etcd", 2380: "etcd-peer",
	3000: "development-http", 3306: "mysql", 4222: "nats", 5432: "postgresql",
	5671: "rabbitmq-tls", 5672: "rabbitmq", 6379: "redis", 6443: "kubernetes-api",
	8080: "http-alt", 8081: "http-alt", 8443: "https-alt", 9090: "prometheus",
	9092: "kafka", 9100: "node-exporter", 9200: "elasticsearch", 9418: "git",
	10250: "kubelet", 15672: "rabbitmq-management", 27017: "mongodb",
}

var udpServices = map[int]string{
	53: "dns", 67: "dhcp-server", 68: "dhcp-client", 123: "ntp", 161: "snmp",
	443: "quic", 514: "syslog", 8125: "statsd",
}

func classifyService(protocol, direction string, srcPort, dstPort int) (string, int) {
	services := tcpServices
	if protocol == "udp" {
		services = udpServices
	}
	// A client socket keeps the service on its remote/destination port even
	// while receiving. A server socket keeps it on its local/source port.
	if name, ok := services[dstPort]; ok {
		return name, dstPort
	}
	if name, ok := services[srcPort]; ok {
		return name, srcPort
	}
	if protocol == "tcp" && dstPort >= 30000 && dstPort <= 32767 {
		return "kubernetes-nodeport", dstPort
	}
	if direction == "ingress" {
		return fmt.Sprintf("%s/%d", protocol, srcPort), srcPort
	}
	return fmt.Sprintf("%s/%d", protocol, dstPort), dstPort
}
