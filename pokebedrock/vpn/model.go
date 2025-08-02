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
}
