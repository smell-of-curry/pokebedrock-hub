// Package vpn provides a model for the VPN service.
package vpn

// Status constants for the VPN service.
const (
	StatusSuccess = "success"
	StatusFail    = "fail"
)

// ResponseModel ...
type ResponseModel struct {
	Status  string `json:"status"`
	Message string `json:"message,omitempty"`
	Proxy   bool   `json:"proxy"`
	// Isp and Org identify the connection's provider. They are only used
	// for logging denied connections so false positives can be audited;
	// they are not cached.
	Isp string `json:"isp,omitempty"`
	Org string `json:"org,omitempty"`
}
