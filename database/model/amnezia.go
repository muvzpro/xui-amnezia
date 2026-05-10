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
	ServerType      string        `json:"serverType" form:"serverType" gorm:"default:'local'"` // local or remote
	AutoEndpoint    bool          `json:"autoEndpoint" form:"autoEndpoint" gorm:"default:true"`
	Enabled         bool          `json:"enabled" form:"enabled" gorm:"index"`
	CreatedAt       int64         `json:"createdAt" gorm:"autoCreateTime"`
	UpdatedAt       int64         `json:"updatedAt" gorm:"autoUpdateTime"`
	Peers           []AmneziaPeer `json:"peers" gorm:"foreignKey:ServerID"`
}

// AmneziaWG 2.0 Obfuscation Parameters
// Based on: https://docs.amnezia.org/documentation/amnezia-wg/
// These parameters are used for traffic obfuscation to bypass DPI detection
type AmneziaObfuscation struct {
	// Jc - Junk packet count (4-12 recommended)
	// Number of junk packets sent before handshake
	Jc int `json:"jc" form:"jc"`
	// Jmin - Minimum junk packet size (64-128 bytes)
	Jmin int `json:"jmin" form:"jmin"`
	// Jmax - Maximum junk packet size (768-1024 bytes)
	Jmax int `json:"jmax" form:"jmax"`
	// S1 - Initiation packet padding (0-64 bytes)
	S1 int `json:"s1" form:"s1"`
	// S2 - Response packet padding (0-64 bytes)
	S2 int `json:"s2" form:"s2"`
	// S3 - Cookie packet padding (0-64 bytes)
	S3 int `json:"s3" form:"s3"`
	// S4 - Transport data packet padding (0-32 bytes)
	S4 int `json:"s4" form:"s4"`
	// H1 - Initiation message header range (uint32, format: "min-max" or single value)
	H1 string `json:"h1" form:"h1"`
	// H2 - Response message header range (uint32)
	H2 string `json:"h2" form:"h2"`
	// H3 - Cookie message header range (uint32)
	H3 string `json:"h3" form:"h3"`
	// H4 - Transport message header range (uint32)
	H4 string `json:"h4" form:"h4"`
	// I1 - Custom signature packet 1 (CPS format: <b hex><r size><t> etc.)
	I1 string `json:"i1" form:"i1"`
	// I2 - Custom signature packet 2 (CPS format)
	I2 string `json:"i2" form:"i2"`
	// I3 - Custom signature packet 3 (CPS format)
	I3 string `json:"i3" form:"i3"`
	// I4 - Custom signature packet 4 (CPS format)
	I4 string `json:"i4" form:"i4"`
	// I5 - Custom signature packet 5 (CPS format)
	I5 string `json:"i5" form:"i5"`
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
