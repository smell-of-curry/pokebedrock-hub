package status

import (
	"github.com/sandertv/gophertunnel/minecraft"
	"github.com/smell-of-curry/pokebedrock-hub/pokebedrock/srv"
)

// Provider ...
type Provider struct {
	name    string
	subName string
}

// NewProvider ...
func NewProvider(serverName, serverSubName string) *Provider {
	return &Provider{name: serverName, subName: serverSubName}
}

// ServerStatus ...
func (p *Provider) ServerStatus(int, int) minecraft.ServerStatus {
	var count, maxCount int
	for _, server := range srv.All() {
		if st := server.Status(); st.Online {
			count += st.PlayerCount
			maxCount += st.MaxPlayerCount
		}
	}

	return minecraft.ServerStatus{
		ServerName:    p.name,
		ServerSubName: p.subName,
		PlayerCount:   count,
		MaxPlayers:    maxCount,
	}
}
