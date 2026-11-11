package main

import (
	"context"
	"encoding/json"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"sync"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/huin/goupnp"
	"github.com/huin/goupnp/dcps/internetgateway1"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

type Mapping struct {
	gorm.Model
	LanIP       string `json:"lan_ip"`
	LanPort     int    `json:"lan_port"`
	WanPort     int    `json:"wan_port"`
	Protocol    string `json:"protocol"` // TCP/UDP
	LeaseSec    int    `json:"lease_sec"` // 0 = 永久
	Description string `json:"description"`
	Enabled     bool   `json:"enabled"`
	Client      *internetgateway1.WANIPConnection1 `gorm:"-"`
	Cancel      context.CancelFunc               `gorm:"-"`
}

var (
	db   *gorm.DB
	mu   sync.Mutex
	gw   *internetgateway1.WANIPConnection1
)

func main() {
	// 1. 初始化 DB
	var err error
	db, err = gorm.Open(sqlite.Open("/data/upnp.db"), &gorm.Config{})
	if err != nil {
		log.Fatal(err)
	}
	db.AutoMigrate(&Mapping{})

	// 2. 发现网关
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	clients, err := goupnp.DiscoverClients(ctx, "urn:schemas-upnp-org:service:WANIPConnection:1")
	if err != nil || len(clients) == 0 {
		log.Fatal("未发现 UPnP 网关:", err)
	}
	gw = clients[0].(*internetgateway1.WANIPConnection1)
	log.Printf("网关: %s", gw.Location())

	// 3. 启动时恢复现有映射
	restoreMappings()

	// 4. HTTP API
	r := gin.Default()
	r.Use(cors.Default())
	r.StaticFile("/", "./web/index.html")
	r.StaticFile("/favicon.svg", "./web/favicon.svg")

	api := r.Group("/api")
	{
		api.GET("/mappings", listMappings)
		api.POST("/mappings", addMapping)
		api.DELETE("/mappings/:id", deleteMapping)
		api.POST("/mappings/:id/toggle", toggleMapping)
		api.GET("/gateway", gatewayInfo)
	}

	// 5. 优雅关闭
	srv := &http.Server{Addr: ":8080", Handler: r}
	go func() {
		log.Fatal(srv.ListenAndServe())
	}()
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt)
	<-quit
	ctx, cancel = context.WithTimeout(context.Background(), 3*time.Second)
	srv.Shutdown(ctx)
	cancel()
}

// ---------- Handlers ----------
func listMappings(c *gin.Context) {
	var ms []Mapping
	db.Where("enabled = ?", true).Find(&ms)
	c.JSON(200, ms)
}

func addMapping(c *gin.Context) {
	var m Mapping
	if err := c.ShouldBindJSON(&m); err != nil {
		c.JSON(400, gin.H{"err": err.Error()})
		return
	}
	if m.LanIP == "" {
		m.LanIP = getLanIP()
	}
	if m.LeaseSec < 0 {
		m.LeaseSec = 0
	}

	ctx := context.Background()
	err := gw.AddPortMapping(ctx, "", uint16(m.WanPort), m.LanIP, uint16(m.LanPort),
		m.Protocol, m.Description, false, uint32(m.LeaseSec))
	if err != nil {
		c.JSON(500, gin.H{"err": err.Error()})
		return
	}

	m.Enabled = true
	db.Create(&m)
	startLeaseWatcher(&m)
	c.JSON(201, m)
}

func deleteMapping(c *gin.Context) {
	id := c.Param("id")
	var m Mapping
	if err := db.First(&m, id).Error; err != nil {
		c.JSON(404, gin.H{"err": "not found"})
		return
	}
	ctx := context.Background()
	gw.DeletePortMapping(ctx, "", uint16(m.WanPort), m.Protocol)
	if m.Cancel != nil {
		m.Cancel()
	}
	db.Delete(&m)
	c.Status(204)
}

func toggleMapping(c *gin.Context) {
	id := c.Param("id")
	var m Mapping
	if err := db.First(&m, id).Error; err != nil {
		c.JSON(404, gin.H{"err": "not found"})
		return
	}
	m.Enabled = !m.Enabled
	if m.Enabled {
		ctx := context.Background()
		gw.AddPortMapping(ctx, "", uint16(m.WanPort), m.LanIP, uint16(m.LanPort),
			m.Protocol, m.Description, false, uint32(m.LeaseSec))
		startLeaseWatcher(&m)
	} else {
		ctx := context.Background()
		gw.DeletePortMapping(ctx, "", uint16(m.WanPort), m.Protocol)
		if m.Cancel != nil {
			m.Cancel()
		}
	}
	db.Save(&m)
	c.JSON(200, m)
}

func gatewayInfo(c *gin.Context) {
	ctx := context.Background()
	ip, _ := gw.GetExternalIPAddress(ctx)
	c.JSON(200, gin.H{"external_ip": ip, "location": gw.Location()})
}

// ---------- Helpers ----------
func restoreMappings() {
	var ms []Mapping
	db.Where("enabled = ?", true).Find(&ms)
	for _, m := range ms {
		ctx := context.Background()
		// 先删再加，保证租约刷新
		gw.DeletePortMapping(ctx, "", uint16(m.WanPort), m.Protocol)
		gw.AddPortMapping(ctx, "", uint16(m.WanPort), m.LanIP, uint16(m.LanPort),
			m.Protocol, m.Description, false, uint32(m.LeaseSec))
		startLeaseWatcher(&m)
	}
}

func startLeaseWatcher(m *Mapping) {
	if m.LeaseSec == 0 {
		return // 永久
	}
	if m.Cancel != nil {
		m.Cancel()
	}
	ctx, cancel := context.WithCancel(context.Background())
	m.Cancel = cancel
	go func() {
		ticker := time.NewTicker(time.Duration(m.LeaseSec/2) * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				gw.AddPortMapping(context.Background(), "", uint16(m.WanPort),
					m.LanIP, uint16(m.LanPort), m.Protocol, m.Description, false, uint32(m.LeaseSec))
			}
		}
	}()
}

func getLanIP() string {
	addrs, _ := net.InterfaceAddrs()
	for _, a := range addrs {
		if ipnet, ok := a.(*net.IPNet); ok && !ipnet.IP.IsLoopback() && ipnet.IP.To4() != nil {
			return ipnet.IP.String()
		}
	}
	return "127.0.0.1"
}