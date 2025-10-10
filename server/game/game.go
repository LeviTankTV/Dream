package game

import (
	"fmt"
	"math"
	"math/rand"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

// Константы
const (
	PlayerRadius    = 15.0
	CollisionBuffer = 5.0
)

// GameMessage — сообщение между клиентом и сервером
type GameMessage struct {
	Type string      `json:"type"`
	Data interface{} `json:"data,omitempty"`
}

// Portal — портал между зонами
type Portal struct {
	ID   string
	X    float64
	Y    float64
	To   string // ID портала назначения
	Zone string
}

// Zone — игровая зона
type Zone struct {
	Name  string
	MinX  float64
	MaxX  float64
	MinY  float64
	MaxY  float64
	Color string
}

// Game — основной игровой мир
type Game struct {
	mu      sync.RWMutex // 🔑 ОДИН МЬЮТЕКС НА ВСЁ
	players map[string]*Player
	mobs    map[string]*Mob

	connections map[string]*websocket.Conn // прямые соединения
	portals     map[string]*Portal
	zones       map[string]*Zone
	worldWidth  float64
	worldHeight float64
	colors      []string

	petalDrops map[string]*PetalDrop // Добавить это поле
	petals     map[string]*Petal     // И это
}

// NewGame создаёт новый игровой мир
func NewGame() *Game {
	g := &Game{
		players:     make(map[string]*Player),
		mobs:        make(map[string]*Mob),
		connections: make(map[string]*websocket.Conn),
		portals:     make(map[string]*Portal),
		zones:       make(map[string]*Zone),
		colors:      []string{"#FF6B6B", "#4ECDC4", "#45B7D1", "#96CEB4", "#FFEAA7", "#DDA0DD", "#98FB98", "#FFD700"},

		petalDrops: make(map[string]*PetalDrop),
		petals:     make(map[string]*Petal),
	}

	g.initZones()
	g.initPortals()

	// Запускаем игровые циклы
	go g.synchronizeGameState()
	go g.mobBehaviorLoop()
	go g.mobSpawnLoop()
	go g.collisionLoop()
	go g.petalSystemLoop()

	return g
}

func (g *Game) collisionLoop() {
	ticker := time.NewTicker(100 * time.Millisecond) // 10 раз в секунду
	defer ticker.Stop()

	for range ticker.C {
		g.checkCollisions()
	}
}

// initZones — инициализация зон
func (g *Game) initZones() {
	g.zones["common"] = &Zone{MinX: 0, MaxX: 6000, MinY: 0, MaxY: 3000, Color: "#666666"}
	g.zones["uncommon"] = &Zone{MinX: 7000, MaxX: 13000, MinY: 0, MaxY: 3000, Color: "#00FF00"}
	g.zones["rare"] = &Zone{MinX: 14000, MaxX: 20000, MinY: 0, MaxY: 3000, Color: "#0088FF"}
	g.zones["epic"] = &Zone{MinX: 21000, MaxX: 27000, MinY: 0, MaxY: 3000, Color: "#FF00FF"}
	g.zones["legendary"] = &Zone{MinX: 28000, MaxX: 34000, MinY: 0, MaxY: 3000, Color: "#FFAA00"}

	g.worldWidth = 34000.0
	g.worldHeight = 3000.0
	fmt.Println("✅ Zones initialized")
}

// initPortals — инициализация порталов
func (g *Game) initPortals() {
	portals := []*Portal{
		{"P1", 5800, 1500, "P2", "common"},
		{"P2", 7200, 1500, "P1", "uncommon"},
		{"P3", 12800, 1500, "P4", "uncommon"},
		{"P4", 14200, 1500, "P3", "rare"},
		{"P5", 19800, 1500, "P6", "rare"},
		{"P6", 21200, 1500, "P5", "epic"},
		{"P7", 26800, 1500, "P8", "epic"},
		{"P8", 28200, 1500, "P7", "legendary"},
	}

	for _, p := range portals {
		g.portals[p.ID] = p
	}
	fmt.Println("✅ Portals initialized")
}

// AddPlayer — добавляет игрока в игру
func (g *Game) AddPlayer(conn *websocket.Conn, userID, username string) *Player {
	g.mu.Lock()
	defer g.mu.Unlock()

	playerID := fmt.Sprintf("p_%d", time.Now().UnixNano())
	spawnX, spawnY := g.findSafeSpawnPosition("common", playerID)
	color := g.colors[rand.Intn(len(g.colors))]

	player := NewPlayer(playerID, userID, username, spawnX, spawnY, color)
	player.CurrentZone = "common"

	g.players[playerID] = player
	g.connections[playerID] = conn

	fmt.Printf("🆕 Player %s joined\n", playerID)
	return player
}

// RemovePlayer — удаляет игрока
func (g *Game) RemovePlayer(playerID string) {
	g.mu.Lock()
	defer g.mu.Unlock()

	if conn, ok := g.connections[playerID]; ok {
		conn.Close()
		delete(g.connections, playerID)
	}
	delete(g.players, playerID)
	fmt.Printf("👋 Player %s left\n", playerID)
}

// MovePlayer — обрабатывает движение игрока
func (g *Game) MovePlayer(playerID string, dx, dy float64) {
	g.mu.Lock()
	defer g.mu.Unlock()

	player := g.players[playerID]
	if player == nil {
		return
	}

	// Нормализуем вектор движения
	length := math.Sqrt(dx*dx + dy*dy)
	if length > 0 {
		dx /= length
		dy /= length
	}

	newX := player.X + dx*player.Speed
	newY := player.Y + dy*player.Speed

	// Ограничиваем зоной
	newX, newY = g.constrainToZone(player, newX, newY)

	// Избегаем других игроков
	for _, other := range g.players {
		if other.ID == playerID {
			continue
		}
		px, py := newX, newY
		ox, oy := other.X, other.Y
		dx := px - ox
		dy := py - oy
		distSq := dx*dx + dy*dy
		minDist := player.Radius + other.Radius + CollisionBuffer
		if distSq < minDist*minDist {
			// Отталкиваем
			angle := math.Atan2(dy, dx)
			pushX := ox + math.Cos(angle)*minDist
			pushY := oy + math.Sin(angle)*minDist
			// Плавное смешивание
			newX = newX*0.7 + pushX*0.3
			newY = newY*0.7 + pushY*0.3
		}
	}

	player.X = newX
	player.Y = newY
	g.checkPortalInteraction(player)
}

// constrainToZone — не даёт выйти за границы зоны
func (g *Game) constrainToZone(player *Player, x, y float64) (float64, float64) {
	zone := g.zones[player.CurrentZone]
	if zone == nil {
		// Если зона не найдена — телепортируем в common
		player.CurrentZone = "common"
		zone = g.zones["common"]
	}

	if x < zone.MinX {
		x = zone.MinX
	}
	if x > zone.MaxX {
		x = zone.MaxX
	}
	if y < zone.MinY {
		y = zone.MinY
	}
	if y > zone.MaxY {
		y = zone.MaxY
	}

	return x, y
}

// checkPortalInteraction — проверяет, стоит ли телепортировать
func (g *Game) checkPortalInteraction(player *Player) {
	if time.Now().Before(player.PortalCooldown) {
		return
	}

	for _, portal := range g.portals {
		dx := player.X - portal.X
		dy := player.Y - portal.Y
		if dx*dx+dy*dy <= 100*100 { // радиус 100 (без sqrt!)
			g.teleportPlayer(player, portal)
			break
		}
	}
}

// teleportPlayer — телепортирует игрока
func (g *Game) teleportPlayer(player *Player, fromPortal *Portal) {
	toPortal := g.portals[fromPortal.To]
	if toPortal == nil {
		return
	}

	player.X = toPortal.X
	player.Y = toPortal.Y
	player.CurrentZone = toPortal.Zone
	player.PortalCooldown = time.Now().Add(10 * time.Second)

	// Отправляем уведомление
	notif := map[string]interface{}{
		"type": "portal_teleport",
		"data": map[string]interface{}{
			"fromZone": fromPortal.Zone,
			"toZone":   toPortal.Zone,
		},
	}
	if conn, ok := g.connections[player.ID]; ok {
		conn.WriteJSON(notif)
	}

	fmt.Printf("🌀 %s teleported to %s zone\n", player.ID, toPortal.Zone)
}

// findSafeSpawnPosition — ищет безопасную позицию для спавна
func (g *Game) findSafeSpawnPosition(zoneName, excludeID string) (float64, float64) {
	zone := g.zones[zoneName]
	for i := 0; i < 20; i++ {
		x := zone.MinX + rand.Float64()*(zone.MaxX-zone.MinX)
		y := zone.MinY + rand.Float64()*(zone.MaxY-zone.MinY)

		safe := true
		for _, p := range g.players {
			if p.ID == excludeID {
				continue
			}
			dx := x - p.X
			dy := y - p.Y
			if dx*dx+dy*dy < (p.Radius*3)*(p.Radius*3) {
				safe = false
				break
			}
		}
		if safe {
			return x, y
		}
	}
	// fallback
	return (zone.MinX + zone.MaxX) / 2, (zone.MinY + zone.MaxY) / 2
}

// synchronizeGameState — рассылает состояние каждые 16 мс
func (g *Game) synchronizeGameState() {
	ticker := time.NewTicker(16 * time.Millisecond)
	defer ticker.Stop()

	for range ticker.C {
		g.broadcastGameState()
	}
}

func (g *Game) broadcastGameState() {
	g.mu.RLock()
	defer g.mu.RUnlock()

	for playerID, conn := range g.connections {
		player := g.players[playerID]
		if player == nil {
			continue
		}

		zone := player.CurrentZone
		if zone == "" {
			zone = "common"
		}

		// Фильтруем игроков в зоне (теперь с петалами)
		playersInZone, mobsInZone := g.filterByZone(zone)

		petalDropsInZone := make(map[string]*PetalDrop)
		for id, drop := range g.petalDrops {
			if drop.Zone == zone {
				petalDropsInZone[id] = drop
			}
		}

		// Петалы текущего игрока (для отдельного управления)
		playerPetals := make(map[string]*Petal)
		if player.Petals != nil {
			for id, petal := range player.Petals {
				playerPetals[id] = &Petal{
					ID:        petal.ID,
					Type:      petal.Type,
					Health:    petal.Health,
					MaxHealth: petal.MaxHealth,
					X:         petal.X,
					Y:         petal.Y,
					IsActive:  petal.IsActive,
				}
			}
		}

		state := map[string]interface{}{
			"type":        "state",
			"players":     playersInZone, // ← Теперь содержит петалы всех игроков в зоне
			"mobs":        mobsInZone,
			"yourId":      playerID,
			"worldWidth":  g.worldWidth,
			"worldHeight": g.worldHeight,
			"yourZone":    zone,
			"petalDrops":  petalDropsInZone,
			"petals":      playerPetals, // ← Петалы текущего игрока (для обратной совместимости)
		}

		_ = conn.WriteJSON(state)
	}
}

// mobSpawnLoop — спавнит мобов раз в 5 сек
func (g *Game) mobSpawnLoop() {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		g.spawnMobsIfNeeded()
	}
}

// spawnMobsIfNeeded — спавнит мобов, если их мало
func (g *Game) spawnMobsIfNeeded() {
	g.mu.Lock()
	defer g.mu.Unlock()

	maxMobsPerZone := 40
	mobCount := make(map[string]int)
	for _, mob := range g.mobs {
		mobCount[mob.Zone]++
	}

	mobTypes := []MobType{MobTypeOrc, MobTypeWolf, MobTypeGoblin}

	for zoneName, zone := range g.zones {
		current := mobCount[zoneName]
		if current >= maxMobsPerZone {
			continue
		}

		need := maxMobsPerZone - current
		spawned := 0
		attempts := 0
		maxAttempts := need * 5

		for spawned < need && attempts < maxAttempts {
			attempts++
			mobType := mobTypes[rand.Intn(len(mobTypes))]
			count := rand.Intn(3) + 1
			if spawned+count > need {
				count = need - spawned
			}

			for i := 0; i < count; i++ {
				x := zone.MinX + rand.Float64()*(zone.MaxX-zone.MinX)
				y := zone.MinY + rand.Float64()*(zone.MaxY-zone.MinY)

				// Проверка: далеко ли от игроков?
				safe := true
				for _, p := range g.players {
					dx := x - p.X
					dy := y - p.Y
					if dx*dx+dy*dy < 200*200 { // 200px от игрока
						safe = false
						break
					}
				}

				if safe {
					mobID := fmt.Sprintf("mob_%s_%d", zoneName, time.Now().UnixNano())
					mob := NewMob(mobID, mobType, x, y, zoneName)
					g.mobs[mobID] = mob
					spawned++
				}
			}
		}
	}
}

// mobBehaviorLoop — обновляет поведение мобов
func (g *Game) mobBehaviorLoop() {
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for range ticker.C {
		g.UpdateMobs() // предполагается, что UpdateMobs использует g.mu
	}
}

// GetGameState — возвращает начальное состояние для игрока
func (g *Game) GetGameState(playerID string) map[string]interface{} {
	g.mu.RLock()
	defer g.mu.RUnlock()

	player := g.players[playerID]
	if player == nil {
		return nil
	}

	zone := player.CurrentZone
	if zone == "" {
		zone = "common"
	}

	playersInZone, mobsInZone := g.filterByZone(zone)

	return map[string]interface{}{
		"type":        "state",
		"players":     playersInZone,
		"mobs":        mobsInZone,
		"yourId":      playerID,
		"worldWidth":  g.worldWidth,
		"worldHeight": g.worldHeight,
		"yourZone":    zone,
	}
}

// filterByZone — вспомогательная функция (вызывается только под RLock)
func (g *Game) filterByZone(zone string) (map[string]*Player, map[string]*Mob) {
	players := make(map[string]*Player)
	for id, p := range g.players {
		if p.CurrentZone == zone {
			players[id] = &Player{
				ID:        p.ID,
				UserID:    p.UserID,
				Username:  p.Username,
				X:         p.X,
				Y:         p.Y,
				Color:     p.Color,
				Speed:     p.Speed,
				Radius:    p.Radius,
				Health:    p.Health,
				MaxHealth: p.MaxHealth,
				Petals:    p.GetPetalsForSerialization(),
			}
		}
	}

	mobs := make(map[string]*Mob)
	for id, m := range g.mobs {
		if m.Zone == zone {
			mobs[id] = &Mob{
				ID:        m.ID,
				Type:      m.Type,
				Rarity:    m.Rarity,
				Health:    m.Health,
				MaxHealth: m.MaxHealth,
				Damage:    m.Damage,
				Speed:     m.Speed,
				X:         m.X,
				Y:         m.Y,
				Zone:      m.Zone,
				Radius:    m.Radius,
			}
		}
	}
	return players, mobs
}

func (g *Game) checkCollisions() {
	g.mu.Lock()
	defer g.mu.Unlock()

	// Создаем копии для безопасной итерации
	players := make([]*Player, 0, len(g.players))
	for _, p := range g.players {
		players = append(players, p)
	}

	mobs := make([]*Mob, 0, len(g.mobs))
	for _, m := range g.mobs {
		mobs = append(mobs, m)
	}

	// Проверяем коллизии между игроками и мобами
	for _, player := range players {
		if !player.IsAlive() {
			continue
		}

		for _, mob := range mobs {
			if !mob.IsAlive() || mob.Zone != player.CurrentZone {
				continue
			}

			distance := player.DistanceTo(mob.X, mob.Y)
			collisionDistance := player.Radius + mob.Radius

			if distance < collisionDistance {
				g.handlePlayerMobCollision(player, mob)
			}
		}
	}

	// Удаляем мертвых мобов
	g.removeDeadMobs()
}

// handlePlayerMobCollision обрабатывает коллизию игрока и моба
func (g *Game) handlePlayerMobCollision(player *Player, mob *Mob) {
	// Моб атакует игрока
	if mob.CanAttack() {
		if player.TakeDamageFromMob(mob.Damage) {
			mob.MarkAttack()

			// Отправляем уведомление игроку
			g.sendDamageNotification(player, mob.Damage)

			// Проверяем смерть игрока
			if !player.IsAlive() {
				g.handlePlayerDeath(player)
			}
		}
	}

	// Игрок атакует моба (коллизией)
	if player.CanAttack() {
		mob.TakeDamage(player.CollisionDamage)
		player.MarkAttack()

		// Если моб умер, отправляем уведомление
		if !mob.IsAlive() {
			var petalType PetalType
			switch mob.Type {
			case MobTypeWolf:
				petalType = PetalTypeWolf
			case MobTypeGoblin:
				petalType = PetalTypeGoblin
			case MobTypeOrc:
				petalType = PetalTypeOrc
			default:
				petalType = PetalTypeGoblin // fallback
			}

			// Создаем дроп лепестка
			g.createPetalDrop(player.ID, petalType, mob.X, mob.Y)
			// Отправляем уведомление
			g.sendMobDeathNotification(player, mob)
		}
	}
}

func (g *Game) createPetalDrop(playerID string, petalType PetalType, x, y float64) {
	player := g.players[playerID]
	if player == nil {
		return
	}

	drop := &PetalDrop{
		ID:       fmt.Sprintf("drop_%d", time.Now().UnixNano()),
		Type:     petalType,
		X:        x,
		Y:        y,
		OwnerID:  playerID,
		Zone:     player.CurrentZone, // ← Установите зону
		Created:  time.Now(),
		Lifetime: 30 * time.Second,
	}

	g.petalDrops[drop.ID] = drop

	// Отправляем уведомление
	if conn, ok := g.connections[playerID]; ok {
		conn.WriteJSON(map[string]interface{}{
			"type": "petal_drop_created",
			"data": map[string]interface{}{
				"id":   drop.ID,
				"type": drop.Type,
				"x":    drop.X,
				"y":    drop.Y,
			},
		})
	}
}

// handlePlayerDeath обрабатывает смерть игрока
func (g *Game) handlePlayerDeath(player *Player) {
	// Отправляем уведомление о смерти
	g.sendDeathNotification(player)
	player.RemoveAllPetals()
	// Игрок остается в игре, но становится "мертвым"
	// Он не может двигаться до возрождения
	// В будущем другие игроки смогут воскрешать его
}

// removeDeadMobs удаляет мертвых мобов
func (g *Game) removeDeadMobs() {
	deadMobs := make([]string, 0)

	for id, mob := range g.mobs {
		if !mob.IsAlive() {
			deadMobs = append(deadMobs, id)
		}
	}

	for _, id := range deadMobs {
		delete(g.mobs, id)
		fmt.Printf("☠️ Mob %s died and removed\n", id)
	}
}

// sendDamageNotification отправляет уведомление о получении урона
func (g *Game) sendDamageNotification(player *Player, damage int) {
	if conn, ok := g.connections[player.ID]; ok {
		conn.WriteJSON(map[string]interface{}{
			"type": "damage_taken",
			"data": map[string]interface{}{
				"damage":     damage,
				"health":     player.Health,
				"max_health": player.MaxHealth,
			},
		})
	}
}

// sendDeathNotification отправляет уведомление о смерти
func (g *Game) sendDeathNotification(player *Player) {
	if conn, ok := g.connections[player.ID]; ok {
		conn.WriteJSON(map[string]interface{}{
			"type": "player_died",
			"data": map[string]interface{}{
				"health": player.Health,
			},
		})
	}
}

// sendMobDeathNotification отправляет уведомление о смерти моба
func (g *Game) sendMobDeathNotification(player *Player, mob *Mob) {
	if conn, ok := g.connections[player.ID]; ok {
		conn.WriteJSON(map[string]interface{}{
			"type": "mob_killed",
			"data": map[string]interface{}{
				"mob_type": mob.Type,
				"rarity":   mob.Rarity,
				"xp":       mob.MaxHealth / 2, // Простая формула опыта
			},
		})
	}
}

// RespawnPlayer возрождает игрока
func (g *Game) RespawnPlayer(playerID string) {
	g.mu.Lock()
	defer g.mu.Unlock()

	player := g.players[playerID]
	if player == nil || player.IsAlive() {
		return
	}

	// Находим безопасную позицию для возрождения
	x, y := g.findSafeSpawnPosition("common", playerID)
	player.Respawn(x, y)

	// Отправляем уведомление о возрождении
	if conn, ok := g.connections[playerID]; ok {
		conn.WriteJSON(map[string]interface{}{
			"type": "player_respawned",
			"data": map[string]interface{}{
				"health": player.Health,
				"x":      player.X,
				"y":      player.Y,
				"zone":   player.CurrentZone,
			},
		})
	}

	fmt.Printf("🔁 Player %s respawned at (%.1f, %.1f)\n", playerID, x, y)
}

// GetPlayersCount — возвращает количество игроков
func (g *Game) GetPlayersCount() int {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return len(g.players)
}

func (g *Game) petalSystemLoop() {
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for range ticker.C {
		g.updatePetals()
		g.checkPetalDrops()
		g.checkPetalCollisions()
		g.checkPetalHealing()
	}
}

func (g *Game) updatePetals() {
	g.mu.Lock()
	defer g.mu.Unlock()

	deltaTime := 0.1 // 100ms в секундах

	for _, player := range g.players {
		for _, petal := range player.Petals {
			if petal.IsActive {
				// Обновляем позицию лепестка
				petalX, petalY := petal.UpdatePosition(player.X, player.Y, deltaTime)

				// Сохраняем позицию для коллизий
				petal.X = petalX
				petal.Y = petalY
			}
		}
	}
}

func (g *Game) checkPetalDrops() {
	g.mu.Lock()
	defer g.mu.Unlock()

	// Проверяем просроченные дропы
	expiredDrops := make([]string, 0)
	for id, drop := range g.petalDrops {
		if drop.IsExpired() {
			expiredDrops = append(expiredDrops, id)
		}
	}

	// Удаляем просроченные дропы
	for _, id := range expiredDrops {
		delete(g.petalDrops, id)
	}

	// Проверяем подбор дропов игроками
	for _, player := range g.players {
		for _, drop := range g.petalDrops {
			if drop.CanBePickedBy(player.ID) && player.IsAlive() {
				distance := player.DistanceTo(drop.X, drop.Y)
				if distance < 50 { // Радиус подбора
					g.pickUpPetal(player, drop)
					break
				}
			}
		}
	}
}

func (g *Game) pickUpPetal(player *Player, drop *PetalDrop) {
	// Добавляем лепесток игроку
	player.AddPetal(drop.Type)

	// Удаляем дроп
	delete(g.petalDrops, drop.ID)

	// Отправляем уведомление
	if conn, ok := g.connections[player.ID]; ok {
		conn.WriteJSON(map[string]interface{}{
			"type": "petal_picked_up",
			"data": map[string]interface{}{
				"type": drop.Type,
			},
		})
	}

	fmt.Printf("🎯 Player %s picked up %s petal\n", player.ID, drop.Type)
}

func (g *Game) checkPetalCollisions() {
	g.mu.Lock()
	defer g.mu.Unlock()

	// Проверяем коллизии лепестков с мобами
	for _, player := range g.players {
		activePetals := player.GetActivePetals()

		for _, petal := range activePetals {
			for _, mob := range g.mobs {
				if !mob.IsAlive() || mob.Zone != player.CurrentZone {
					continue
				}

				distance := math.Sqrt(math.Pow(petal.X-mob.X, 2) + math.Pow(petal.Y-mob.Y, 2))
				collisionDistance := 10.0 + mob.Radius // Радиус лепестка + моба

				if distance < collisionDistance {
					g.handlePetalMobCollision(petal, mob)
				}
			}
		}
	}
}

func (g *Game) handlePetalMobCollision(petal *Petal, mob *Mob) {
	// Лепесток атакует моба
	if petal.CanAttack() {
		mob.TakeDamage(petal.Damage)
		petal.LastAttack = time.Now()

		// Если моб умер, засчитываем килл игроку и создаем дроп
		if !mob.IsAlive() {
			// Находим владельца лепестка
			player := g.players[petal.OwnerID]
			if player != nil {
				// Создаем дроп лепестка для игрока
				var petalType PetalType
				switch mob.Type {
				case MobTypeWolf:
					petalType = PetalTypeWolf
				case MobTypeGoblin:
					petalType = PetalTypeGoblin
				case MobTypeOrc:
					petalType = PetalTypeOrc
				default:
					petalType = PetalTypeGoblin // fallback
				}

				g.createPetalDrop(petal.OwnerID, petalType, mob.X, mob.Y)

				// Отправляем уведомление об убийстве
				g.sendMobDeathNotification(player, mob)
			}

			// Также отправляем специальное уведомление о убийстве петалом
			if conn, ok := g.connections[petal.OwnerID]; ok {
				conn.WriteJSON(map[string]interface{}{
					"type": "mob_killed_by_petal",
					"data": map[string]interface{}{
						"mob_type":   mob.Type,
						"petal_type": petal.Type,
						"xp":         mob.MaxHealth / 2, // Та же формула опыта
					},
				})
			}
		}
	}

	// Моб атакует лепесток
	if mob.CanAttack() {
		petal.TakeDamage(mob.Damage)
		mob.MarkAttack()

		// Если лепесток уничтожен
		if !petal.IsActive {
			g.handlePetalDestroyed(petal)
		}
	}
}

func (g *Game) handlePetalDestroyed(petal *Petal) {
	// Запускаем таймер восстановления
	go func() {
		time.Sleep(2 * time.Second)
		g.mu.Lock()
		defer g.mu.Unlock()

		if player, ok := g.players[petal.OwnerID]; ok {
			if existingPetal, ok := player.Petals[petal.ID]; ok {
				existingPetal.Respawn()

				// Уведомляем игрока о восстановлении
				if conn, ok := g.connections[petal.OwnerID]; ok {
					conn.WriteJSON(map[string]interface{}{
						"type": "petal_respawned",
						"data": map[string]interface{}{
							"petal_id": petal.ID,
							"type":     petal.Type,
						},
					})
				}
			}
		}
	}()

	// Отправляем уведомление об уничтожении
	if conn, ok := g.connections[petal.OwnerID]; ok {
		conn.WriteJSON(map[string]interface{}{
			"type": "petal_destroyed",
			"data": map[string]interface{}{
				"petal_id": petal.ID,
				"type":     petal.Type,
			},
		})
	}
}

func (p *Player) GetPetalsForSerialization() map[string]*Petal {
	if p.Petals == nil {
		return make(map[string]*Petal)
	}

	// Создаем копию для безопасной сериализации
	petalsCopy := make(map[string]*Petal)
	for id, petal := range p.Petals {
		petalsCopy[id] = &Petal{
			ID:        petal.ID,
			Type:      petal.Type,
			Health:    petal.Health,
			MaxHealth: petal.MaxHealth,
			X:         petal.X,
			Y:         petal.Y,
			IsActive:  petal.IsActive,
			// Не копируем чувствительные или временные поля
		}
	}
	return petalsCopy
}

func (g *Game) checkPetalHealing() {
	g.mu.Lock()
	defer g.mu.Unlock()

	now := time.Now()

	for _, player := range g.players {
		for _, petal := range player.Petals {
			if petal.CanHeal() && player.Health < player.MaxHealth {
				// Исцеляем игрока
				player.Health += petal.HealAmount
				if player.Health > player.MaxHealth {
					player.Health = player.MaxHealth
				}

				petal.LastHeal = now

				// Отправляем уведомление об исцелении
				if conn, ok := g.connections[player.ID]; ok {
					conn.WriteJSON(map[string]interface{}{
						"type": "petal_healed",
						"data": map[string]interface{}{
							"petal_id": petal.ID,
							"amount":   petal.HealAmount,
							"health":   player.Health,
						},
					})
				}
			}
		}
	}
}
