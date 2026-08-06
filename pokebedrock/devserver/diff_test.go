package devserver

import (
	"reflect"
	"sort"
	"testing"

	"github.com/smell-of-curry/pokebedrock-hub/pokebedrock/srv"
)

func TestDiffRegisterUnregisterAndUpdate(t *testing.T) {
	pr := 123
	desired, err := DesiredConfigs([]APIServer{
		{Name: "BETA", Type: "beta", Port: 19200, Status: "running"},
		{Name: "PR-123", Type: "pr", PRNumber: &pr, Branch: "feature/x", Port: 19204, Status: "running"},
		{Name: "PR-999", Type: "pr", PRNumber: intPtr(999), Port: 19205, Status: "stopping"},
	}, "198.244.176.51")
	if err != nil {
		t.Fatal(err)
	}

	current := map[string]Snapshot{
		"dev-beta":   {Name: "BETA", Address: "198.244.176.51:19200"},
		"dev-pr-50":  {Name: "PR-50 (old)", Address: "198.244.176.51:19201"},
		"dev-pr-123": {Name: "PR-123 (old)", Address: "198.244.176.51:19204"},
	}

	register, unregister := Diff(current, desired)
	sort.Strings(unregister)
	sort.Slice(register, func(i, j int) bool {
		return register[i].Identifier < register[j].Identifier
	})

	if !reflect.DeepEqual(unregister, []string{"dev-pr-50"}) {
		t.Fatalf("unregister = %v, want [dev-pr-50]", unregister)
	}
	if len(register) != 1 {
		t.Fatalf("register len = %d, want 1 (name change for pr-123)", len(register))
	}
	if register[0].Identifier != "dev-pr-123" {
		t.Fatalf("register[0] = %s, want dev-pr-123", register[0].Identifier)
	}
	if register[0].Name != "PR-123 (feature/x)" {
		t.Fatalf("register name = %q", register[0].Name)
	}
	if !register[0].BetaLock {
		t.Fatal("expected BetaLock")
	}
}

func TestDiffEmptyCurrentRegistersAllRunning(t *testing.T) {
	desired, err := DesiredConfigs([]APIServer{
		{Name: "BETA", Type: "beta", Port: 19200, Status: "running"},
	}, "10.0.0.1")
	if err != nil {
		t.Fatal(err)
	}
	register, unregister := Diff(nil, desired)
	if len(unregister) != 0 {
		t.Fatalf("unregister = %v, want none", unregister)
	}
	if len(register) != 1 || register[0].Identifier != "dev-beta" {
		t.Fatalf("register = %+v", register)
	}
	if register[0].Address != "10.0.0.1:19200" {
		t.Fatalf("address = %s", register[0].Address)
	}
}

func TestDesiredConfigsFiltersNonRunning(t *testing.T) {
	desired, err := DesiredConfigs([]APIServer{
		{Name: "BETA", Type: "beta", Port: 1, Status: "running"},
		{Name: "GONE", Type: "beta", Port: 2, Status: "stopped"},
	}, "h")
	if err != nil {
		t.Fatal(err)
	}
	if len(desired) != 1 {
		t.Fatalf("desired len = %d", len(desired))
	}
	if _, ok := desired["dev-beta"]; !ok {
		t.Fatal("missing dev-beta")
	}
}

func TestDecodeResponse(t *testing.T) {
	raw := []byte(`{"servers":[{"name":"BETA","type":"beta","prNumber":null,"branch":"main","port":19200,"maxPlayers":40,"status":"running"},{"name":"PR-123","type":"pr","prNumber":123,"branch":"feature/x","port":19204,"maxPlayers":20,"status":"running"}]}`)
	resp, err := DecodeResponse(raw)
	if err != nil {
		t.Fatal(err)
	}
	if len(resp.Servers) != 2 {
		t.Fatalf("len = %d", len(resp.Servers))
	}
	if resp.Servers[0].Name != "BETA" || resp.Servers[0].Port != 19200 {
		t.Fatalf("beta = %+v", resp.Servers[0])
	}
	if resp.Servers[1].PRNumber == nil || *resp.Servers[1].PRNumber != 123 {
		t.Fatalf("pr = %+v", resp.Servers[1])
	}
}

func TestDiffNoChangeWhenIdentical(t *testing.T) {
	cfg := srv.Config{
		Name: "BETA", Identifier: "dev-beta", Address: "h:1", BetaLock: true,
	}
	register, unregister := Diff(
		map[string]Snapshot{"dev-beta": {Name: "BETA", Address: "h:1"}},
		map[string]srv.Config{"dev-beta": cfg},
	)
	if len(register) != 0 || len(unregister) != 0 {
		t.Fatalf("register=%v unregister=%v", register, unregister)
	}
}

func intPtr(v int) *int { return &v }
