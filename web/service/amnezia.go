package service

import (
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
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
        return nil, errors.New("address is required")
    }
    if server.DNS == "" {
        server.DNS = "8.8.4.4"
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
        peer.Address = "10.0.0.2/32"
    }

    builder := strings.Builder{}
    builder.WriteString("[Interface]\n")
    builder.WriteString(fmt.Sprintf("PrivateKey = %s\n", peer.PrivateKey))
    if peer.Address != "" {
        builder.WriteString(fmt.Sprintf("Address = %s\n", peer.Address))
    }
    if server.DNS != "" {
        builder.WriteString(fmt.Sprintf("DNS = %s\n", server.DNS))
    }
    if server.MTU > 0 {
        builder.WriteString(fmt.Sprintf("MTU = %d\n", server.MTU))
    }
    builder.WriteString("\n[Peer]\n")
    builder.WriteString(fmt.Sprintf("PublicKey = %s\n", server.PublicKey))
    if peer.PresharedKey != "" {
        builder.WriteString(fmt.Sprintf("PresharedKey = %s\n", peer.PresharedKey))
    }
    endpoint := peer.Endpoint
    if endpoint == "" {
        endpoint = server.Endpoint
    }
    if endpoint != "" {
        builder.WriteString(fmt.Sprintf("Endpoint = %s\n", endpoint))
    }
    if peer.AllowedIPs != "" {
        builder.WriteString(fmt.Sprintf("AllowedIPs = %s\n", peer.AllowedIPs))
    }
    if peer.PersistentKeepalive > 0 {
        builder.WriteString(fmt.Sprintf("PersistentKeepalive = %d\n", peer.PersistentKeepalive))
    }
    return builder.String(), nil
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
    // For now, this is a placeholder. Full implementation would:
    // 1. Get server and active peers
    // 2. Generate AmneziaWG config file
    // 3. Write to /etc/amneziawg/<interface>.conf
    // 4. Reload/restart the amneziawg service
    // The actual config generation and service management will be
    // implemented in a separate config manager module.
    return nil
}
