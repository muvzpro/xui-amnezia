package controller

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/mhsanaei/3x-ui/v3/database/model"
	"github.com/mhsanaei/3x-ui/v3/web/service"
)

type AmneziaController struct {
    BaseController
    amneziaService *service.AmneziaService
}

func NewAmneziaController(g *gin.RouterGroup) *AmneziaController {
    a := &AmneziaController{
        amneziaService: service.NewAmneziaService(),
    }
    a.initRouter(g)
    return a
}

func (a *AmneziaController) initRouter(g *gin.RouterGroup) {
    g.GET("/servers", a.getServers)
    g.POST("/servers", a.createServer)
    g.GET("/servers/:id", a.getServer)
    g.PUT("/servers/:id", a.updateServer)
    g.DELETE("/servers/:id", a.deleteServer)

    g.GET("/servers/:id/peers", a.getPeers)
    g.GET("/servers/:id/active-peers", a.getActivePeers)
    g.POST("/servers/:id/peers", a.createPeer)
    g.PUT("/peers/:id", a.updatePeer)
    g.DELETE("/peers/:id", a.deletePeer)
    g.POST("/peers/:id/extend", a.extendPeer)

    g.POST("/servers/:id/start", a.startServer)
    g.POST("/servers/:id/stop", a.stopServer)
    g.POST("/servers/:id/restart", a.restartServer)

    g.GET("/peers/:id/config", a.getPeerConfig)
    g.GET("/peers/:id/qrcode", a.getPeerQRCode)
    g.GET("/peers/:id/stats", a.getPeerStats)
    g.GET("/peers/:id/vpnuri", a.getPeerVPNURI)
    g.GET("/peers/:id/vpnuri-qrcode", a.getPeerVPNURICode)
}

func (a *AmneziaController) getServers(c *gin.Context) {
    servers, err := a.amneziaService.GetAllServers()
    if err != nil {
        jsonMsg(c, "Failed to load AmneziaWG servers", err)
        return
    }
    jsonObj(c, servers, nil)
}

func (a *AmneziaController) getServer(c *gin.Context) {
    id, err := strconv.Atoi(c.Param("id"))
    if err != nil {
        jsonMsg(c, "Invalid server ID", err)
        return
    }
    server, err := a.amneziaService.GetServer(id)
    if err != nil {
        jsonMsg(c, "Failed to load AmneziaWG server", err)
        return
    }
    jsonObj(c, server, nil)
}

func (a *AmneziaController) createServer(c *gin.Context) {
    server := &model.AmneziaServer{}
    if err := c.ShouldBind(server); err != nil {
        jsonMsg(c, "Failed to parse server payload", err)
        return
    }
    created, err := a.amneziaService.CreateServer(server)
    if err != nil {
        jsonMsg(c, "Failed to create AmneziaWG server", err)
        return
    }
    jsonObj(c, created, nil)
}

func (a *AmneziaController) updateServer(c *gin.Context) {
    id, err := strconv.Atoi(c.Param("id"))
    if err != nil {
        jsonMsg(c, "Invalid server ID", err)
        return
    }
    server := &model.AmneziaServer{Id: id}
    if err := c.ShouldBind(server); err != nil {
        jsonMsg(c, "Failed to parse server payload", err)
        return
    }
    updated, err := a.amneziaService.UpdateServer(server)
    if err != nil {
        jsonMsg(c, "Failed to update AmneziaWG server", err)
        return
    }
    jsonObj(c, updated, nil)
}

func (a *AmneziaController) deleteServer(c *gin.Context) {
    id, err := strconv.Atoi(c.Param("id"))
    if err != nil {
        jsonMsg(c, "Invalid server ID", err)
        return
    }
    if err := a.amneziaService.DeleteServer(id); err != nil {
        jsonMsg(c, "Failed to delete AmneziaWG server", err)
        return
    }
    jsonObj(c, gin.H{"deleted": id}, nil)
}

func (a *AmneziaController) getPeers(c *gin.Context) {
    serverId, err := strconv.Atoi(c.Param("id"))
    if err != nil {
        jsonMsg(c, "Invalid server ID", err)
        return
    }
    peers, err := a.amneziaService.GetPeers(serverId)
    if err != nil {
        jsonMsg(c, "Failed to load AmneziaWG peers", err)
        return
    }
    jsonObj(c, peers, nil)
}

func (a *AmneziaController) createPeer(c *gin.Context) {
    serverId, err := strconv.Atoi(c.Param("id"))
    if err != nil {
        jsonMsg(c, "Invalid server ID", err)
        return
    }
    peer := &model.AmneziaPeer{ServerID: serverId}
    if err := c.ShouldBind(peer); err != nil {
        jsonMsg(c, "Failed to parse peer payload", err)
        return
    }
    created, err := a.amneziaService.CreatePeer(peer)
    if err != nil {
        jsonMsg(c, "Failed to create AmneziaWG peer", err)
        return
    }
    jsonObj(c, created, nil)
}

func (a *AmneziaController) updatePeer(c *gin.Context) {
    id, err := strconv.Atoi(c.Param("id"))
    if err != nil {
        jsonMsg(c, "Invalid peer ID", err)
        return
    }
    peer := &model.AmneziaPeer{Id: id}
    if err := c.ShouldBind(peer); err != nil {
        jsonMsg(c, "Failed to parse peer payload", err)
        return
    }
    updated, err := a.amneziaService.UpdatePeer(peer)
    if err != nil {
        jsonMsg(c, "Failed to update AmneziaWG peer", err)
        return
    }
    jsonObj(c, updated, nil)
}

func (a *AmneziaController) deletePeer(c *gin.Context) {
    id, err := strconv.Atoi(c.Param("id"))
    if err != nil {
        jsonMsg(c, "Invalid peer ID", err)
        return
    }
    if err := a.amneziaService.DeletePeer(id); err != nil {
        jsonMsg(c, "Failed to delete AmneziaWG peer", err)
        return
    }
    jsonObj(c, gin.H{"deleted": id}, nil)
}

func (a *AmneziaController) startServer(c *gin.Context) {
    id, err := strconv.Atoi(c.Param("id"))
    if err != nil {
        jsonMsg(c, "Invalid server ID", err)
        return
    }
    if err := a.amneziaService.StartServer(id); err != nil {
        jsonMsg(c, "Failed to start AmneziaWG server", err)
        return
    }
    jsonObj(c, gin.H{"started": id}, nil)
}

func (a *AmneziaController) stopServer(c *gin.Context) {
    id, err := strconv.Atoi(c.Param("id"))
    if err != nil {
        jsonMsg(c, "Invalid server ID", err)
        return
    }
    if err := a.amneziaService.StopServer(id); err != nil {
        jsonMsg(c, "Failed to stop AmneziaWG server", err)
        return
    }
    jsonObj(c, gin.H{"stopped": id}, nil)
}

func (a *AmneziaController) restartServer(c *gin.Context) {
    id, err := strconv.Atoi(c.Param("id"))
    if err != nil {
        jsonMsg(c, "Invalid server ID", err)
        return
    }
    if err := a.amneziaService.RestartServer(id); err != nil {
        jsonMsg(c, "Failed to restart AmneziaWG server", err)
        return
    }
    jsonObj(c, gin.H{"restarted": id}, nil)
}

func (a *AmneziaController) getPeerConfig(c *gin.Context) {
    id, err := strconv.Atoi(c.Param("id"))
    if err != nil {
        jsonMsg(c, "Invalid peer ID", err)
        return
    }
    config, err := a.amneziaService.GetPeerConfig(id)
    if err != nil {
        jsonMsg(c, "Failed to generate peer config", err)
        return
    }
    c.Data(http.StatusOK, "text/plain; charset=utf-8", []byte(config))
}

func (a *AmneziaController) getPeerQRCode(c *gin.Context) {
    id, err := strconv.Atoi(c.Param("id"))
    if err != nil {
        jsonMsg(c, "Invalid peer ID", err)
        return
    }
    qr, err := a.amneziaService.GetPeerQRCode(id)
    if err != nil {
        jsonMsg(c, "Failed to generate peer QR code", err)
        return
    }
    jsonObj(c, gin.H{"qr": qr}, nil)
}

func (a *AmneziaController) getPeerStats(c *gin.Context) {
    id, err := strconv.Atoi(c.Param("id"))
    if err != nil {
        jsonMsg(c, "Invalid peer ID", err)
        return
    }
    stat, err := a.amneziaService.GetPeerStats(id)
    if err != nil {
        jsonMsg(c, "Failed to load peer stats", err)
        return
    }
    jsonObj(c, stat, nil)
}

func (a *AmneziaController) getActivePeers(c *gin.Context) {
    serverId, err := strconv.Atoi(c.Param("id"))
    if err != nil {
        jsonMsg(c, "Invalid server ID", err)
        return
    }
    peers, err := a.amneziaService.GetActivePeersForConfig(serverId)
    if err != nil {
        jsonMsg(c, "Failed to load active AmneziaWG peers", err)
        return
    }
    jsonObj(c, peers, nil)
}

func (a *AmneziaController) extendPeer(c *gin.Context) {
    id, err := strconv.Atoi(c.Param("id"))
    if err != nil {
        jsonMsg(c, "Invalid peer ID", err)
        return
    }
    var req struct {
        Days int `json:"days"`
    }
    if err := c.ShouldBind(&req); err != nil {
        jsonMsg(c, "Failed to parse extend payload", err)
        return
    }
    if req.Days <= 0 {
        jsonMsg(c, "Days must be greater than zero", nil)
        return
    }
    if err := a.amneziaService.ExtendPeer(id, req.Days); err != nil {
        jsonMsg(c, "Failed to extend AmneziaWG peer", err)
        return
    }
    jsonObj(c, gin.H{"extended": id, "days": req.Days}, nil)
}

// getPeerVPNURI returns the vpn:// URI for Amnezia VPN app import
// This is a killer-feature for Amnezia - allows one-click import
func (a *AmneziaController) getPeerVPNURI(c *gin.Context) {
    id, err := strconv.Atoi(c.Param("id"))
    if err != nil {
        jsonMsg(c, "Invalid peer ID", err)
        return
    }
    vpnURI, err := a.amneziaService.GetPeerVPNURI(id)
    if err != nil {
        jsonMsg(c, "Failed to generate vpn:// URI", err)
        return
    }
    jsonObj(c, gin.H{"vpnUri": vpnURI}, nil)
}

// getPeerVPNURICode returns a QR code for the vpn:// URI
// Use this for mobile app import via camera scan
func (a *AmneziaController) getPeerVPNURICode(c *gin.Context) {
    id, err := strconv.Atoi(c.Param("id"))
    if err != nil {
        jsonMsg(c, "Invalid peer ID", err)
        return
    }
    qr, err := a.amneziaService.GetPeerVPNURICode(id)
    if err != nil {
        jsonMsg(c, "Failed to generate vpn:// URI QR code", err)
        return
    }
    jsonObj(c, gin.H{"qr": qr}, nil)
}
