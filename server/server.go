package main

import (
	"fmt"
	"net/http"
	"os"

	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		allowedOrigins := []string{
			"http://localhost:3000",
			"http://localhost:8080",
			"http://192.168.1.46:8080",
		}
		origin := r.Header.Get("Origin")
		for _, allowed := range allowedOrigins {
			if origin == allowed {
				return true
			}
		}
		return false
	},
}

func serveHome(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.Error(w, "Not found", http.StatusNotFound)
		return
	}
	if r.Method != "GET" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	http.ServeFile(w, r, "client/index.html")
}

func serveCSS(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/css")
	http.ServeFile(w, r, "client/style.css")
}

func serveJS(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/javascript")
	http.ServeFile(w, r, "client/index.js")
}

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

func main() {
	// Инициализируем игровой мир
	InitGame()

	// Статические файлы
	http.HandleFunc("/", serveHome)
	http.HandleFunc("/style.css", serveCSS)
	http.HandleFunc("/index.js", serveJS)

	// WebSocket endpoint
	http.HandleFunc("/ws", handleWebSocket)

	fmt.Println("Game server started on :8080")
	fmt.Println("Visit http://localhost:8080 in your browser")

	err := http.ListenAndServe(":8080", nil)
	if err != nil {
		fmt.Println("Error starting server:", err)
		os.Exit(1)
	}
}
