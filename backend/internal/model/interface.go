package model

type Interface struct {
	Name       string `json:"name"`
	IPAddress  string `json:"ip_address,omitempty"`
	MACAddress string `json:"mac_address,omitempty"`
}
