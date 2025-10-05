package game

import (
	"fmt"
	"math"
	"math/rand"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

// Player представляет игрока
type Player struct {
	ID             string    `json:"id"`
	UserID         string    `json:"user_id"`
	Username       string    `json:"username"`
	X              float64   `json:"x"`
	Y              float64   `json:"y"`
	Color          string    `json:"color"`
	PortalCooldown time.Time `json:"-"`
	CurrentZone    string    `json:"-"`
}

// GameMessage представляет сообщение между клиентом и сервером
type GameMessage struct {
	Type string      `json:"type"`
	Data interface{} `json:"data,omitempty"`
}

// MoveData представляет данные о движении
type MoveData struct {
	DX float64 `json:"dx"`
	DY float64 `json:"dy"`
}

// Portal represents a teleportation portal
type Portal struct {
	ID   string
	X    float64
	Y    float64
	To   string // ID of connected portal
	Zone string
}

// Zone represents a game zone with boundaries
type Zone struct {
	Name  string
	MinX  float64
	MaxX  float64
	MinY  float64
	MaxY  float64
	Color string
}

type Game struct {
	players       map[string]*Player
	playersMu     sync.RWMutex
	worldWidth    float64
	worldHeight   float64
	colors        []string
	connections   map[string]*websocket.Conn
	connectionsMu sync.RWMutex
	portals       map[string]*Portal
	portalLinks   map[string]string // portal connections
	zones         map[string]*Zone  // game zones
}

func NewGame() *Game {
	rand.Seed(time.Now().UnixNano())

	g := &Game{
		players:     make(map[string]*Player),
		worldWidth:  7000.0,
		worldHeight: 1000.0,
		colors:      []string{"#FF6B6B", "#4ECDC4", "#45B7D1", "#96CEB4", "#FFEAA7", "#DDA0DD", "#98FB98", "#FFD700"},
		connections: make(map[string]*websocket.Conn),
		portals:     make(map[string]*Portal),
		portalLinks: make(map[string]string),
		zones:       make(map[string]*Zone),
	}

	// Initialize zones and portals
	g.initZones()
	g.initPortals()

	// Запускаем периодическую синхронизацию состояния
	go g.synchronizeGameState()

	return g
}

func (g *Game) initZones() {
	// Define zones according to the specifications
	g.zones["common"] = &Zone{
		Name:  "Common Zone",
		MinX:  0,
		MaxX:  1000,
		MinY:  0,
		MaxY:  1000,
		Color: "#666666",
	}
	g.zones["uncommon"] = &Zone{
		Name:  "Uncommon Zone",
		MinX:  1500,
		MaxX:  2500,
		MinY:  0,
		MaxY:  1000,
		Color: "#00FF00",
	}
	g.zones["rare"] = &Zone{
		Name:  "Rare Zone",
		MinX:  3000,
		MaxX:  4000,
		MinY:  0,
		MaxY:  1000,
		Color: "#0088FF",
	}
	g.zones["epic"] = &Zone{
		Name:  "Epic Zone",
		MinX:  4500,
		MaxX:  5500,
		MinY:  0,
		MaxY:  1000,
		Color: "#FF00FF",
	}
	g.zones["legendary"] = &Zone{
		Name:  "Legendary Zone",
		MinX:  6000,
		MaxX:  7000,
		MinY:  0,
		MaxY:  1000,
		Color: "#FFAA00",
	}

	fmt.Println("Zones initialized:")
	for name, zone := range g.zones {
		fmt.Printf("Zone %s: (%.0f-%.0f, %.0f-%.0f)\n", name, zone.MinX, zone.MaxX, zone.MinY, zone.MaxY)
	}
}

func (g *Game) initPortals() {
	// Define all portals
	portals := []*Portal{
		{ID: "P1", X: 800, Y: 500, Zone: "common"},
		{ID: "P2", X: 1700, Y: 500, Zone: "uncommon"},
		{ID: "P3", X: 2300, Y: 500, Zone: "uncommon"},
		{ID: "P4", X: 3200, Y: 500, Zone: "rare"},
		{ID: "P5", X: 3800, Y: 500, Zone: "rare"},
		{ID: "P6", X: 4700, Y: 500, Zone: "epic"},
		{ID: "P7", X: 5300, Y: 500, Zone: "epic"},
		{ID: "P8", X: 6200, Y: 500, Zone: "legendary"},
	}

	// Store portals in map
	for _, portal := range portals {
		g.portals[portal.ID] = portal
	}

	// Define portal connections (bidirectional)
	connections := map[string]string{
		"P1": "P2",
		"P2": "P1",
		"P3": "P4",
		"P4": "P3",
		"P5": "P6",
		"P6": "P5",
		"P7": "P8",
		"P8": "P7",
	}

	g.portalLinks = connections

	// Set the "To" field for each portal
	for fromID, toID := range connections {
		if portal, exists := g.portals[fromID]; exists {
			portal.To = toID
		}
	}

	fmt.Println("Portals initialized with connections:")
	for _, portal := range g.portals {
		fmt.Printf("Portal %s at (%.0f, %.0f) -> %s\n", portal.ID, portal.X, portal.Y, portal.To)
	}
}

// getPlayerZone determines which zone the player is currently in
func (g *Game) getPlayerZone(x, y float64) string {
	for name, zone := range g.zones {
		if x >= zone.MinX && x <= zone.MaxX && y >= zone.MinY && y <= zone.MaxY {
			return name
		}
	}
	return "" // No zone found (in gap between zones)
}

// constrainToZone restricts player movement to stay within their current zone
func (g *Game) constrainToZone(player *Player, newX, newY float64) (float64, float64) {
	currentZone := g.getPlayerZone(player.X, player.Y)
	if currentZone == "" {
		// Player is between zones, find the nearest zone
		var nearestZone *Zone
		minDistance := math.MaxFloat64

		for _, zone := range g.zones {
			// Calculate distance to zone center
			zoneCenterX := (zone.MinX + zone.MaxX) / 2
			zoneCenterY := (zone.MinY + zone.MaxY) / 2
			distance := math.Sqrt(math.Pow(player.X-zoneCenterX, 2) + math.Pow(player.Y-zoneCenterY, 2))

			if distance < minDistance {
				minDistance = distance
				nearestZone = zone
			}
		}

		if nearestZone != nil {
			// Teleport player to the nearest zone center
			player.CurrentZone = nearestZone.Name
			return (nearestZone.MinX + nearestZone.MaxX) / 2, (nearestZone.MinY + nearestZone.MaxY) / 2
		}
		return player.X, player.Y // Can't move
	}

	zone := g.zones[currentZone]
	player.CurrentZone = currentZone

	// Constrain X coordinate
	if newX < zone.MinX {
		newX = zone.MinX
	} else if newX > zone.MaxX {
		newX = zone.MaxX
	}

	// Constrain Y coordinate
	if newY < zone.MinY {
		newY = zone.MinY
	} else if newY > zone.MaxY {
		newY = zone.MaxY
	}

	return newX, newY
}

func (g *Game) synchronizeGameState() {
	ticker := time.NewTicker(16 * time.Millisecond) // Рассылаем состояние каждые 16 мс
	defer ticker.Stop()

	for range ticker.C {
		g.broadcastGameState()
	}
}

func (g *Game) broadcastGameState() {
	g.connectionsMu.RLock()
	defer g.connectionsMu.RUnlock()

	if len(g.connections) == 0 {
		return
	}

	// Получаем текущее состояние игры
	g.playersMu.RLock()
	playersCopy := make(map[string]*Player)
	for id, player := range g.players {
		playersCopy[id] = &Player{
			ID:       player.ID,
			UserID:   player.UserID,
			Username: player.Username,
			X:        player.X,
			Y:        player.Y,
			Color:    player.Color,
		}
	}
	g.playersMu.RUnlock()

	// Рассылаем состояние всем подключенным клиентам
	for playerID, conn := range g.connections {
		// Get the player's current zone
		var currentZone string
		g.playersMu.RLock()
		if player, exists := g.players[playerID]; exists {
			currentZone = player.CurrentZone
		}
		g.playersMu.RUnlock()

		state := map[string]interface{}{
			"type":        "state",
			"players":     playersCopy,
			"yourId":      playerID,
			"worldWidth":  g.worldWidth,
			"worldHeight": g.worldHeight,
			"yourZone":    currentZone, // Send the player's current zone
		}

		if err := conn.WriteJSON(state); err != nil {
			fmt.Printf("Error broadcasting to player %s: %v\n", playerID, err)
			// Удаляем проблемное соединение
			g.connectionsMu.RUnlock()
			g.removeConnection(playerID)
			g.connectionsMu.RLock()
		}
	}
}

func (g *Game) AddPlayer(conn *websocket.Conn, userID string, username string) *Player {
	g.playersMu.Lock()
	defer g.playersMu.Unlock()

	playerID := fmt.Sprintf("%d", time.Now().UnixNano()+rand.Int63())

	// Spawn player in a random zone
	spawnX := rand.Float64() * 250           // от 0 до 250
	spawnY := rand.Float64() * g.worldHeight // от 0 до 1000 (worldHeight = 1000)

	player := &Player{
		ID:          playerID,
		UserID:      userID,
		Username:    username,
		X:           spawnX,
		Y:           spawnY,
		Color:       g.colors[rand.Intn(len(g.colors))],
		CurrentZone: "common",
	}

	g.players[playerID] = player

	g.connectionsMu.Lock()
	g.connections[playerID] = conn
	g.connectionsMu.Unlock()

	fmt.Printf("New player connected: %s (user: %s) at (%.1f, %.1f) in %s zone\n",
		playerID, username, player.X, player.Y, "Common")
	return player
}

func (g *Game) RemovePlayer(playerID string) {
	g.playersMu.Lock()
	defer g.playersMu.Unlock()

	delete(g.players, playerID)
	g.removeConnection(playerID)

	fmt.Printf("Player %s removed from game\n", playerID)
}

func (g *Game) removeConnection(playerID string) {
	g.connectionsMu.Lock()
	defer g.connectionsMu.Unlock()

	delete(g.connections, playerID)
}

func (g *Game) MovePlayer(playerID string, dx, dy float64) {
	g.playersMu.Lock()
	defer g.playersMu.Unlock()

	if player := g.players[playerID]; player != nil {
		// Calculate new position
		newX := player.X + dx
		newY := player.Y + dy

		// Constrain movement to current zone
		newX, newY = g.constrainToZone(player, newX, newY)

		// Update position
		player.X = newX
		player.Y = newY

		// Check for portal interaction
		g.checkPortalInteraction(player)

		fmt.Printf("Player %s moved to (%.1f, %.1f) in %s zone\n",
			playerID, player.X, player.Y, player.CurrentZone)
	}
}

func (g *Game) checkPortalInteraction(player *Player) {
	// Check if player is on cooldown
	if time.Now().Before(player.PortalCooldown) {
		return
	}

	// Check distance to each portal
	for _, portal := range g.portals {
		distance := math.Sqrt(math.Pow(player.X-portal.X, 2) + math.Pow(player.Y-portal.Y, 2))

		if distance <= 100 { // Portal radius
			g.teleportPlayer(player, portal)
			break // Only teleport to one portal at a time
		}
	}
}

func (g *Game) teleportPlayer(player *Player, fromPortal *Portal) {
	toPortalID := fromPortal.To
	toPortal, exists := g.portals[toPortalID]

	if !exists {
		fmt.Printf("Error: Destination portal %s not found for portal %s\n", toPortalID, fromPortal.ID)
		return
	}

	// Teleport player to destination portal
	player.X = toPortal.X
	player.Y = toPortal.Y
	player.CurrentZone = toPortal.Zone

	// Set cooldown (10 seconds)
	player.PortalCooldown = time.Now().Add(10 * time.Second)

	fmt.Printf("Player %s teleported from %s (%s) to %s (%s). Cooldown until: %s\n",
		player.ID, fromPortal.ID, fromPortal.Zone, toPortal.ID, toPortal.Zone,
		player.PortalCooldown.Format("15:04:05"))

	// Send portal notification to player
	g.sendPortalNotification(player, fromPortal, toPortal)
}

func (g *Game) sendPortalNotification(player *Player, fromPortal, toPortal *Portal) {
	g.connectionsMu.RLock()
	defer g.connectionsMu.RUnlock()

	conn, exists := g.connections[player.ID]
	if !exists {
		return
	}

	notification := map[string]interface{}{
		"type": "portal_teleport",
		"data": map[string]interface{}{
			"fromPortal": fromPortal.ID,
			"toPortal":   toPortal.ID,
			"fromZone":   fromPortal.Zone,
			"toZone":     toPortal.Zone,
			"cooldown":   10,
		},
	}

	conn.WriteJSON(notification)
}

func (g *Game) GetGameState(playerID string) map[string]interface{} {
	g.playersMu.RLock()
	defer g.playersMu.RUnlock()

	playersCopy := make(map[string]*Player)
	for id, player := range g.players {
		playersCopy[id] = &Player{
			ID:       player.ID,
			UserID:   player.UserID,
			Username: player.Username,
			X:        player.X,
			Y:        player.Y,
			Color:    player.Color,
		}
	}

	return map[string]interface{}{
		"type":        "state",
		"players":     playersCopy,
		"yourId":      playerID,
		"worldWidth":  g.worldWidth,
		"worldHeight": g.worldHeight,
	}
}

func (g *Game) GetPlayersCount() int {
	g.playersMu.RLock()
	defer g.playersMu.RUnlock()
	return len(g.players)
}
