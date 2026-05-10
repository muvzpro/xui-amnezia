package service

import (
	"testing"
	"time"

	"github.com/mhsanaei/3x-ui/v3/database/model"
)

// TestGenerateWireGuardKeyPair tests the WireGuard key generation
func TestGenerateWireGuardKeyPair(t *testing.T) {
	service := NewAmneziaService()

	privateKey, publicKey, err := service.GenerateWireGuardKeyPair()
	if err != nil {
		t.Fatalf("Failed to generate key pair: %v", err)
	}

	if privateKey == "" {
		t.Error("Private key should not be empty")
	}

	if publicKey == "" {
		t.Error("Public key should not be empty")
	}

	// Keys should be base64 encoded and 44 characters (32 bytes base64)
	if len(privateKey) != 44 {
		t.Errorf("Private key length should be 44, got %d", len(privateKey))
	}

	if len(publicKey) != 44 {
		t.Errorf("Public key length should be 44, got %d", len(publicKey))
	}

	// Generate another pair and ensure they're different
	privateKey2, publicKey2, err := service.GenerateWireGuardKeyPair()
	if err != nil {
		t.Fatalf("Failed to generate second key pair: %v", err)
	}

	if privateKey == privateKey2 {
		t.Error("Private keys should be unique")
	}

	if publicKey == publicKey2 {
		t.Error("Public keys should be unique")
	}
}

// TestGenerateWireGuardPresharedKey tests the preshared key generation
func TestGenerateWireGuardPresharedKey(t *testing.T) {
	service := NewAmneziaService()

	key, err := service.GenerateWireGuardPresharedKey()
	if err != nil {
		t.Fatalf("Failed to generate preshared key: %v", err)
	}

	if key == "" {
		t.Error("Preshared key should not be empty")
	}

	// Key should be base64 encoded and 44 characters (32 bytes base64)
	if len(key) != 44 {
		t.Errorf("Preshared key length should be 44, got %d", len(key))
	}

	// Generate another key and ensure it's different
	key2, err := service.GenerateWireGuardPresharedKey()
	if err != nil {
		t.Fatalf("Failed to generate second preshared key: %v", err)
	}

	if key == key2 {
		t.Error("Preshared keys should be unique")
	}
}

// TestCalculatePeerExpiry tests the expiry calculation
func TestCalculatePeerExpiry(t *testing.T) {
	service := NewAmneziaService()

	// Test with nil expiry days (unlimited)
	result := service.CalculatePeerExpiry(nil)
	if result != nil {
		t.Error("Expected nil for nil expiry days")
	}

	// Test with zero expiry days (unlimited)
	zeroDays := 0
	result = service.CalculatePeerExpiry(&zeroDays)
	if result != nil {
		t.Error("Expected nil for zero expiry days")
	}

	// Test with negative expiry days (unlimited)
	negDays := -1
	result = service.CalculatePeerExpiry(&negDays)
	if result != nil {
		t.Error("Expected nil for negative expiry days")
	}

	// Test with valid expiry days
	days := 30
	result = service.CalculatePeerExpiry(&days)
	if result == nil {
		t.Fatal("Expected non-nil result for valid expiry days")
	}

	// Verify the timestamp is approximately 30 days from now
	expectedExpiry := time.Now().AddDate(0, 0, 30).Unix()
	tolerance := int64(60) // 60 seconds tolerance for test execution time

	diff := *result - expectedExpiry
	if diff < -tolerance || diff > tolerance {
		t.Errorf("Expiry timestamp difference too large: %d seconds", diff)
	}

	// Test with 1 day
	oneDay := 1
	result = service.CalculatePeerExpiry(&oneDay)
	if result == nil {
		t.Fatal("Expected non-nil result for 1 day expiry")
	}

	expectedExpiry = time.Now().AddDate(0, 0, 1).Unix()
	diff = *result - expectedExpiry
	if diff < -tolerance || diff > tolerance {
		t.Errorf("1 day expiry timestamp difference too large: %d seconds", diff)
	}
}

// TestParseObfuscationParams tests the obfuscation parameter parsing
func TestParseObfuscationParams(t *testing.T) {
	service := NewAmneziaService()

	// Test with empty string
	params := service.parseObfuscationParams("")
	if len(params) != 0 {
		t.Error("Expected empty params for empty string")
	}

	// Test with empty JSON object
	params = service.parseObfuscationParams("{}")
	if len(params) != 0 {
		t.Error("Expected empty params for empty JSON object")
	}

	// Test with valid JSON
	jsonStr := `{"Jc": 3, "Jmin": 50, "Jmax": 1000, "S1": 15, "S2": 30, "H1": 15, "H2": 30}`
	params = service.parseObfuscationParams(jsonStr)
	if len(params) == 0 {
		t.Error("Expected non-empty params for valid JSON")
	}

	// Check specific values
	if jc, ok := params["Jc"].(float64); !ok || jc != 3 {
		t.Errorf("Expected Jc=3, got %v", params["Jc"])
	}

	if jmin, ok := params["Jmin"].(float64); !ok || jmin != 50 {
		t.Errorf("Expected Jmin=50, got %v", params["Jmin"])
	}

	// Test with invalid JSON (should return empty params, not crash)
	params = service.parseObfuscationParams("invalid json")
	// Function should handle error gracefully
}

func TestGenerateValidatedObfuscationParamsNoHeaderOverlap(t *testing.T) {
	service := NewAmneziaService()

	for i := 0; i < 50; i++ {
		obf, err := service.GenerateValidatedObfuscationParams()
		if err != nil {
			t.Fatalf("GenerateValidatedObfuscationParams returned error: %v", err)
		}
		if err := service.ValidateAwg20Params(obf); err != nil {
			t.Fatalf("generated obfuscation params should be valid: %v", err)
		}
	}
}

// TestGenerateServerConfig tests the server config generation
func TestGenerateServerConfig(t *testing.T) {
	service := NewAmneziaService()

	server := &model.AmneziaServer{
		Id:              1,
		Name:            "Test Server",
		InterfaceName:   "amnezia0",
		ListenPort:      51820,
		PrivateKey:      "privateKeyBase64String",
		PublicKey:       "publicKeyBase64String",
		Address:         "10.0.0.1/24",
		DNS:             "8.8.4.4",
		MTU:             1420,
		ProtocolMode:    "AmneziaWG",
		ObfuscationJSON: `{"Jc": 3, "Jmin": 50}`,
		Enabled:         true,
	}

	peers := []model.AmneziaPeer{
		{
			Id:           1,
			ServerID:     1,
			Name:         "Peer 1",
			PrivateKey:   "peerPrivateKey",
			PublicKey:    "peerPublicKey",
			PresharedKey: "presharedKey",
			Address:      "10.0.0.2/32",
			AllowedIPs:   "0.0.0.0/0, ::/0",
			Enabled:      true,
		},
	}

	config := service.generateServerConfig(server, peers)

	// Verify config contains expected sections
	if !contains(config, "[Interface]") {
		t.Error("Config should contain [Interface] section")
	}

	if !contains(config, "[Peer]") {
		t.Error("Config should contain [Peer] section")
	}

	if !contains(config, "PrivateKey = privateKeyBase64String") {
		t.Error("Config should contain server private key")
	}

	if !contains(config, "ListenPort = 51820") {
		t.Error("Config should contain listen port")
	}

	if !contains(config, "Address = 10.0.0.1/24") {
		t.Error("Config should contain server address")
	}

	if contains(config, "DNS = 8.8.4.4") {
		t.Error("Server config should not contain DNS; awg-quick may fail without resolvconf")
	}

	if !contains(config, "MTU = 1420") {
		t.Error("Config should contain MTU")
	}

	if !contains(config, "SaveConfig = false") {
		t.Error("Config should disable awg-quick SaveConfig")
	}

	if !contains(config, "PostUp =") || !contains(config, "10.0.0.0/24") {
		t.Error("Config should contain NAT PostUp with masked source CIDR")
	}

	if !contains(config, "PublicKey = peerPublicKey") {
		t.Error("Config should contain peer public key")
	}

	if !contains(config, "PresharedKey = presharedKey") {
		t.Error("Config should contain preshared key")
	}

	// Test with empty peers
	emptyConfig := service.generateServerConfig(server, []model.AmneziaPeer{})
	if !contains(emptyConfig, "[Interface]") {
		t.Error("Config should still contain [Interface] section with empty peers")
	}

	if contains(emptyConfig, "[Peer]") {
		t.Error("Config should not contain [Peer] section with empty peers")
	}
}

// TestGenerateServerConfigWithoutObfuscation tests config generation without obfuscation
func TestGenerateServerConfigWithoutObfuscation(t *testing.T) {
	service := NewAmneziaService()

	server := &model.AmneziaServer{
		Id:              1,
		Name:            "Test Server",
		InterfaceName:   "amnezia0",
		ListenPort:      52281,
		PrivateKey:      "privateKey",
		PublicKey:       "publicKey",
		Address:         "10.0.0.1/24",
		ObfuscationJSON: "", // Empty obfuscation
		Enabled:         true,
	}

	peers := []model.AmneziaPeer{}

	config := service.generateServerConfig(server, peers)

	// Should not contain obfuscation parameters
	if contains(config, "Jc") {
		t.Error("Config should not contain obfuscation params when empty")
	}
}

func TestRenderServerConfigSkipsPeersWithoutAddress(t *testing.T) {
	server := &model.AmneziaServer{
		InterfaceName: "awg0",
		ListenPort:    51820,
		PrivateKey:    "privateKey",
		Address:       "10.0.0.1/24",
	}
	obf := &model.AmneziaObfuscation{
		Jc:   5,
		Jmin: 50,
		Jmax: 200,
		S1:   72,
		S2:   56,
		S3:   32,
		S4:   16,
		H1:   "100000-200000",
		H2:   "300000-400000",
		H3:   "500000-600000",
		H4:   "700000-800000",
	}
	peers := []model.AmneziaPeer{{
		Name:      "empty-address",
		PublicKey: "peerPublicKey",
		Enabled:   true,
	}}

	config, err := RenderServerConfig(server, peers, obf)
	if err != nil {
		t.Fatalf("RenderServerConfig failed: %v", err)
	}
	if contains(config, "AllowedIPs = \n") || contains(config, "AllowedIPs = /32") {
		t.Error("Config should not emit an invalid empty peer AllowedIPs")
	}
}

func TestParseAwgDump(t *testing.T) {
	dump := "awg0\tserver-private\tserver-public\t51820\toff\n" +
		"awg0\tpeerPublic\tpsk\t1.2.3.4:5555\t10.0.0.2/32\t1710000000\t1234\t5678\t25\n"

	peers := parseAwgDump(dump)
	peer, ok := peers["peerPublic"]
	if !ok {
		t.Fatal("expected peer runtime row")
	}
	if peer.Interface != "awg0" {
		t.Errorf("expected interface awg0, got %s", peer.Interface)
	}
	if peer.Handshake != 1710000000 {
		t.Errorf("expected handshake 1710000000, got %d", peer.Handshake)
	}
	if peer.ReceiveBytes != 1234 || peer.TransmitBytes != 5678 {
		t.Errorf("unexpected transfer counters: rx=%d tx=%d", peer.ReceiveBytes, peer.TransmitBytes)
	}
}

// Helper function to check if string contains substring
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsHelper(s, substr))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
