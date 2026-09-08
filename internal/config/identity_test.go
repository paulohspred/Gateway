package config

import "testing"

func TestBridgeCoreDoesNotRequireDeviceInventory(t *testing.T) {
	path := writeConfig(t, `{
		"schema":3,
		"nodeId":"test-node",
		"tunnels":[{
			"id":"raw",
			"field":{"mode":"listen","bind":"127.0.0.1:15001"},
			"consumer":{"mode":"listen","bind":"127.0.0.1:25001"}
		}]
	}`)
	if _, err := Load(path); err != nil {
		t.Fatalf("bridge core must not require a device inventory: %v", err)
	}
}
