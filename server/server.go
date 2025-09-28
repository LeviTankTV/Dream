package server

import (
	"context"
	"fmt"
	"net/http"
	"log"
	"time"
	"encoding/json"

	"mpg/server/game"
	"mpg/server/user"

	"github.com/gorilla/websocket"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type Server struct {
	addr   string
	game   *game.Game
	client *mongo.Client
	users  *user.Repository
}

type AuthRequest struct {
	Login    string `json:"login"`
	Password string `json:"password"`
}

type AuthResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
	UserID  string `json:"user_id,omitempty"`
}

func NewServer(addr string) *Server {
	// Подключаемся к MongoDB
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	client, err := mongo.Connect(ctx, options.Client().ApplyURI("mongodb://localhost:27017"))
	if err != nil {
		log.Fatal("Failed to connect to MongoDB:", err)
	}

	// Проверяем подключение
	err = client.Ping(ctx, nil)
	if err != nil {
		log.Fatal("Failed to ping MongoDB:", err)
	}

	fmt.Println("Connected to MongoDB!")

	db := client.Database("mpg")
	userRepo := user.NewRepository(db)

	return &Server{
		addr:   addr,
		game:   game.NewGame(),
		client: client,
		users:  userRepo,
	}
}

func (s *Server) Start() error {
	// Статические файлы
	http.HandleFunc("/", s.serveHome)
	http.HandleFunc("/style.css", s.serveCSS)
	http.HandleFunc("/index.js", s.serveJS)

	// API endpoints
	http.HandleFunc("/api/register", s.handleRegister)
	http.HandleFunc("/api/login", s.handleLogin)

	// WebSocket endpoint
	http.HandleFunc("/ws", s.handleWebSocket)

	return http.ListenAndServe(s.addr, nil)
}

func (s *Server) handleRegister(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req AuthRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.Login == "" || req.Password == "" {
		http.Error(w, "Login and password are required", http.StatusBadRequest)
		return
	}

	_, err := s.users.CreateUser(req.Login, req.Password)
	if err != nil {
		response := AuthResponse{
			Success: false,
			Message: "Failed to create user: " + err.Error(),
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
		return
	}

	response := AuthResponse{
		Success: true,
		Message: "User created successfully",
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req AuthRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	user, err := s.users.ValidateCredentials(req.Login, req.Password)
	if err != nil {
		response := AuthResponse{
			Success: false,
			Message: "Invalid credentials",
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
		return
	}

	response := AuthResponse{
		Success: true,
		Message: "Login successful",
		UserID:  user.ID.Hex(),
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func (s *Server) serveHome(w http.ResponseWriter, r *http.Request) {
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

func (s *Server) serveCSS(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/css")
	http.ServeFile(w, r, "client/style.css")
}

func (s *Server) serveJS(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/javascript")
	http.ServeFile(w, r, "client/index.js")
}

func (s *Server) handleWebSocket(w http.ResponseWriter, r *http.Request) {
    upgrader := websocket.Upgrader{
        CheckOrigin: func(r *http.Request) bool {
            return true // Для разработки разрешаем все origin
        },
    }

    ws, err := upgrader.Upgrade(w, r, nil)
    if err != nil {
        fmt.Println("WebSocket upgrade error:", err)
        return
    }
    defer ws.Close()

    // Проверяем аутентификацию
    token := r.URL.Query().Get("token")
    if token == "" {
        ws.WriteJSON(map[string]interface{}{
            "type": "error", 
            "message": "Authentication required",
        })
        return
    }

    // Здесь можно добавить проверку токена в базе данных
    // Пока просто используем токен как ID пользователя
    userID := token

    // Создаем нового игрока, передавая соединение и userID
    player := s.game.AddPlayer(ws, userID)
    defer s.game.RemovePlayer(player.ID)

    // Отправляем начальное состояние
    initialState := s.game.GetGameState(player.ID)
    if err := ws.WriteJSON(initialState); err != nil {
        fmt.Println("Error sending initial state:", err)
        return
    }

    // Обрабатываем сообщения от клиента
    for {
        var msg game.GameMessage
        if err := ws.ReadJSON(&msg); err != nil {
            fmt.Printf("Player %s disconnected: %v\n", player.ID, err)
            break
        }

        switch msg.Type {
        case "move":
            if moveData, ok := msg.Data.(map[string]interface{}); ok {
                dx, _ := moveData["dx"].(float64)
                dy, _ := moveData["dy"].(float64)

                s.game.MovePlayer(player.ID, dx, dy)
            }
        case "ping":
            ws.WriteJSON(game.GameMessage{Type: "pong"})
        }
    }
}