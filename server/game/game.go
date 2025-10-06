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
	Speed          float64   `json:"speed"`
	PortalCooldown time.Time `json:"-"`
	CurrentZone    string    `json:"-"`
	Radius         float64   `json:"-"` // Радиус коллизии
}

type Mob struct {
	ID        string  `json:"id"`
	Type      string  `json:"type"`
	Health    int     `json:"health"`
	MaxHealth int     `json:"max_health"`
	Damage    int     `json:"damage"`
	Speed     float64 `json:"speed"`
	X         float64 `json:"x"`
	Y         float64 `json:"y"`
	Zone      string  `json:"zone"`
	Radius    float64 `json:"-"`
}

// Константы мобов
const (
	MaxMobsPerZone = 100
	MobRadius      = 20.0
)

// Константы коллизий
const (
	PlayerRadius    = 15.0
	CollisionBuffer = 5.0 // Буфер для более плавного избегания
	AvoidanceForce  = 2.0 // Сила избегания других игроков
)

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
	connections   map[string]*playerConnection // Changed to playerConnection
	connectionsMu sync.RWMutex
	portals       map[string]*Portal
	portalLinks   map[string]string
	zones         map[string]*Zone
	mobs          map[string]*Mob
	mobsMu        sync.RWMutex
}

type playerConnection struct {
	conn *websocket.Conn
	mu   sync.Mutex
}

func NewGame() *Game {
	g := &Game{
		players:     make(map[string]*Player),
		worldWidth:  7000.0,
		worldHeight: 1000.0,
		colors:      []string{"#FF6B6B", "#4ECDC4", "#45B7D1", "#96CEB4", "#FFEAA7", "#DDA0DD", "#98FB98", "#FFD700"},
		connections: make(map[string]*playerConnection),
		portals:     make(map[string]*Portal),
		portalLinks: make(map[string]string),
		zones:       make(map[string]*Zone),
		mobs:        make(map[string]*Mob),
	}

	g.initZones()
	g.initPortals()
	g.spawnMobs()

	go g.synchronizeGameState()
	return g
}

func (g *Game) spawnMobs() {
	g.mobsMu.Lock()
	defer g.mobsMu.Unlock()

	mobTypes := []struct {
		mobType string
		health  int
		damage  int
		speed   float64
	}{
		{"goblin", 50, 10, 2.0},
		{"orc", 80, 15, 1.5},
		{"wolf", 30, 8, 3.0},
	}

	for zoneName := range g.zones {
		mobsInZone := 0

		for i := 0; i < MaxMobsPerZone && mobsInZone < 30; i++ {
			mobType := mobTypes[rand.Intn(len(mobTypes))]

			// Находим случайную позицию в зоне
			zone := g.zones[zoneName]
			x := zone.MinX + rand.Float64()*(zone.MaxX-zone.MinX)
			y := zone.MinY + rand.Float64()*(zone.MaxY-zone.MinY)

			// Проверяем, чтобы не спавнить слишком близко к игрокам
			if g.isPositionSafeForMob(x, y) {
				mobID := fmt.Sprintf("mob_%s_%d", zoneName, len(g.mobs))

				mob := &Mob{
					ID:        mobID,
					Type:      mobType.mobType,
					Health:    mobType.health,
					MaxHealth: mobType.health,
					Damage:    mobType.damage,
					Speed:     mobType.speed,
					X:         x,
					Y:         y,
					Zone:      zoneName,
					Radius:    MobRadius,
				}

				g.mobs[mobID] = mob
				mobsInZone++
				fmt.Printf("Spawned %s at (%.1f, %.1f) in %s zone\n", mobType.mobType, x, y, zoneName)
			}
		}
	}
}

// Проверка безопасности позиции для моба
func (g *Game) isPositionSafeForMob(x, y float64) bool {
	g.playersMu.RLock()
	defer g.playersMu.RUnlock()

	for _, player := range g.players {
		distance := math.Sqrt(math.Pow(x-player.X, 2) + math.Pow(y-player.Y, 2))
		if distance < 200 { // Минимальная дистанция от игроков
			return false
		}
	}
	return true
}

func (g *Game) initZones() {
	// Define zones according to the new specifications
	g.zones["common"] = &Zone{
		Name:  "Common Zone",
		MinX:  0,
		MaxX:  6000,
		MinY:  0,
		MaxY:  3000,
		Color: "#666666",
	}
	g.zones["uncommon"] = &Zone{
		Name:  "Uncommon Zone",
		MinX:  7000,
		MaxX:  13000,
		MinY:  0,
		MaxY:  3000,
		Color: "#00FF00",
	}
	g.zones["rare"] = &Zone{
		Name:  "Rare Zone",
		MinX:  14000,
		MaxX:  20000,
		MinY:  0,
		MaxY:  3000,
		Color: "#0088FF",
	}
	g.zones["epic"] = &Zone{
		Name:  "Epic Zone",
		MinX:  21000,
		MaxX:  27000,
		MinY:  0,
		MaxY:  3000,
		Color: "#FF00FF",
	}
	g.zones["legendary"] = &Zone{
		Name:  "Legendary Zone",
		MinX:  28000,
		MaxX:  34000,
		MinY:  0,
		MaxY:  3000,
		Color: "#FFAA00",
	}

	// Update world dimensions to accommodate new zones
	g.worldWidth = 34000.0
	g.worldHeight = 3000.0

	fmt.Println("Zones initialized:")
	for name, zone := range g.zones {
		fmt.Printf("Zone %s: (%.0f-%.0f, %.0f-%.0f)\n", name, zone.MinX, zone.MaxX, zone.MinY, zone.MaxY)
	}
}

func (g *Game) filterObjectsByZone(zoneName string) (map[string]*Player, map[string]*Mob) {
	g.playersMu.RLock()
	defer g.playersMu.RUnlock()
	g.mobsMu.RLock()
	defer g.mobsMu.RUnlock()

	filteredPlayers := make(map[string]*Player)
	filteredMobs := make(map[string]*Mob)

	// Фильтруем игроков
	for id, player := range g.players {
		if player.CurrentZone == zoneName {
			filteredPlayers[id] = &Player{
				ID:       player.ID,
				UserID:   player.UserID,
				Username: player.Username,
				X:        player.X,
				Y:        player.Y,
				Color:    player.Color,
			}
		}
	}

	// Фильтруем мобов
	for id, mob := range g.mobs {
		if mob.Zone == zoneName {
			filteredMobs[id] = &Mob{
				ID:        mob.ID,
				Type:      mob.Type,
				Health:    mob.Health,
				MaxHealth: mob.MaxHealth,
				Damage:    mob.Damage,
				X:         mob.X,
				Y:         mob.Y,
				Zone:      mob.Zone,
			}
		}
	}

	return filteredPlayers, filteredMobs
}

func (g *Game) initPortals() {
	// Define all portals with new positions
	portals := []*Portal{
		{ID: "P1", X: 5800, Y: 1500, Zone: "common"},
		{ID: "P2", X: 7200, Y: 1500, Zone: "uncommon"},
		{ID: "P3", X: 12800, Y: 1500, Zone: "uncommon"},
		{ID: "P4", X: 14200, Y: 1500, Zone: "rare"},
		{ID: "P5", X: 19800, Y: 1500, Zone: "rare"},
		{ID: "P6", X: 21200, Y: 1500, Zone: "epic"},
		{ID: "P7", X: 26800, Y: 1500, Zone: "epic"},
		{ID: "P8", X: 28200, Y: 1500, Zone: "legendary"},
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
	// First, copy the connections map without holding the lock for a long time
	g.connectionsMu.RLock()
	connectionsCopy := make(map[string]*playerConnection, len(g.connections))
	for id, pc := range g.connections {
		connectionsCopy[id] = pc
	}
	g.connectionsMu.RUnlock()

	if len(connectionsCopy) == 0 {
		return
	}

	// Broadcast to all connections with zone-based filtering
	for playerID, pc := range connectionsCopy {
		var currentZone string
		g.playersMu.RLock()
		if player, exists := g.players[playerID]; exists {
			currentZone = player.CurrentZone
		}
		g.playersMu.RUnlock()

		// Если зона не определена, используем common как дефолт
		if currentZone == "" {
			currentZone = "common"
		}

		// Получаем объекты только из текущей зоны игрока
		playersInZone, mobsInZone := g.filterObjectsByZone(currentZone)

		state := map[string]interface{}{
			"type":        "state",
			"players":     playersInZone,
			"mobs":        mobsInZone,
			"yourId":      playerID,
			"worldWidth":  g.worldWidth,
			"worldHeight": g.worldHeight,
			"yourZone":    currentZone,
		}

		pc.mu.Lock()
		err := pc.conn.WriteJSON(state)
		pc.mu.Unlock()

		if err != nil {
			fmt.Printf("Error broadcasting to player %s: %v\n", playerID, err)
		}
	}
}

func (g *Game) AddPlayer(conn *websocket.Conn, userID string, username string) *Player {
	g.playersMu.Lock()
	defer g.playersMu.Unlock()

	playerID := fmt.Sprintf("%d", time.Now().UnixNano()+rand.Int63())
	spawnX, spawnY := g.findSafeSpawnPosition("common", "")

	player := &Player{
		ID:          playerID,
		UserID:      userID,
		Username:    username,
		X:           spawnX,
		Y:           spawnY,
		Color:       g.colors[rand.Intn(len(g.colors))],
		Speed:       5.0,
		CurrentZone: "common",
		Radius:      PlayerRadius,
	}

	g.players[playerID] = player

	g.connectionsMu.Lock()
	g.connections[playerID] = &playerConnection{
		conn: conn,
		mu:   sync.Mutex{},
	}
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

	// Close the connection before removing
	if pc, exists := g.connections[playerID]; exists {
		pc.conn.Close()
		delete(g.connections, playerID)
	}
}
func (g *Game) MovePlayer(playerID string, dx, dy float64) {
	g.playersMu.Lock()
	defer g.playersMu.Unlock()

	if player := g.players[playerID]; player != nil {
		// Нормализуем вектор направления (чтобы диагональное движение не было быстрее)
		length := math.Sqrt(dx*dx + dy*dy)
		if length > 0 {
			dx = dx / length
			dy = dy / length
		}

		// Умножаем на скорость игрока
		movementX := dx * player.Speed
		movementY := dy * player.Speed

		// Рассчитываем новую позицию
		newX := player.X + movementX
		newY := player.Y + movementY

		// Применяем ограничения зоны
		newX, newY = g.constrainToZone(player, newX, newY)

		// Применяем избегание других игроков
		finalX, finalY := g.avoidOtherPlayers(player, newX, newY)

		// Обновляем позицию
		player.X = finalX
		player.Y = finalY

		// Проверяем взаимодействие с порталами
		g.checkPortalInteraction(player)

		fmt.Printf("Player %s moved to (%.1f, %.1f) in %s zone (speed: %.1f)\n",
			playerID, player.X, player.Y, player.CurrentZone, player.Speed)
	}
}

// SetPlayerSpeed устанавливает скорость игрока
func (g *Game) SetPlayerSpeed(playerID string, speed float64) {
	g.playersMu.Lock()
	defer g.playersMu.Unlock()

	if player, exists := g.players[playerID]; exists {
		player.Speed = speed
		fmt.Printf("Player %s speed set to %.1f\n", playerID, speed)
	}
}

// ResetPlayerSpeed сбрасывает скорость к базовому значению
func (g *Game) ResetPlayerSpeed(playerID string) {
	g.playersMu.Lock()
	defer g.playersMu.Unlock()

	if player, exists := g.players[playerID]; exists {
		player.Speed = 5.0 // Базовая скорость
		fmt.Printf("Player %s speed reset to %.1f\n", playerID, player.Speed)
	}
}

// avoidOtherPlayers предотвращает прохождение через других игроков
func (g *Game) avoidOtherPlayers(movingPlayer *Player, targetX, targetY float64) (float64, float64) {
	finalX, finalY := targetX, targetY

	for _, otherPlayer := range g.players {
		if otherPlayer.ID == movingPlayer.ID {
			continue
		}

		// Вычисляем расстояние до другого игрока
		dx := finalX - otherPlayer.X
		dy := finalY - otherPlayer.Y
		distance := math.Sqrt(dx*dx + dy*dy)
		minDistance := movingPlayer.Radius + otherPlayer.Radius + CollisionBuffer

		// Если слишком близко, корректируем движение с учетом скорости
		if distance < minDistance {
			// Вычисляем вектор отталкивания
			angle := math.Atan2(dy, dx)
			desiredDistance := minDistance

			// Корректируем позицию, чтобы сохранить минимальную дистанцию
			// Учитываем скорость для более плавного избегания
			avoidanceX := otherPlayer.X + math.Cos(angle)*desiredDistance
			avoidanceY := otherPlayer.Y + math.Sin(angle)*desiredDistance

			// Плавное перемещение к безопасной позиции
			finalX = finalX + (avoidanceX-finalX)*0.3
			finalY = finalY + (avoidanceY-finalY)*0.3

			fmt.Printf("Player %s avoiding %s, distance: %.1f\n",
				movingPlayer.ID, otherPlayer.ID, distance)
		}
	}

	return finalX, finalY
}

// Поиск безопасной позиции для спавна
func (g *Game) findSafeSpawnPosition(zoneName, excludePlayerID string) (float64, float64) {
	zone := g.zones[zoneName]

	// Пробуем несколько случайных позиций
	for i := 0; i < 20; i++ {
		x := zone.MinX + rand.Float64()*(zone.MaxX-zone.MinX)
		y := zone.MinY + rand.Float64()*(zone.MaxY-zone.MinY)

		if g.isPositionSafeForSpawn(x, y, excludePlayerID) {
			return x, y
		}
	}

	// Если не нашли, возвращаем центр зоны
	return (zone.MinX + zone.MaxX) / 2, (zone.MinY + zone.MaxY) / 2
}

// Проверка позиции для спавна
func (g *Game) isPositionSafeForSpawn(x, y float64, excludePlayerID string) bool {
	for _, player := range g.players {
		if player.ID == excludePlayerID {
			continue
		}

		dx := x - player.X
		dy := y - player.Y
		distance := math.Sqrt(dx*dx + dy*dy)

		// Больший радиус для спавна, чтобы избежать immediate коллизий
		if distance < (PlayerRadius * 3) {
			return false
		}
	}
	return true
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

	fmt.Printf("Player %s teleported from %s (%s) to %s (%s). Zone changed.\n",
		player.ID, fromPortal.ID, fromPortal.Zone, toPortal.ID, toPortal.Zone)

	// Send portal notification to player
	g.sendPortalNotification(player, fromPortal, toPortal)
}

func (g *Game) sendPortalNotification(player *Player, fromPortal, toPortal *Portal) {
	g.connectionsMu.RLock()
	defer g.connectionsMu.RUnlock()

	pc, exists := g.connections[player.ID]
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

	pc.mu.Lock()
	defer pc.mu.Unlock()
	pc.conn.WriteJSON(notification)
}

func (g *Game) GetGameState(playerID string) map[string]interface{} {
	g.playersMu.RLock()
	defer g.playersMu.RUnlock()

	var currentZone string
	if player, exists := g.players[playerID]; exists {
		currentZone = player.CurrentZone
	}
	if currentZone == "" {
		currentZone = "common"
	}

	// Используем фильтрацию по зоне
	playersInZone, mobsInZone := g.filterObjectsByZone(currentZone)

	return map[string]interface{}{
		"type":        "state",
		"players":     playersInZone,
		"mobs":        mobsInZone,
		"yourId":      playerID,
		"worldWidth":  g.worldWidth,
		"worldHeight": g.worldHeight,
		"yourZone":    currentZone,
	}
}

func (g *Game) GetPlayersCount() int {
	g.playersMu.RLock()
	defer g.playersMu.RUnlock()
	return len(g.players)
}
