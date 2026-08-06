package devserver

import (
	"encoding/json"
	"fmt"
	"net"
	"strconv"
	"strings"

	"github.com/smell-of-curry/pokebedrock-hub/pokebedrock/srv"
)

// IdentifierPrefix marks dynamically registered beta/dev servers.
const IdentifierPrefix = "dev-"

// APIResponse is the JSON body from GET /dev-servers.
type APIResponse struct {
	Servers []APIServer `json:"servers"`
}

// APIServer is one entry from the remote manager API.
type APIServer struct {
	Name       string `json:"name"`
	Type       string `json:"type"`
	PRNumber   *int   `json:"prNumber"`
	Branch     string `json:"branch"`
	Port       int    `json:"port"`
	MaxPlayers int    `json:"maxPlayers"`
	Status     string `json:"status"`
}

// Snapshot is the comparable view of a currently registered dev server.
type Snapshot struct {
	Name    string
	Address string
}

// DecodeResponse parses a /dev-servers JSON payload.
func DecodeResponse(data []byte) (APIResponse, error) {
	var resp APIResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		return APIResponse{}, fmt.Errorf("decode dev-servers response: %w", err)
	}
	return resp, nil
}

// IdentifierFor returns the registry identifier for an API server entry.
func IdentifierFor(s APIServer) (string, error) {
	switch strings.ToLower(s.Type) {
	case "beta":
		return IdentifierPrefix + "beta", nil
	case "pr":
		if s.PRNumber == nil {
			return "", fmt.Errorf("pr server %q missing prNumber", s.Name)
		}
		return fmt.Sprintf("%spr-%d", IdentifierPrefix, *s.PRNumber), nil
	default:
		return "", fmt.Errorf("unknown server type %q", s.Type)
	}
}

// DisplayName returns the hub-facing server name.
func DisplayName(s APIServer) string {
	if strings.EqualFold(s.Type, "pr") && s.Branch != "" {
		return fmt.Sprintf("%s (%s)", s.Name, s.Branch)
	}
	return s.Name
}

// DesiredConfigs builds registry configs for running API servers on host.
// Non-running entries are omitted (treated as gone).
func DesiredConfigs(servers []APIServer, host string) (map[string]srv.Config, error) {
	desired := make(map[string]srv.Config, len(servers))
	for _, s := range servers {
		if !strings.EqualFold(s.Status, "running") {
			continue
		}
		id, err := IdentifierFor(s)
		if err != nil {
			return nil, err
		}
		desired[id] = srv.Config{
			Name:       DisplayName(s),
			Identifier: id,
			Address:    net.JoinHostPort(host, strconv.Itoa(s.Port)),
			BetaLock:   true,
		}
	}
	return desired, nil
}

// Diff computes register/unregister ops between current and desired dev servers.
// Only identifiers present in current/desired are considered; callers must pass
// only IdentifierPrefix entries as current.
func Diff(current map[string]Snapshot, desired map[string]srv.Config) (register []srv.Config, unregister []string) {
	for id := range current {
		if _, ok := desired[id]; !ok {
			unregister = append(unregister, id)
		}
	}
	for id, cfg := range desired {
		cur, ok := current[id]
		if !ok || cur.Name != cfg.Name || cur.Address != cfg.Address {
			register = append(register, cfg)
		}
	}
	return register, unregister
}

// IsDevIdentifier reports whether id belongs to the dynamic dev registry.
func IsDevIdentifier(id string) bool {
	return strings.HasPrefix(id, IdentifierPrefix)
}
