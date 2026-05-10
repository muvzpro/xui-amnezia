package service

import (
	"bytes"
	"compress/zlib"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"net/netip"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
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
	AmneziaOnlineWindow        = 3 * time.Minute
	amneziaConfigDir           = "/etc/amnezia/amneziawg"
	amneziaSystemdUnitPath     = "/etc/systemd/system/amneziawg@.service"
	amneziaRuntimeDocker       = "docker"
	amneziaDockerContainerEnv  = "XUI_AMNEZIA_DOCKER_CONTAINER"
	amneziaRuntimeEnv          = "XUI_AMNEZIA_RUNTIME"
	amneziaDefaultContainer    = "3xui_amneziawg"
	amneziaHelperPath          = "/usr/local/bin/awg-helper"
)

type AmneziaService struct{}

type AmneziaRuntimePeer struct {
	Peer           model.AmneziaPeer        `json:"peer"`
	Stat           model.AmneziaTrafficStat `json:"stat"`
	Online         bool                     `json:"online"`
	Usage          int64                    `json:"usage"`
	TrafficLimited bool                     `json:"trafficLimited"`
	Expired        bool                     `json:"expired"`
}

type AmneziaRuntimeServer struct {
	Server  model.AmneziaServer  `json:"server"`
	Peers   []AmneziaRuntimePeer `json:"peers"`
	Running bool                 `json:"running"`
	Up      int64                `json:"up"`
	Down    int64                `json:"down"`
	Online  int                  `json:"online"`
}

type AmneziaRuntimeSnapshot struct {
	Servers []AmneziaRuntimeServer `json:"servers"`
	Time    int64                  `json:"time"`
}

type awgPeerRuntime struct {
	Interface     string
	PublicKey     string
	Handshake     int64
	ReceiveBytes  int64
	TransmitBytes int64
}

func NewAmneziaService() *AmneziaService {
	return &AmneziaService{}
}

func (s *AmneziaService) PrepareRuntime() error {
	if err := s.ensureSystemdTemplate(); err != nil {
		fmt.Printf("Warning: failed to ensure AmneziaWG systemd template: %v\n", err)
	}
	if err := s.SyncLocalConfigFiles(); err != nil {
		return err
	}
	if err := s.ensureDefaultServer(); err != nil {
		return err
	}
	servers, err := s.GetAllServers()
	if err != nil {
		return err
	}
	for i := range servers {
		server := &servers[i]
		if !server.Enabled {
			continue
		}
		if _, err := s.writeServerConfig(server.Id); err != nil {
			fmt.Printf("Warning: failed to write AmneziaWG config for %s: %v\n", server.InterfaceName, err)
			continue
		}
		_ = s.systemctlAction("enable", server.InterfaceName)
		if !s.IsServerRunning(server.InterfaceName) {
			if err := s.systemctlAction("start", server.InterfaceName); err != nil {
				fmt.Printf("Warning: failed to start AmneziaWG server %s: %v\n", server.InterfaceName, err)
			}
		}
	}
	return nil
}

func (s *AmneziaService) ensureDefaultServer() error {
	db := database.GetDB()
	var count int64
	if err := db.Model(&model.AmneziaServer{}).Count(&count).Error; err != nil {
		return err
	}
	if count > 0 {
		return nil
	}
	privateKey, publicKey, err := s.GenerateWireGuardKeyPair()
	if err != nil {
		return err
	}
	obf, err := s.GenerateValidatedObfuscationParams()
	if err != nil {
		return err
	}
	obfJSON, _ := json.Marshal(obf)
	server := &model.AmneziaServer{
		Name:            "AmneziaWG awg0",
		InterfaceName:   "awg0",
		ListenPort:      randomInt(20000, 62000),
		PrivateKey:      privateKey,
		PublicKey:       publicKey,
		Address:         fmt.Sprintf("10.%d.%d.1/24", randomInt(10, 250), randomInt(0, 250)),
		DNS:             "8.8.8.8, 1.1.1.1",
		MTU:             1280,
		ProtocolMode:    DefaultAmneziaProtocolMode,
		ObfuscationJSON: string(obfJSON),
		ServerType:      "local",
		AutoEndpoint:    true,
		Enabled:         true,
	}
	created, err := s.CreateServer(server)
	if err != nil {
		return err
	}
	_, err = s.writeServerConfig(created.Id)
	return err
}

func (s *AmneziaService) ensureSystemdTemplate() error {
	if _, err := exec.LookPath("systemctl"); err != nil {
		return nil
	}
	unit := `[Unit]
Description=AmneziaWG interface %i
After=network-online.target
Wants=network-online.target

[Service]
Type=oneshot
RemainAfterExit=yes
Environment=PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin
Environment=WG_QUICK_USERSPACE_IMPLEMENTATION=amneziawg-go
ExecStart=/bin/sh -c 'exec awg-quick up /etc/amnezia/amneziawg/%i.conf'
ExecStop=/bin/sh -c 'exec awg-quick down /etc/amnezia/amneziawg/%i.conf'
ExecReload=/bin/sh -c 'awg-quick down /etc/amnezia/amneziawg/%i.conf || true; exec awg-quick up /etc/amnezia/amneziawg/%i.conf'

[Install]
WantedBy=multi-user.target
`
	if current, err := os.ReadFile(amneziaSystemdUnitPath); err == nil && string(current) == unit {
		return nil
	}
	if err := os.WriteFile(amneziaSystemdUnitPath, []byte(unit), 0644); err != nil {
		return err
	}
	_ = exec.Command("systemctl", "daemon-reload").Run()
	return nil
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
	if server.ServerType != "local" {
		return nil, errors.New("remote AmneziaWG servers are not supported yet")
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
	if server.ServerType == "" {
		server.ServerType = existing.ServerType
	}
	if server.ServerType != "local" {
		return nil, errors.New("remote AmneziaWG servers are not supported yet")
	}
	updates := map[string]any{
		"name":             server.Name,
		"interface_name":   server.InterfaceName,
		"listen_port":      server.ListenPort,
		"private_key":      server.PrivateKey,
		"public_key":       server.PublicKey,
		"address":          server.Address,
		"dns":              server.DNS,
		"endpoint":         server.Endpoint,
		"mtu":              server.MTU,
		"protocol_mode":    server.ProtocolMode,
		"obfuscation_json": server.ObfuscationJSON,
		"server_type":      server.ServerType,
		"auto_endpoint":    server.AutoEndpoint,
		"enabled":          server.Enabled,
	}
	if err := db.Model(existing).Updates(updates).Error; err != nil {
		return nil, err
	}
	if err := db.First(existing, server.Id).Error; err != nil {
		return nil, err
	}
	if _, err := s.writeServerConfig(existing.Id); err != nil {
		return nil, err
	}
	running := s.IsServerRunning(existing.InterfaceName)
	if !existing.Enabled && running {
		if err := s.systemctlAction("stop", existing.InterfaceName); err != nil {
			return nil, err
		}
	} else if existing.Enabled || running {
		if err := s.systemctlAction("restart", existing.InterfaceName); err != nil {
			return nil, err
		}
	}
	return existing, nil
}

func (s *AmneziaService) DeleteServer(id int) error {
	db := database.GetDB()
	server, err := s.GetServer(id)
	if err != nil {
		return err
	}
	_ = s.systemctlAction("stop", server.InterfaceName)
	_ = s.systemctlAction("disable", server.InterfaceName)
	configPath := filepath.Join(amneziaConfigDir, fmt.Sprintf("%s.conf", server.InterfaceName))
	if err := os.Remove(configPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to remove AmneziaWG config %s: %w", configPath, err)
	}
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
	if err := s.RebuildServerConfig(peer.ServerID); err != nil {
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
	if strings.TrimSpace(peer.Address) == "" {
		peer.Address = existing.Address
	}
	if peer.ExpiryDays == nil {
		peer.ExpiryDays = existing.ExpiryDays
		peer.ExpiresAt = existing.ExpiresAt
	} else {
		peer.ExpiresAt = s.CalculatePeerExpiry(peer.ExpiryDays)
	}
	if peer.Enabled {
		peer.PausedReason = nil
		peer.PausedAt = nil
	} else {
		peer.PausedReason = existing.PausedReason
		peer.PausedAt = existing.PausedAt
	}
	updates := map[string]any{
		"server_id":            existing.ServerID,
		"name":                 peer.Name,
		"private_key":          peer.PrivateKey,
		"public_key":           peer.PublicKey,
		"preshared_key":        peer.PresharedKey,
		"address":              peer.Address,
		"allowed_ips":          peer.AllowedIPs,
		"endpoint":             peer.Endpoint,
		"persistent_keepalive": peer.PersistentKeepalive,
		"enabled":              peer.Enabled,
		"expiry_days":          peer.ExpiryDays,
		"expires_at":           peer.ExpiresAt,
		"paused_reason":        peer.PausedReason,
		"paused_at":            peer.PausedAt,
		"traffic_limit":        peer.TrafficLimit,
		"expiry_time":          peer.ExpiryTime,
	}
	if err := db.Model(existing).Updates(updates).Error; err != nil {
		return nil, err
	}
	if err := s.RebuildServerConfig(existing.ServerID); err != nil {
		return nil, err
	}
	return existing, nil
}

func (s *AmneziaService) DeletePeer(id int) error {
	db := database.GetDB()
	peer, err := s.GetPeer(id)
	if err != nil {
		return err
	}
	if err := db.Delete(&model.AmneziaPeer{}, id).Error; err != nil {
		if database.IsNotFound(err) {
			return ErrAmneziaPeerNotFound
		}
		return err
	}
	if err := s.RebuildServerConfig(peer.ServerID); err != nil {
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
		_ = database.GetDB().Model(peer).Update("address", peer.Address).Error
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
	if len(config) == 0 {
		return "", errors.New("empty peer config")
	}
	// Limit config size for QR code (max ~2953 bytes for version 40)
	if len(config) > 2800 {
		config = config[:2800]
	}
	png, err := qrcode.Encode(config, qrcode.Medium, 256)
	if err != nil {
		return "", fmt.Errorf("qrcode encode failed: %w", err)
	}
	if len(png) == 0 {
		return "", errors.New("qrcode encode returned empty data")
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

func (s *AmneziaService) GetRuntimeSnapshot() (*AmneziaRuntimeSnapshot, error) {
	_ = s.SyncLocalConfigFiles()
	servers, err := s.GetAllServers()
	if err != nil {
		return nil, err
	}
	snapshot := &AmneziaRuntimeSnapshot{
		Servers: make([]AmneziaRuntimeServer, 0, len(servers)),
		Time:    time.Now().Unix(),
	}
	now := time.Now()
	for _, server := range servers {
		row := AmneziaRuntimeServer{
			Server:  server,
			Running: s.IsServerRunning(server.InterfaceName),
			Peers:   make([]AmneziaRuntimePeer, 0, len(server.Peers)),
		}
		for _, peer := range server.Peers {
			stat, err := s.GetPeerStats(peer.Id)
			if err != nil {
				return nil, err
			}
			usage := stat.RxBytes + stat.TxBytes
			online := stat.LastHandshake > 0 && now.Sub(time.Unix(stat.LastHandshake, 0)) <= AmneziaOnlineWindow
			expired := peer.ExpiresAt != nil && *peer.ExpiresAt <= now.Unix()
			trafficLimited := peer.TrafficLimit > 0 && usage >= peer.TrafficLimit
			if online {
				row.Online++
			}
			row.Down += stat.RxBytes
			row.Up += stat.TxBytes
			row.Peers = append(row.Peers, AmneziaRuntimePeer{
				Peer:           peer,
				Stat:           *stat,
				Online:         online,
				Usage:          usage,
				TrafficLimited: trafficLimited,
				Expired:        expired,
			})
		}
		snapshot.Servers = append(snapshot.Servers, row)
	}
	return snapshot, nil
}

func (s *AmneziaService) SyncLocalConfigFiles() error {
	entries, err := os.ReadDir(amneziaConfigDir)
	if err != nil {
		return nil
	}
	db := database.GetDB()
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".conf" {
			continue
		}
		interfaceName := strings.TrimSuffix(entry.Name(), ".conf")
		var count int64
		if err := db.Model(&model.AmneziaServer{}).Where("interface_name = ?", interfaceName).Count(&count).Error; err != nil {
			return err
		}
		if count > 0 {
			continue
		}
		server, err := s.serverFromConfigFile(filepath.Join(amneziaConfigDir, entry.Name()), interfaceName)
		if err != nil {
			continue
		}
		if _, err := s.CreateServer(server); err != nil {
			return err
		}
	}
	return nil
}

func (s *AmneziaService) serverFromConfigFile(path, interfaceName string) (*model.AmneziaServer, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	values := parseIniAssignments(string(content))
	listenPort, _ := strconv.Atoi(values["ListenPort"])
	mtu, _ := strconv.Atoi(values["MTU"])
	publicKey := strings.TrimSpace(values["PublicKey"])
	privateKey := strings.TrimSpace(values["PrivateKey"])
	if publicKey == "" {
		publicKey = s.publicKeyFromPrivate(privateKey)
	}
	obf := map[string]any{}
	for _, key := range []string{"Jc", "Jmin", "Jmax", "S1", "S2", "S3", "S4", "H1", "H2", "H3", "H4", "I1", "I2", "I3", "I4", "I5"} {
		if value, ok := values[key]; ok && strings.TrimSpace(value) != "" {
			if n, err := strconv.Atoi(value); err == nil {
				obf[key] = n
			} else {
				obf[key] = value
			}
		}
	}
	obfJSON, _ := json.Marshal(obf)
	server := &model.AmneziaServer{
		Name:            interfaceName,
		InterfaceName:   interfaceName,
		ListenPort:      listenPort,
		PrivateKey:      privateKey,
		PublicKey:       publicKey,
		Address:         strings.TrimSpace(values["Address"]),
		DNS:             strings.TrimSpace(values["DNS"]),
		MTU:             mtu,
		ProtocolMode:    DefaultAmneziaProtocolMode,
		ObfuscationJSON: string(obfJSON),
		ServerType:      "local",
		AutoEndpoint:    false,
		Enabled:         s.IsServerRunning(interfaceName),
	}
	return server, nil
}

func parseIniAssignments(content string) map[string]string {
	values := map[string]string{}
	for _, line := range strings.Split(content, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, "[") {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}
		values[strings.TrimSpace(parts[0])] = strings.TrimSpace(parts[1])
	}
	return values
}

func (s *AmneziaService) publicKeyFromPrivate(privateKey string) string {
	if privateKey == "" {
		return ""
	}
	cmd, err := s.runtimeCommand("awg", "pubkey")
	if err != nil {
		return ""
	}
	cmd.Stdin = strings.NewReader(privateKey)
	output, err := cmd.Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(output))
}

func (s *AmneziaService) CollectRuntimeStats() (*AmneziaRuntimeSnapshot, error) {
	runtimePeers, err := s.readAwgRuntime()
	if err != nil {
		return s.GetRuntimeSnapshot()
	}
	if len(runtimePeers) > 0 {
		if err := s.persistRuntimePeers(runtimePeers); err != nil {
			return nil, err
		}
	}
	if err := s.enforceRuntimeLimits(); err != nil {
		return nil, err
	}
	return s.GetRuntimeSnapshot()
}

func (s *AmneziaService) IsServerRunning(interfaceName string) bool {
	interfaceName = strings.TrimSpace(interfaceName)
	if interfaceName == "" {
		return false
	}
	if s.useDockerRuntime() {
		// Check if container is running first
		container := s.dockerContainerName()
		if out, _ := exec.Command("docker", "inspect", "--format={{.State.Status}}", container).Output(); strings.TrimSpace(string(out)) != "running" {
			return false
		}
		cmd, err := s.runtimeCommand("awg", "show", "interfaces")
		if err == nil {
			output, err := cmd.Output()
			if err == nil {
				for _, name := range strings.Fields(string(output)) {
					if name == interfaceName {
						return true
					}
				}
			}
		}
		return false
	}
	if _, err := exec.LookPath("systemctl"); err == nil {
		unit := fmt.Sprintf("amneziawg@%s.service", interfaceName)
		if err := exec.Command("systemctl", "is-active", "--quiet", unit).Run(); err == nil {
			return true
		}
	}
	if _, err := exec.LookPath("ip"); err == nil {
		if err := exec.Command("ip", "link", "show", "dev", interfaceName).Run(); err == nil {
			return true
		}
	}
	return false
}

func (s *AmneziaService) readAwgRuntime() (map[string]awgPeerRuntime, error) {
	cmd, err := s.runtimeCommand("awg", "show", "all", "dump")
	if err != nil {
		return nil, err
	}
	output, err := cmd.Output()
	if err != nil {
		return nil, err
	}
	return parseAwgDump(string(output)), nil
}

func parseAwgDump(output string) map[string]awgPeerRuntime {
	peers := map[string]awgPeerRuntime{}
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 9 {
			continue
		}
		handshake, _ := strconv.ParseInt(fields[5], 10, 64)
		rx, _ := strconv.ParseInt(fields[6], 10, 64)
		tx, _ := strconv.ParseInt(fields[7], 10, 64)
		peers[fields[1]] = awgPeerRuntime{
			Interface:     fields[0],
			PublicKey:     fields[1],
			Handshake:     handshake,
			ReceiveBytes:  rx,
			TransmitBytes: tx,
		}
	}
	return peers
}

func (s *AmneziaService) persistRuntimePeers(runtimePeers map[string]awgPeerRuntime) error {
	db := database.GetDB()
	var peers []model.AmneziaPeer
	if err := db.Find(&peers).Error; err != nil {
		return err
	}
	for _, peer := range peers {
		rt, ok := runtimePeers[peer.PublicKey]
		if !ok {
			continue
		}
		stat, err := s.GetPeerStats(peer.Id)
		if err != nil {
			return err
		}
		stat.RxBytes = rt.ReceiveBytes
		stat.TxBytes = rt.TransmitBytes
		stat.LastHandshake = rt.Handshake
		if err := db.Save(stat).Error; err != nil {
			return err
		}
	}
	return nil
}

func (s *AmneziaService) enforceRuntimeLimits() error {
	db := database.GetDB()
	var peers []model.AmneziaPeer
	now := time.Now().Unix()
	if err := db.Where("enabled = ?", true).Find(&peers).Error; err != nil {
		return err
	}
	rebuild := map[int]bool{}
	for _, peer := range peers {
		reason := ""
		if peer.ExpiresAt != nil && *peer.ExpiresAt <= now {
			reason = "expired"
		}
		if reason == "" && peer.TrafficLimit > 0 {
			stat, err := s.GetPeerStats(peer.Id)
			if err != nil {
				return err
			}
			if stat.RxBytes+stat.TxBytes >= peer.TrafficLimit {
				reason = "traffic_limit"
			}
		}
		if reason == "" {
			continue
		}
		peer.Enabled = false
		peer.PausedReason = &reason
		peer.PausedAt = &now
		if err := db.Save(&peer).Error; err != nil {
			return err
		}
		rebuild[peer.ServerID] = true
	}
	for serverID := range rebuild {
		if err := s.RebuildServerConfig(serverID); err != nil {
			return err
		}
	}
	return nil
}

func (s *AmneziaService) StartServer(id int) error {
	server, err := s.GetServer(id)
	if err != nil {
		return err
	}
	if _, err := s.writeServerConfig(id); err != nil {
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
	if _, err := s.writeServerConfig(id); err != nil {
		return err
	}
	if err := s.systemctlAction("restart", server.InterfaceName); err != nil {
		return err
	}
	server.Enabled = true
	return database.GetDB().Save(server).Error
}

func (s *AmneziaService) systemctlAction(action, interfaceName string) error {
	if s.useDockerRuntime() {
		return s.dockerAction(action, interfaceName)
	}
	unit := fmt.Sprintf("amneziawg@%s.service", interfaceName)
	if _, err := exec.LookPath("systemctl"); err != nil {
		return fmt.Errorf("systemctl not found")
	}
	if err := s.ensureSystemdTemplate(); err != nil {
		return err
	}
	cmd := exec.Command("systemctl", action, unit)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%w: %s", err, strings.TrimSpace(string(output)))
	}
	return nil
}

func (s *AmneziaService) useDockerRuntime() bool {
	// Explicit override via environment variable
	mode := strings.ToLower(strings.TrimSpace(os.Getenv(amneziaRuntimeEnv)))
	if mode == amneziaRuntimeDocker {
		return true
	}
	if mode == "native" || mode == "systemd" {
		return false
	}
	// Auto-detect: if docker is available and the amneziawg container exists, use docker mode
	if _, err := exec.LookPath("docker"); err == nil {
		container := s.dockerContainerName()
		cmd := exec.Command("docker", "ps", "-a", "--filter", "name=^"+container+"$", "--format", "{{.Names}}")
		output, _ := cmd.Output()
		if strings.TrimSpace(string(output)) == container {
			return true
		}
	}
	return false
}

func (s *AmneziaService) helperStripCommand(args []string) (*exec.Cmd, error) {
	if len(args) != 1 {
		return nil, fmt.Errorf("strip command requires exactly one argument")
	}
	// Get config from helper
	getCmd := exec.Command(amneziaHelperPath, "get-config")
	output, err := getCmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to get config: %w", err)
	}
	// Strip the config (remove PostUp/PostDown)
	stripped := s.stripConfig(string(output))
	// Return command that outputs stripped config
	cmd := exec.Command("echo", stripped)
	return cmd, nil
}

func (s *AmneziaService) stripConfig(config string) string {
	lines := strings.Split(config, "\n")
	var result []string
	inInterface := false
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "[Interface]" {
			inInterface = true
		} else if strings.HasPrefix(trimmed, "[") {
			inInterface = false
		}
		// Skip PostUp/PostDown in Interface section
		if inInterface && (strings.HasPrefix(trimmed, "PostUp") || strings.HasPrefix(trimmed, "PostDown")) {
			continue
		}
		result = append(result, line)
	}
	return strings.Join(result, "\n")
}

func (s *AmneziaService) runtimeCommand(name string, args ...string) (*exec.Cmd, error) {
	if s.useDockerRuntime() {
		// Use helper script for Docker operations
		if _, err := exec.LookPath(amneziaHelperPath); err != nil {
			return nil, fmt.Errorf("awg-helper not found: %w", err)
		}
		// Map awg commands to helper commands
		var helperArgs []string
		switch name {
		case "awg":
			if len(args) >= 2 && args[0] == "show" && args[1] == "interfaces" {
				helperArgs = []string{"check-container"}
			} else if len(args) >= 3 && args[0] == "show" && args[1] == "all" && args[2] == "dump" {
				helperArgs = []string{"show-dump"}
			} else if len(args) >= 1 && args[0] == "genkey" {
				helperArgs = []string{"gen-key"}
			} else if len(args) >= 1 && args[0] == "genpsk" {
				helperArgs = []string{"gen-psk"}
			} else {
				return nil, fmt.Errorf("unsupported awg command: %v", args)
			}
		case "awg-quick":
			if len(args) >= 1 && args[0] == "strip" {
				// For strip, we need to get config and process it
				return s.helperStripCommand(args[1:])
			} else {
				return nil, fmt.Errorf("unsupported awg-quick command: %v", args)
			}
		default:
			return nil, fmt.Errorf("unsupported command: %s", name)
		}
		return exec.Command(amneziaHelperPath, helperArgs...), nil
	}
	if _, err := exec.LookPath(name); err != nil {
		return nil, err
	}
	return exec.Command(name, args...), nil
}

func (s *AmneziaService) dockerContainerName() string {
	name := strings.TrimSpace(os.Getenv(amneziaDockerContainerEnv))
	if name == "" {
		return amneziaDefaultContainer
	}
	return name
}

func (s *AmneziaService) dockerAction(action, interfaceName string) error {
	if _, err := exec.LookPath("docker"); err != nil {
		return fmt.Errorf("docker CLI not found: %w", err)
	}
	container := s.dockerContainerName()
	configPath := filepath.Join(amneziaConfigDir, fmt.Sprintf("%s.conf", interfaceName))
	switch action {
	case "enable", "disable":
		return nil
	case "start":
		_ = exec.Command("docker", "start", container).Run()
		cmd := exec.Command("docker", "exec", container, "awg-quick", "up", configPath)
		output, err := cmd.CombinedOutput()
		if err != nil && !s.IsServerRunning(interfaceName) {
			return fmt.Errorf("%w: %s", err, strings.TrimSpace(string(output)))
		}
		return nil
	case "stop":
		cmd := exec.Command("docker", "exec", container, "awg-quick", "down", configPath)
		output, err := cmd.CombinedOutput()
		if err != nil && s.IsServerRunning(interfaceName) {
			return fmt.Errorf("%w: %s", err, strings.TrimSpace(string(output)))
		}
		return nil
	case "restart":
		_ = s.dockerAction("stop", interfaceName)
		return s.dockerAction("start", interfaceName)
	case "syncconf":
		// Use helper for hot-reload config without restarting container
		cmd := exec.Command(amneziaHelperPath, "sync-config", interfaceName)
		output, err := cmd.CombinedOutput()
		if err != nil {
			return fmt.Errorf("awg syncconf failed: %w: %s", err, strings.TrimSpace(string(output)))
		}
		return nil
	default:
		return fmt.Errorf("unsupported docker AmneziaWG action: %s", action)
	}
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
	server, err := s.writeServerConfig(serverId)
	if err != nil {
		return err
	}
	configPath := filepath.Join(amneziaConfigDir, fmt.Sprintf("%s.conf", server.InterfaceName))
	if err := s.applyServerConfig(server, configPath); err != nil {
		fmt.Printf("Warning: failed to apply AmneziaWG config for server %d: %v\n", serverId, err)
	}

	return nil
}

func (s *AmneziaService) applyServerConfig(server *model.AmneziaServer, configPath string) error {
	if !s.IsServerRunning(server.InterfaceName) {
		if server.Enabled {
			return s.systemctlAction("start", server.InterfaceName)
		}
		return nil
	}
	if err := s.syncServerConfig(server.InterfaceName, configPath); err != nil {
		if s.useDockerRuntime() {
			// In Docker mode, try awg-quick down/up inside container instead of full restart
			_ = s.dockerAction("stop", server.InterfaceName)
			if startErr := s.dockerAction("start", server.InterfaceName); startErr != nil {
				return fmt.Errorf("sync failed: %v; docker start failed: %w", err, startErr)
			}
			return nil
		}
		if restartErr := s.systemctlAction("restart", server.InterfaceName); restartErr != nil {
			return fmt.Errorf("sync failed: %v; restart failed: %w", err, restartErr)
		}
	}
	return nil
}

func (s *AmneziaService) syncServerConfig(interfaceName, configPath string) error {
	if s.useDockerRuntime() {
		return s.dockerAction("syncconf", interfaceName)
	}
	stripCmd, err := s.runtimeCommand("awg-quick", "strip", configPath)
	if err != nil {
		return err
	}
	stripped, err := stripCmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("awg-quick strip failed: %w: %s", err, strings.TrimSpace(string(stripped)))
	}
	tmp, err := os.CreateTemp(amneziaConfigDir, fmt.Sprintf(".x-ui-awg-sync-%s-*.conf", interfaceName))
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)
	if _, err := tmp.Write(stripped); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	syncCmd, err := s.runtimeCommand("awg", "syncconf", interfaceName, tmpName)
	if err != nil {
		return err
	}
	output, err := syncCmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("awg syncconf failed: %w: %s", err, strings.TrimSpace(string(output)))
	}
	return nil
}

func (s *AmneziaService) writeServerConfig(serverId int) (*model.AmneziaServer, error) {
	server, err := s.GetServer(serverId)
	if err != nil {
		return nil, fmt.Errorf("failed to get server: %w", err)
	}

	peers, err := s.GetActivePeersForConfig(serverId)
	if err != nil {
		return nil, fmt.Errorf("failed to get active peers: %w", err)
	}
	if err := s.ensurePeerAddresses(server, peers); err != nil {
		return nil, fmt.Errorf("failed to assign peer addresses: %w", err)
	}
	peers, err = s.GetActivePeersForConfig(serverId)
	if err != nil {
		return nil, fmt.Errorf("failed to reload active peers: %w", err)
	}

	// Parse obfuscation parameters from JSON
	obf := s.parseObfuscationStruct(server.ObfuscationJSON)

	// Validate AWG 2.0 parameters before generating config
	if err := s.ValidateAwg20Params(obf); err != nil {
		// Generate valid parameters if validation fails
		obf, _ = s.GenerateValidatedObfuscationParams()
	}

	// Use template renderer instead of string concatenation
	config, err := RenderServerConfig(server, peers, obf, s.useDockerRuntime())
	if err != nil {
		return nil, fmt.Errorf("failed to render server config: %w", err)
	}

	// Use /etc/amnezia/amneziawg/ directory for AmneziaWG configs
	configPath := filepath.Join(amneziaConfigDir, fmt.Sprintf("%s.conf", server.InterfaceName))

	if err := s.writeConfigFile(configPath, config); err != nil {
		return nil, fmt.Errorf("failed to write config: %w", err)
	}

	// For Docker runtime, also save config to container
	if s.useDockerRuntime() {
		cmd := exec.Command(amneziaHelperPath, "save-config", server.InterfaceName)
		cmd.Stdin = strings.NewReader(config)
		if output, err := cmd.CombinedOutput(); err != nil {
			return nil, fmt.Errorf("failed to save config to container: %w: %s", err, strings.TrimSpace(string(output)))
		}
	}

	return server, nil
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

	headers := generateHeaderRanges()
	return &model.AmneziaObfuscation{
		Jc:   randomInt(4, 12),     // Junk packet count: 4-12
		Jmin: randomInt(64, 128),   // Min junk size: 64-128
		Jmax: randomInt(768, 1024), // Max junk size: 768-1024
		S1:   randomInt(0, 64),     // Init padding: 0-64
		S2:   randomInt(0, 64),     // Response padding: 0-64
		S3:   randomInt(0, 64),     // Cookie padding: 0-64
		S4:   randomInt(0, 32),     // Transport padding: 0-32
		H1:   headers[0],           // Init header range
		H2:   headers[1],           // Response header range
		H3:   headers[2],           // Cookie header range
		H4:   headers[3],           // Transport header range
		I1:   "",                   // Custom signature packets (optional)
		I2:   "",
		I3:   "",
		I4:   "",
		I5:   "",
	}
}

// generateHeaderRanges creates random header ranges in separated bands so they cannot overlap.
func generateHeaderRanges() [4]string {
	bases := [4]int{100000, 300000, 500000, 700000}
	var ranges [4]string
	for i, base := range bases {
		start := base + randomInt(0, 20000)
		end := start + randomInt(20000, 60000)
		ranges[i] = fmt.Sprintf("%d-%d", start, end)
	}
	return ranges
}

// randomInt generates a random integer between min and max (inclusive)
func randomInt(min, max int) int {
	if min >= max {
		return min
	}
	v, err := rand.Int(rand.Reader, big.NewInt(int64(max-min+1)))
	if err != nil {
		return min
	}
	return min + int(v.Int64())
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
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
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
		Jmin: 64,
		Jmax: 200,
		S1:   32,
		S2:   8,
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

	serverIP := strings.TrimSpace(parts[0])
	cidr := strings.TrimSpace(parts[1])
	prefixLen, err := strconv.Atoi(cidr)
	if err != nil {
		return "", fmt.Errorf("invalid CIDR prefix: %s", cidr)
	}
	prefix, err := netip.ParsePrefix(fmt.Sprintf("%s/%d", serverIP, prefixLen))
	if err != nil || !prefix.Addr().Is4() {
		return "", fmt.Errorf("invalid IPv4 subnet: %s", serverAddr)
	}

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

	addr := prefix.Masked().Addr()
	for {
		addr = addr.Next()
		if !prefix.Contains(addr) {
			break
		}
		candidate := addr.String()
		if candidate == serverIP || isIPv4Broadcast(prefix, addr) {
			continue
		}
		if !usedIPs[candidate] {
			return candidate + "/32", nil
		}
	}

	return "", fmt.Errorf("no available IP addresses in subnet %s", serverAddr)
}

func (s *AmneziaService) ensurePeerAddresses(server *model.AmneziaServer, peers []model.AmneziaPeer) error {
	db := database.GetDB()
	for i := range peers {
		if strings.TrimSpace(peers[i].Address) != "" {
			continue
		}
		address, err := s.GetNextPeerIP(server)
		if err != nil {
			return err
		}
		if err := db.Model(&model.AmneziaPeer{}).
			Where("id = ?", peers[i].Id).
			Update("address", address).Error; err != nil {
			return err
		}
		peers[i].Address = address
	}
	return nil
}

func isIPv4Broadcast(prefix netip.Prefix, addr netip.Addr) bool {
	if prefix.Bits() >= 31 {
		return false
	}
	next := addr.Next()
	return !prefix.Contains(next)
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
