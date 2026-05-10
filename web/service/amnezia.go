package service

import (
	"bytes"
	"compress/zlib"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/mhsanaei/3x-ui/v3/database"
	"github.com/mhsanaei/3x-ui/v3/database/model"
	"github.com/skip2/go-qrcode"
	"golang.org/x/crypto/curve25519"
)

var (
    ErrAmneziaServerNotFound = errors.New("amnezia server not found")
    ErrAmneziaPeerNotFound   = errors.New("amnezia peer not found")
)

const (
    DefaultAmneziaProtocolMode = "AmneziaWG"
    DefaultPeerAllowedIPs      = "0.0.0.0/0, ::/0"
    DefaultPeerKeepalive       = 25
)

type AmneziaService struct{}

func NewAmneziaService() *AmneziaService {
    return &AmneziaService{}
}

func (s *AmneziaService) GetAllServers() ([]model.AmneziaServer, error) {
    db := database.GetDB()
    var servers []model.AmneziaServer
    err := db.Preload("Peers").Order("id desc").Find(&servers).Error
    return servers, err
}

func (s *AmneziaService) GetServer(id int) (*model.AmneziaServer, error) {
    db := database.GetDB()
    server := &model.AmneziaServer{}
    err := db.Preload("Peers").First(server, id).Error
    if err != nil {
        if database.IsNotFound(err) {
            return nil, ErrAmneziaServerNotFound
        }
        return nil, err
    }
    return server, nil
}

func (s *AmneziaService) CreateServer(server *model.AmneziaServer) (*model.AmneziaServer, error) {
    if server.ProtocolMode == "" {
        server.ProtocolMode = DefaultAmneziaProtocolMode
    }
    if server.ObfuscationJSON == "" {
        server.ObfuscationJSON = "{}"
    }
    if server.PrivateKey == "" || server.PublicKey == "" {
        privateKey, publicKey, err := s.GenerateWireGuardKeyPair()
        if err != nil {
            return nil, err
        }
        server.PrivateKey = privateKey
        server.PublicKey = publicKey
    }
    if server.InterfaceName == "" {
        return nil, errors.New("interface name is required")
    }
    if server.ListenPort == 0 {
        return nil, errors.New("listen port is required")
    }
    if server.Address == "" {
        server.Address = "10.0.0.1/24" // Default subnet
    }
    if server.DNS == "" {
        server.DNS = "8.8.4.4"
    }
    if server.ServerType == "" {
        server.ServerType = "local" // Default: local server
    }
    
    // Auto-detect endpoint if enabled
    if server.AutoEndpoint && server.Endpoint == "" {
        endpoint, err := s.GetPublicEndpoint(server)
        if err != nil {
            // Log warning but don't fail - user can set manually
            fmt.Printf("Warning: failed to auto-detect endpoint: %v\n", err)
        } else {
            server.Endpoint = endpoint
        }
    }

    db := database.GetDB()
    if err := db.Create(server).Error; err != nil {
        return nil, err
    }
    return server, nil
}

func (s *AmneziaService) UpdateServer(server *model.AmneziaServer) (*model.AmneziaServer, error) {
    db := database.GetDB()
    existing := &model.AmneziaServer{}
    if err := db.First(existing, server.Id).Error; err != nil {
        if database.IsNotFound(err) {
            return nil, ErrAmneziaServerNotFound
        }
        return nil, err
    }
    if server.PrivateKey == "" {
        server.PrivateKey = existing.PrivateKey
    }
    if server.PublicKey == "" {
        server.PublicKey = existing.PublicKey
    }
    if server.ObfuscationJSON == "" {
        server.ObfuscationJSON = existing.ObfuscationJSON
    }
    if server.ProtocolMode == "" {
        server.ProtocolMode = existing.ProtocolMode
    }
    if err := db.Model(existing).Updates(server).Error; err != nil {
        return nil, err
    }
    return existing, nil
}

func (s *AmneziaService) DeleteServer(id int) error {
    db := database.GetDB()
    if err := db.Delete(&model.AmneziaPeer{}, "server_id = ?", id).Error; err != nil {
        return err
    }
    if err := db.Delete(&model.AmneziaServer{}, id).Error; err != nil {
        if database.IsNotFound(err) {
            return ErrAmneziaServerNotFound
        }
        return err
    }
    return nil
}

func (s *AmneziaService) GetPeers(serverId int) ([]model.AmneziaPeer, error) {
    db := database.GetDB()
    var peers []model.AmneziaPeer
    err := db.Where("server_id = ?", serverId).Order("id desc").Find(&peers).Error
    return peers, err
}

func (s *AmneziaService) GetPeer(id int) (*model.AmneziaPeer, error) {
    db := database.GetDB()
    peer := &model.AmneziaPeer{}
    err := db.First(peer, id).Error
    if err != nil {
        if database.IsNotFound(err) {
            return nil, ErrAmneziaPeerNotFound
        }
        return nil, err
    }
    return peer, nil
}

func (s *AmneziaService) CreatePeer(peer *model.AmneziaPeer) (*model.AmneziaPeer, error) {
    if peer.Name == "" {
        return nil, errors.New("peer name is required")
    }
    if peer.ServerID == 0 {
        return nil, errors.New("server id is required")
    }
    if peer.PrivateKey == "" && peer.PublicKey == "" {
        privateKey, publicKey, err := s.GenerateWireGuardKeyPair()
        if err != nil {
            return nil, err
        }
        peer.PrivateKey = privateKey
        peer.PublicKey = publicKey
    }
    if peer.PresharedKey == "" {
        presharedKey, err := s.GenerateWireGuardPresharedKey()
        if err != nil {
            return nil, err
        }
        peer.PresharedKey = presharedKey
    }
    if peer.AllowedIPs == "" {
        peer.AllowedIPs = DefaultPeerAllowedIPs
    }
    if peer.PersistentKeepalive == 0 {
        peer.PersistentKeepalive = DefaultPeerKeepalive
    }
    
    // Auto-assign IP address if not provided
    if peer.Address == "" {
        server, err := s.GetServer(peer.ServerID)
        if err != nil {
            return nil, fmt.Errorf("failed to get server: %w", err)
        }
        peer.Address, err = s.GetNextPeerIP(server)
        if err != nil {
            return nil, fmt.Errorf("failed to assign IP: %w", err)
        }
    }
    
    // Calculate expires_at from expiry_days
    peer.ExpiresAt = s.CalculatePeerExpiry(peer.ExpiryDays)
    // Clear pause fields on creation
    peer.PausedReason = nil
    peer.PausedAt = nil

    db := database.GetDB()
    if err := db.Create(peer).Error; err != nil {
        return nil, err
    }
    return peer, nil
}

func (s *AmneziaService) UpdatePeer(peer *model.AmneziaPeer) (*model.AmneziaPeer, error) {
    db := database.GetDB()
    existing := &model.AmneziaPeer{}
    if err := db.First(existing, peer.Id).Error; err != nil {
        if database.IsNotFound(err) {
            return nil, ErrAmneziaPeerNotFound
        }
        return nil, err
    }
    if peer.PrivateKey == "" {
        peer.PrivateKey = existing.PrivateKey
    }
    if peer.PublicKey == "" {
        peer.PublicKey = existing.PublicKey
    }
    if peer.PresharedKey == "" {
        peer.PresharedKey = existing.PresharedKey
    }
    if peer.AllowedIPs == "" {
        peer.AllowedIPs = existing.AllowedIPs
    }
    if peer.PersistentKeepalive == 0 {
        peer.PersistentKeepalive = existing.PersistentKeepalive
    }
    if err := db.Model(existing).Updates(peer).Error; err != nil {
        return nil, err
    }
    return existing, nil
}

func (s *AmneziaService) DeletePeer(id int) error {
    db := database.GetDB()
    if err := db.Delete(&model.AmneziaPeer{}, id).Error; err != nil {
        if database.IsNotFound(err) {
            return ErrAmneziaPeerNotFound
        }
        return err
    }
    return nil
}

func (s *AmneziaService) GenerateWireGuardKeyPair() (string, string, error) {
    priv, pub, err := generateWireGuardKeyPair()
    if err != nil {
        return "", "", err
    }
    return priv, pub, nil
}

func (s *AmneziaService) GenerateWireGuardPresharedKey() (string, error) {
    key := make([]byte, 32)
    if _, err := rand.Read(key); err != nil {
        return "", err
    }
    return base64.StdEncoding.EncodeToString(key), nil
}

func generateWireGuardKeyPair() (string, string, error) {
    priv := make([]byte, 32)
    if _, err := rand.Read(priv); err != nil {
        return "", "", err
    }
    priv[0] &= 248
    priv[31] &= 127
    priv[31] |= 64
    pub, err := curve25519.X25519(priv, curve25519.Basepoint)
    if err != nil {
        return "", "", err
    }
    return base64.StdEncoding.EncodeToString(priv), base64.StdEncoding.EncodeToString(pub), nil

}

func (s *AmneziaService) GetPeerConfig(peerId int) (string, error) {
    peer, err := s.GetPeer(peerId)
    if err != nil {
        return "", err
    }
    server, err := s.GetServer(peer.ServerID)
    if err != nil {
        return "", err
    }

    if peer.Address == "" {
        // Auto-assign IP if not set
        peer.Address, err = s.GetNextPeerIP(server)
        if err != nil {
            peer.Address = "10.0.0.2/32" // Fallback
        }
    }

    // Parse obfuscation parameters from server
    obf := s.parseObfuscationStruct(server.ObfuscationJSON)

    // Use template renderer instead of string concatenation
    config, err := RenderClientConfig(peer, server, obf)
    if err != nil {
        return "", fmt.Errorf("failed to render client config: %w", err)
    }

    return config, nil
}

func (s *AmneziaService) GetPeerQRCode(peerId int) (string, error) {
    config, err := s.GetPeerConfig(peerId)
    if err != nil {
        return "", err
    }
    png, err := qrcode.Encode(config, qrcode.Medium, 256)
    if err != nil {
        return "", err
    }
    return "data:image/png;base64," + base64.StdEncoding.EncodeToString(png), nil
}

func (s *AmneziaService) GetPeerStats(peerId int) (*model.AmneziaTrafficStat, error) {
    db := database.GetDB()
    stat := &model.AmneziaTrafficStat{}
    err := db.Where("peer_id = ?", peerId).First(stat).Error
    if err != nil {
        if database.IsNotFound(err) {
            return &model.AmneziaTrafficStat{PeerID: peerId}, nil
        }
        return nil, err
    }
    return stat, nil
}

func (s *AmneziaService) StartServer(id int) error {
    server, err := s.GetServer(id)
    if err != nil {
        return err
    }
    if err := s.systemctlAction("start", server.InterfaceName); err != nil {
        return err
    }
    server.Enabled = true
    return database.GetDB().Save(server).Error
}

func (s *AmneziaService) StopServer(id int) error {
    server, err := s.GetServer(id)
    if err != nil {
        return err
    }
    if err := s.systemctlAction("stop", server.InterfaceName); err != nil {
        return err
    }
    server.Enabled = false
    return database.GetDB().Save(server).Error
}

func (s *AmneziaService) RestartServer(id int) error {
    server, err := s.GetServer(id)
    if err != nil {
        return err
    }
    if err := s.systemctlAction("restart", server.InterfaceName); err != nil {
        return err
    }
    server.Enabled = true
    return database.GetDB().Save(server).Error
}

func (s *AmneziaService) systemctlAction(action, interfaceName string) error {
    unit := fmt.Sprintf("amneziawg@%s.service", interfaceName)
    if _, err := exec.LookPath("systemctl"); err != nil {
        return fmt.Errorf("systemctl not found")
    }
    cmd := exec.Command("systemctl", action, unit)
    output, err := cmd.CombinedOutput()
    if err != nil {
        return fmt.Errorf("%w: %s", err, strings.TrimSpace(string(output)))
    }
    return nil
}

// CalculatePeerExpiry computes the expiration timestamp from expiry days.
// Returns nil if expiryDays is nil or <= 0 (unlimited).
func (s *AmneziaService) CalculatePeerExpiry(expiryDays *int) *int64 {
    if expiryDays == nil || *expiryDays <= 0 {
        return nil
    }
    expiresAt := time.Now().AddDate(0, 0, *expiryDays).Unix()
    return &expiresAt
}

// GetActivePeersForConfig returns only enabled peers that have not expired.
// Expired or paused peers are excluded from the server config.
func (s *AmneziaService) GetActivePeersForConfig(serverId int) ([]model.AmneziaPeer, error) {
    db := database.GetDB()
    var peers []model.AmneziaPeer
    now := time.Now().Unix()
    err := db.Where("server_id = ? AND enabled = ? AND (expires_at IS NULL OR expires_at > ?)", serverId, true, now).
        Order("id asc").
        Find(&peers).Error
    return peers, err
}

// PauseExpiredPeers finds all enabled peers where expires_at <= now
// and sets them to disabled with paused_reason = "expired".
func (s *AmneziaService) PauseExpiredPeers() error {
    db := database.GetDB()
    var peers []model.AmneziaPeer
    now := time.Now().Unix()
    if err := db.Where("enabled = ? AND expires_at IS NOT NULL AND expires_at <= ?", true, now).
        Find(&peers).Error; err != nil {
        return err
    }
    for _, peer := range peers {
        reason := "expired"
        peer.Enabled = false
        peer.PausedReason = &reason
        peer.PausedAt = &now
        if err := db.Save(&peer).Error; err != nil {
            return err
        }
        // Rebuild server config to remove expired peer
        if err := s.RebuildServerConfig(peer.ServerID); err != nil {
            return err
        }
    }
    return nil
}

// ExtendPeer extends the expiration time of a peer by the given days.
// If the peer is currently expired, it will be re-enabled.
func (s *AmneziaService) ExtendPeer(peerId int, days int) error {
    if days <= 0 {
        return errors.New("days must be greater than zero")
    }
    peer, err := s.GetPeer(peerId)
    if err != nil {
        return err
    }
    now := time.Now()
    var base time.Time
    if peer.ExpiresAt != nil && time.Unix(*peer.ExpiresAt, 0).After(now) {
        base = time.Unix(*peer.ExpiresAt, 0)
    } else {
        base = now
    }
    expiresAt := base.AddDate(0, 0, days).Unix()
    peer.ExpiresAt = &expiresAt
    peer.Enabled = true
    peer.PausedReason = nil
    peer.PausedAt = nil
    db := database.GetDB()
    if err := db.Save(peer).Error; err != nil {
        return err
    }
    // Rebuild server config to include extended peer
    if err := s.RebuildServerConfig(peer.ServerID); err != nil {
        return err
    }
    return nil
}

// RebuildServerConfig regenerates the AmneziaWG server config with active peers.
// This is called after peer creation, update, deletion, or expiry changes.
func (s *AmneziaService) RebuildServerConfig(serverId int) error {
    server, err := s.GetServer(serverId)
    if err != nil {
        return fmt.Errorf("failed to get server: %w", err)
    }

    peers, err := s.GetActivePeersForConfig(serverId)
    if err != nil {
        return fmt.Errorf("failed to get active peers: %w", err)
    }

    // Parse obfuscation parameters from JSON
    obf := s.parseObfuscationStruct(server.ObfuscationJSON)

    // Validate AWG 2.0 parameters before generating config
    if err := s.ValidateAwg20Params(obf); err != nil {
        // Generate valid parameters if validation fails
        obf, _ = s.GenerateValidatedObfuscationParams()
    }

    // Use template renderer instead of string concatenation
    config, err := RenderServerConfig(server, peers, obf)
    if err != nil {
        return fmt.Errorf("failed to render server config: %w", err)
    }

    // Use /etc/amnezia/amneziawg/ directory for AmneziaWG configs
    configPath := fmt.Sprintf("/etc/amnezia/amneziawg/%s.conf", server.InterfaceName)

    if err := s.writeConfigFile(configPath, config); err != nil {
        return fmt.Errorf("failed to write config: %w", err)
    }

    // Restart the AmneziaWG service
    if err := s.RestartServer(serverId); err != nil {
        // Log warning but don't fail - config is written
        fmt.Printf("Warning: failed to restart server %d: %v\n", serverId, err)
    }

    return nil
}

// parseObfuscationStruct parses JSON obfuscation params into AmneziaObfuscation struct
func (s *AmneziaService) parseObfuscationStruct(jsonStr string) *model.AmneziaObfuscation {
    obf := &model.AmneziaObfuscation{}
    if jsonStr == "" || jsonStr == "{}" {
        // Return default values
        return s.GenerateObfuscationParams()
    }

    params := make(map[string]interface{})
    if err := json.Unmarshal([]byte(jsonStr), &params); err != nil {
        return s.GenerateObfuscationParams()
    }

    // Parse each field
    if v, ok := params["Jc"]; ok {
        if f, ok := v.(float64); ok {
            obf.Jc = int(f)
        }
    }
    if v, ok := params["Jmin"]; ok {
        if f, ok := v.(float64); ok {
            obf.Jmin = int(f)
        }
    }
    if v, ok := params["Jmax"]; ok {
        if f, ok := v.(float64); ok {
            obf.Jmax = int(f)
        }
    }
    if v, ok := params["S1"]; ok {
        if f, ok := v.(float64); ok {
            obf.S1 = int(f)
        }
    }
    if v, ok := params["S2"]; ok {
        if f, ok := v.(float64); ok {
            obf.S2 = int(f)
        }
    }
    if v, ok := params["S3"]; ok {
        if f, ok := v.(float64); ok {
            obf.S3 = int(f)
        }
    }
    if v, ok := params["S4"]; ok {
        if f, ok := v.(float64); ok {
            obf.S4 = int(f)
        }
    }
    if v, ok := params["H1"]; ok {
        if str, ok := v.(string); ok {
            obf.H1 = str
        }
    }
    if v, ok := params["H2"]; ok {
        if str, ok := v.(string); ok {
            obf.H2 = str
        }
    }
    if v, ok := params["H3"]; ok {
        if str, ok := v.(string); ok {
            obf.H3 = str
        }
    }
    if v, ok := params["H4"]; ok {
        if str, ok := v.(string); ok {
            obf.H4 = str
        }
    }
    if v, ok := params["I1"]; ok {
        if str, ok := v.(string); ok {
            obf.I1 = str
        }
    }
    if v, ok := params["I2"]; ok {
        if str, ok := v.(string); ok {
            obf.I2 = str
        }
    }
    if v, ok := params["I3"]; ok {
        if str, ok := v.(string); ok {
            obf.I3 = str
        }
    }
    if v, ok := params["I4"]; ok {
        if str, ok := v.(string); ok {
            obf.I4 = str
        }
    }
    if v, ok := params["I5"]; ok {
        if str, ok := v.(string); ok {
            obf.I5 = str
        }
    }

    return obf
}

// GenerateObfuscationParams creates random AmneziaWG 2.0 obfuscation parameters
// Based on: https://docs.amnezia.org/documentation/amnezia-wg/
func (s *AmneziaService) GenerateObfuscationParams() *model.AmneziaObfuscation {
    // AmneziaWG 2.0 recommended values based on official documentation
    // Jc: 4-12 (junk packet count)
    // Jmin-Jmax: 64-1024 (junk packet size range)
    // S1-S3: 0-64 (message padding for init/response/cookie)
    // S4: 0-32 (transport data padding)
    // H1-H4: uint32 ranges for dynamic headers
    // I1-I5: CPS format custom signature packets (optional)
    
    return &model.AmneziaObfuscation{
        Jc:   randomInt(4, 12),     // Junk packet count: 4-12
        Jmin: randomInt(64, 128),   // Min junk size: 64-128
        Jmax: randomInt(768, 1024), // Max junk size: 768-1024
        S1:   randomInt(0, 64),     // Init padding: 0-64
        S2:   randomInt(0, 64),     // Response padding: 0-64
        S3:   randomInt(0, 64),     // Cookie padding: 0-64
        S4:   randomInt(0, 32),     // Transport padding: 0-32
        H1:   generateHeaderRange(), // Init header range
        H2:   generateHeaderRange(), // Response header range
        H3:   generateHeaderRange(), // Cookie header range
        H4:   generateHeaderRange(), // Transport header range
        I1:   "", // Custom signature packets (optional)
        I2:   "",
        I3:   "",
        I4:   "",
        I5:   "",
    }
}

// generateHeaderRange creates a random uint32 header range for AmneziaWG 2.0
func generateHeaderRange() string {
    start := randomInt(0, 1000000)
    end := start + randomInt(100000, 200000)
    return fmt.Sprintf("%d-%d", start, end)
}

// randomInt generates a random integer between min and max (inclusive)
func randomInt(min, max int) int {
    if min >= max {
        return min
    }
    n := max - min + 1
    b := make([]byte, 1)
    rand.Read(b)
    return min + int(b[0])%n
}

// parseObfuscationParams extracts AmneziaWG 2.0 obfuscation parameters from JSON
func (s *AmneziaService) parseObfuscationParams(jsonStr string) map[string]interface{} {
    params := make(map[string]interface{})
    if jsonStr == "" || jsonStr == "{}" {
        return params
    }

    // Parse JSON obfuscation parameters
    // AmneziaWG 2.0 supports parameters based on official documentation:
    // - Jc (junk packet count): 4-12 recommended
    // - Jmin, Jmax (junk packet size): 64-1024 bytes
    // - S1, S2, S3, S4 (message padding): 0-64 for S1-S3, 0-32 for S4
    // - H1, H2, H3, H4 (dynamic headers): uint32 ranges
    // - I1-I5 (custom signature packets): CPS format (optional)
    if err := json.Unmarshal([]byte(jsonStr), &params); err != nil {
        fmt.Printf("Warning: failed to parse obfuscation params: %v\n", err)
        return params
    }

    return params
}

// writeConfigFile writes the configuration to the specified path
func (s *AmneziaService) writeConfigFile(path, content string) error {
    // Write to file with proper permissions (0600 for security)
    file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
    if err != nil {
        return err
    }
    defer file.Close()

    _, err = file.WriteString(content)
    return err
}

// ValidateAwg20Params validates AmneziaWG 2.0 obfuscation parameters
// Based on: https://github.com/bivlked/amneziawg-installer/blob/main/ADVANCED.en.md
// Critical constraints:
// - Jmax > Jmin (junk packet size range must be valid)
// - S3, S4 >= 0 (required for AWG 2.0)
// - H1-H4 ranges must not overlap
// - H* values must be <= 2147483647 (max safe int32 for compatibility)
// - S1 + 56 ≠ S2 (prevents init and response messages from having same size)
func (s *AmneziaService) ValidateAwg20Params(obf *model.AmneziaObfuscation) error {
    // Validate Jmax > Jmin
    if obf.Jmax <= obf.Jmin {
        return fmt.Errorf("Jmax (%d) must be greater than Jmin (%d)", obf.Jmax, obf.Jmin)
    }

    // Validate Jmin/Jmax ranges
    if obf.Jmin < 0 || obf.Jmin > 1280 {
        return fmt.Errorf("Jmin (%d) must be between 0 and 1280", obf.Jmin)
    }
    if obf.Jmax < 0 || obf.Jmax > 1280 {
        return fmt.Errorf("Jmax (%d) must be between 0 and 1280", obf.Jmax)
    }

    // Validate Jc range
    if obf.Jc < 1 || obf.Jc > 128 {
        return fmt.Errorf("Jc (%d) must be between 1 and 128", obf.Jc)
    }

    // Validate S3 and S4 (required for AWG 2.0)
    if obf.S3 < 0 {
        return fmt.Errorf("S3 (%d) must be >= 0", obf.S3)
    }
    if obf.S4 < 0 {
        return fmt.Errorf("S4 (%d) must be >= 0", obf.S4)
    }

    // Validate S1 + 56 ≠ S2 (prevents init and response messages from having same size)
    if obf.S1+56 == obf.S2 {
        return fmt.Errorf("S1 + 56 (%d) must not equal S2 (%d) - this would make init and response messages the same size", obf.S1+56, obf.S2)
    }

    // Validate H1-H4 ranges don't overlap
    ranges := []string{obf.H1, obf.H2, obf.H3, obf.H4}
    parsedRanges := make([][2]int, 0, 4)

    for i, h := range ranges {
        start, end, err := parseHeaderRange(h)
        if err != nil {
            return fmt.Errorf("H%d range parsing failed: %w", i+1, err)
        }
        // Validate max uint32/2 for safety (2147483647)
        if start > 2147483647 || end > 2147483647 {
            return fmt.Errorf("H%d range (%d-%d) exceeds maximum allowed value 2147483647", i+1, start, end)
        }
        parsedRanges = append(parsedRanges, [2]int{start, end})
    }

    // Check for overlapping ranges
    for i := 0; i < len(parsedRanges); i++ {
        for j := i + 1; j < len(parsedRanges); j++ {
            if rangesOverlap(parsedRanges[i], parsedRanges[j]) {
                return fmt.Errorf("H%d and H%d ranges overlap", i+1, j+1)
            }
        }
    }

    return nil
}

// parseHeaderRange parses a header range string like "100-200" or "150" (single value)
func parseHeaderRange(s string) (start, end int, err error) {
    s = strings.TrimSpace(s)
    if s == "" {
        return 0, 0, fmt.Errorf("empty range")
    }

    // Single value case
    if !strings.Contains(s, "-") {
        val, err := parseInt(s)
        if err != nil {
            return 0, 0, fmt.Errorf("invalid single value: %w", err)
        }
        return val, val, nil
    }

    // Range case
    parts := strings.Split(s, "-")
    if len(parts) != 2 {
        return 0, 0, fmt.Errorf("invalid range format, expected 'start-end'")
    }

    start, err = parseInt(parts[0])
    if err != nil {
        return 0, 0, fmt.Errorf("invalid start: %w", err)
    }
    end, err = parseInt(parts[1])
    if err != nil {
        return 0, 0, fmt.Errorf("invalid end: %w", err)
    }

    if start > end {
        return 0, 0, fmt.Errorf("start (%d) greater than end (%d)", start, end)
    }

    return start, end, nil
}

// parseInt parses an integer from a string
func parseInt(s string) (int, error) {
    s = strings.TrimSpace(s)
    var val int
    _, err := fmt.Sscanf(s, "%d", &val)
    return val, err
}

// rangesOverlap checks if two ranges [a1, a2] and [b1, b2] overlap
func rangesOverlap(a, b [2]int) bool {
    return a[0] <= b[1] && b[0] <= a[1]
}

// GenerateValidatedObfuscationParams generates and validates AWG 2.0 parameters
func (s *AmneziaService) GenerateValidatedObfuscationParams() (*model.AmneziaObfuscation, error) {
    // Try up to 10 times to generate valid non-overlapping parameters
    for i := 0; i < 10; i++ {
        obf := s.GenerateObfuscationParams()
        if err := s.ValidateAwg20Params(obf); err == nil {
            return obf, nil
        }
    }
    // Fallback to deterministic non-overlapping values
    return &model.AmneziaObfuscation{
        Jc:   5,
        Jmin: 50,
        Jmax: 200,
        S1:   72,
        S2:   56,  // S1 + 56 = 128 ≠ 56
        S3:   32,
        S4:   16,
        H1:   "100000-200000",
        H2:   "300000-400000",
        H3:   "500000-600000",
        H4:   "700000-800000",
        I1:   "",
        I2:   "",
        I3:   "",
        I4:   "",
        I5:   "",
    }, nil
}

// GetPeerVPNURI generates a vpn:// URI for Amnezia VPN app import
// Based on: https://github.com/bivlked/amneziawg-installer/blob/main/ADVANCED.en.md
// The URI format is: vpn://<base64-encoded-zlib-compressed-config>
func (s *AmneziaService) GetPeerVPNURI(peerId int) (string, error) {
    // Get the standard config first
    config, err := s.GetPeerConfig(peerId)
    if err != nil {
        return "", err
    }

    // Compress with zlib and encode with base64
    var buf bytes.Buffer
    w := zlib.NewWriter(&buf)
    if _, err := w.Write([]byte(config)); err != nil {
        return "", fmt.Errorf("failed to compress config: %w", err)
    }
    if err := w.Close(); err != nil {
        return "", fmt.Errorf("failed to close zlib writer: %w", err)
    }

    // Create vpn:// URI
    encoded := base64.StdEncoding.EncodeToString(buf.Bytes())
    return "vpn://" + encoded, nil
}

// GetPeerVPNURICode generates a QR code for the vpn:// URI
func (s *AmneziaService) GetPeerVPNURICode(peerId int) (string, error) {
    vpnURI, err := s.GetPeerVPNURI(peerId)
    if err != nil {
        return "", err
    }

    png, err := qrcode.Encode(vpnURI, qrcode.Medium, 256)
    if err != nil {
        return "", err
    }
    return "data:image/png;base64," + base64.StdEncoding.EncodeToString(png), nil
}

// GetNextPeerIP assigns the next available IP address for a peer
// based on the server's subnet configuration
func (s *AmneziaService) GetNextPeerIP(server *model.AmneziaServer) (string, error) {
    // Parse server address to get subnet
    // Format: 10.0.0.1/24 or 192.168.1.1/24
    serverAddr := server.Address
    if serverAddr == "" {
        serverAddr = "10.0.0.1/24" // Default
    }
    
    // Extract network portion
    parts := strings.Split(serverAddr, "/")
    if len(parts) != 2 {
        return "", fmt.Errorf("invalid server address format: %s", serverAddr)
    }
    
    serverIP := parts[0]
    cidr := parts[1]
    
    // Get all existing peers for this server
    peers, err := s.GetPeers(server.Id)
    if err != nil {
        return "", fmt.Errorf("failed to get peers: %w", err)
    }
    
    // Build map of used IPs
    usedIPs := make(map[string]bool)
    usedIPs[serverIP] = true // Server IP is reserved
    
    for _, peer := range peers {
        if peer.Address != "" {
            // Extract IP from peer.Address (format: 10.0.0.2/32)
            peerIP := strings.Split(peer.Address, "/")[0]
            usedIPs[peerIP] = true
        }
    }
    
    // Generate next available IP based on CIDR
    // For /24 subnet: 10.0.0.1 is server, peers start from 10.0.0.2
    ipParts := strings.Split(serverIP, ".")
    if len(ipParts) != 4 {
        return "", fmt.Errorf("invalid IPv4 address: %s", serverIP)
    }
    
    // Get the base (first 3 octets)
    base := strings.Join(ipParts[:3], ".")
    
    // Find next available IP (start from 2, skip server at 1)
    var lastOctet int
    fmt.Sscanf(ipParts[3], "%d", &lastOctet)
    
    // Determine max based on CIDR
    maxIP := 254
    if cidr == "/24" {
        maxIP = 254
    } else if cidr == "/25" {
        maxIP = 126
    } else if cidr == "/26" {
        maxIP = 62
    } else if cidr == "/27" {
        maxIP = 30
    } else if cidr == "/28" {
        maxIP = 14
    }
    
    // Find next available IP
    for i := 2; i <= maxIP; i++ {
        candidate := fmt.Sprintf("%s.%d", base, i)
        if !usedIPs[candidate] {
            return candidate + "/32", nil
        }
    }
    
    return "", fmt.Errorf("no available IP addresses in subnet %s", serverAddr)
}

// GetPublicEndpoint returns the public IP/domain and port for the server
// If AutoEndpoint is enabled, it attempts to detect the public IP
func (s *AmneziaService) GetPublicEndpoint(server *model.AmneziaServer) (string, error) {
    // If endpoint is already set and not auto-detect, use it
    if server.Endpoint != "" && !server.AutoEndpoint {
        return server.Endpoint, nil
    }
    
    // Try to get public IP
    publicIP, err := s.getPublicIP()
    if err != nil {
        // If we can't get public IP, use the server's address as fallback
        if server.Address != "" {
            ip := strings.Split(server.Address, "/")[0]
            return fmt.Sprintf("%s:%d", ip, server.ListenPort), nil
        }
        return "", fmt.Errorf("failed to get public IP: %w", err)
    }
    
    return fmt.Sprintf("%s:%d", publicIP, server.ListenPort), nil
}

// getPublicIP attempts to get the server's public IP address
func (s *AmneziaService) getPublicIP() (string, error) {
    // Try multiple services for redundancy
    services := []string{
        "https://api.ipify.org",
        "https://ifconfig.me",
        "https://icanhazip.com",
    }
    
    for _, service := range services {
        cmd := exec.Command("curl", "-s", "-4", "--connect-timeout", "5", service)
        output, err := cmd.Output()
        if err == nil {
            ip := strings.TrimSpace(string(output))
            if ip != "" && len(ip) < 16 { // Basic validation
                return ip, nil
            }
        }
    }
    
    return "", fmt.Errorf("failed to get public IP from any service")
}
