package main

import (
	"fmt"
	"math/rand"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

// Player представляет игрока
type Player struct {
	ID    string  `json:"id"`
	X     float64 `json:"x"`
	Y     float64 `json:"y"`
	Color string  `json:"color"`
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

type Game struct {
	players       map[string]*Player
	playersMu     sync.RWMutex
	worldSize     float64
	colors        []string
	connections   map[string]*websocket.Conn
	connectionsMu sync.RWMutex
}

var game *Game

func InitGame() {
	rand.Seed(time.Now().UnixNano())

	game = &Game{
		players:     make(map[string]*Player),
		worldSize:   1000.0,
		colors:      []string{"#FF6B6B", "#4ECDC4", "#45B7D1", "#96CEB4", "#FFEAA7", "#DDA0DD", "#98FB98", "#FFD700"},
		connections: make(map[string]*websocket.Conn),
	}

	// Запускаем периодическую синхронизацию состояния
	go game.synchronizeGameState()
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
			ID:    player.ID,
			X:     player.X,
			Y:     player.Y,
			Color: player.Color,
		}
	}
	g.playersMu.RUnlock()

	// Рассылаем состояние всем подключенным клиентам
	for playerID, conn := range g.connections {
		state := map[string]interface{}{
			"type":      "state",
			"players":   playersCopy,
			"yourId":    playerID,
			"worldSize": g.worldSize,
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

func (g *Game) AddPlayer(conn *websocket.Conn) *Player {
	g.playersMu.Lock()
	defer g.playersMu.Unlock()

	playerID := fmt.Sprintf("%d", time.Now().UnixNano()+rand.Int63())
	player := &Player{
		ID:    playerID,
		X:     rand.Float64() * g.worldSize,
		Y:     rand.Float64() * g.worldSize,
		Color: g.colors[rand.Intn(len(g.colors))],
	}

	g.players[playerID] = player

	// Сохраняем соединение
	g.connectionsMu.Lock()
	g.connections[playerID] = conn
	g.connectionsMu.Unlock()

	fmt.Printf("New player connected: %s at (%.1f, %.1f)\n", playerID, player.X, player.Y)

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
		// Обновляем позицию с проверкой границ
		newX := player.X + dx
		newY := player.Y + dy

		if newX >= 0 && newX <= g.worldSize {
			player.X = newX
		}
		if newY >= 0 && newY <= g.worldSize {
			player.Y = newY
		}

		fmt.Printf("Player %s moved to (%.1f, %.1f)\n", playerID, player.X, player.Y)
	}
}

func (g *Game) GetGameState(playerID string) map[string]interface{} {
	g.playersMu.RLock()
	defer g.playersMu.RUnlock()

	playersCopy := make(map[string]*Player)
	for id, player := range g.players {
		playersCopy[id] = &Player{
			ID:    player.ID,
			X:     player.X,
			Y:     player.Y,
			Color: player.Color,
		}
	}

	return map[string]interface{}{
		"type":      "state",
		"players":   playersCopy,
		"yourId":    playerID,
		"worldSize": g.worldSize,
	}
}

func (g *Game) GetPlayersCount() int {
	g.playersMu.RLock()
	defer g.playersMu.RUnlock()
	return len(g.players)
}

// В handleWebSocket изменим вызов AddPlayer:
func handleWebSocket(w http.ResponseWriter, r *http.Request) {
	ws, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		fmt.Println("WebSocket upgrade error:", err)
		return
	}
	defer ws.Close()

	// Создаем нового игрока, передавая соединение
	player := game.AddPlayer(ws)
	defer game.RemovePlayer(player.ID)

	// Отправляем начальное состояние
	initialState := game.GetGameState(player.ID)
	if err := ws.WriteJSON(initialState); err != nil {
		fmt.Println("Error sending initial state:", err)
		return
	}

	// Обрабатываем сообщения от клиента
	for {
		var msg GameMessage
		if err := ws.ReadJSON(&msg); err != nil {
			fmt.Printf("Player %s disconnected: %v\n", player.ID, err)
			break
		}

		switch msg.Type {
		case "move":
			if moveData, ok := msg.Data.(map[string]interface{}); ok {
				dx, _ := moveData["dx"].(float64)
				dy, _ := moveData["dy"].(float64)

				game.MovePlayer(player.ID, dx, dy)
				// Теперь состояние рассылается автоматически через synchronizeGameState
			}
		case "ping":
			// Ответ на пинг-сообщение
			ws.WriteJSON(GameMessage{Type: "pong"})
		}
	}
}
