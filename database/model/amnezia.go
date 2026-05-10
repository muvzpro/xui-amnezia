package model

// AmneziaServer represents a configured AmneziaWG server instance.
type AmneziaServer struct {
	Id              int           `json:"id" gorm:"primaryKey;autoIncrement"`
	Name            string        `json:"name" form:"name" gorm:"not null"`
	InterfaceName   string        `json:"interfaceName" form:"interfaceName" gorm:"not null;uniqueIndex"`
	ListenPort      int           `json:"listenPort" form:"listenPort"`
	PrivateKey      string        `json:"privateKey" form:"privateKey" gorm:"type:text"`
	PublicKey       string        `json:"publicKey" form:"publicKey" gorm:"type:text"`
	Address         string        `json:"address" form:"address"`
	DNS             string        `json:"dns" form:"dns"`
	Endpoint        string        `json:"endpoint" form:"endpoint"`
	MTU             int           `json:"mtu" form:"mtu"`
	ProtocolMode    string        `json:"protocolMode" form:"protocolMode"`
	ObfuscationJSON string        `json:"obfuscationJson" form:"obfuscationJson" gorm:"type:text"`
	Enabled         bool          `json:"enabled" form:"enabled" gorm:"index"`
	CreatedAt       int64         `json:"createdAt" gorm:"autoCreateTime"`
	UpdatedAt       int64         `json:"updatedAt" gorm:"autoUpdateTime"`
	Peers           []AmneziaPeer `json:"peers" gorm:"foreignKey:ServerID"`
}

// AmneziaWG 2.0 Obfuscation Parameters
// These parameters are used for traffic obfuscation to bypass DPI detection
type AmneziaObfuscation struct {
	// Jc - Junk packet count: number of junk packets to send before handshake
	Jc int `json:"jc" form:"jc"`
	// Jmin - Minimum junk packet size
	Jmin int `json:"jmin" form:"jmin"`
	// Jmax - Maximum junk packet size
	Jmax int `json:"jmax" form:"jmax"`
	// S1 - Initiation packet junk size (first packet)
	S1 int `json:"s1" form:"s1"`
	// S2 - Initiation packet junk size (second packet)
	S2 int `json:"s2" form:"s2"`
	// H1 - Response packet junk size (first packet)
	H1 int `json:"h1" form:"h1"`
	// H2 - Response packet junk size (second packet)
	H2 int `json:"h2" form:"h2"`
	// I1 - Interval parameter 1 (for timing obfuscation)
	I1 int `json:"i1" form:"i1"`
	// I2 - Interval parameter 2 (for timing obfuscation)
	I2 int `json:"i2" form:"i2"`
}

// AmneziaPeer represents a peer client attached to an AmneziaWG server.
type AmneziaPeer struct {
	Id                  int     `json:"id" gorm:"primaryKey;autoIncrement"`
	ServerID            int     `json:"serverId" form:"serverId" gorm:"index;not null"`
	Name                string  `json:"name" form:"name" gorm:"not null"`
	PrivateKey          string  `json:"privateKey" form:"privateKey" gorm:"type:text"`
	PublicKey           string  `json:"publicKey" form:"publicKey" gorm:"type:text"`
	PresharedKey        string  `json:"presharedKey" form:"presharedKey" gorm:"type:text"`
	Address             string  `json:"address" form:"address"`
	AllowedIPs          string  `json:"allowedIps" form:"allowedIps" gorm:"type:text"`
	Endpoint            string  `json:"endpoint" form:"endpoint"`
	PersistentKeepalive int     `json:"persistentKeepalive" form:"persistentKeepalive"`
	Enabled             bool    `json:"enabled" form:"enabled" gorm:"index"`
	ExpiryDays          *int    `json:"expiryDays" form:"expiryDays"`
	ExpiresAt           *int64  `json:"expiresAt" form:"expiresAt"`
	PausedReason        *string `json:"pausedReason" form:"pausedReason" gorm:"type:text"`
	PausedAt            *int64  `json:"pausedAt" form:"pausedAt"`
	TrafficLimit        int64   `json:"trafficLimit" form:"trafficLimit"`
	ExpiryTime          int64   `json:"expiryTime" form:"expiryTime"`
	CreatedAt           int64   `json:"createdAt" gorm:"autoCreateTime"`
	UpdatedAt           int64   `json:"updatedAt" gorm:"autoUpdateTime"`
}

// AmneziaTrafficStat stores traffic statistics for an AmneziaWG peer.
type AmneziaTrafficStat struct {
	Id            int   `json:"id" gorm:"primaryKey;autoIncrement"`
	PeerID        int   `json:"peerId" form:"peerId" gorm:"index;not null"`
	RxBytes       int64 `json:"rxBytes" form:"rxBytes"`
	TxBytes       int64 `json:"txBytes" form:"txBytes"`
	LastHandshake int64 `json:"lastHandshake" form:"lastHandshake"`
	CreatedAt     int64 `json:"createdAt" gorm:"autoCreateTime"`
	UpdatedAt     int64 `json:"updatedAt" gorm:"autoUpdateTime"`
}

// AmneziaSetting stores global AmneziaWG panel settings.
type AmneziaSetting struct {
	Id        int    `json:"id" gorm:"primaryKey;autoIncrement"`
	Key       string `json:"key" form:"key" gorm:"uniqueIndex;not null"`
	Value     string `json:"value" form:"value" gorm:"type:text"`
	CreatedAt int64  `json:"createdAt" gorm:"autoCreateTime"`
	UpdatedAt int64  `json:"updatedAt" gorm:"autoUpdateTime"`
}
