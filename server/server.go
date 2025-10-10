package server

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"mpg/server/game"
	"mpg/server/user"
	"net/http"
	"time"

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

	// Оставить только API
	http.HandleFunc("/api/register", s.handleRegister)
	http.HandleFunc("/api/login", s.handleLogin)
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

func (s *Server) Close() error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return s.client.Disconnect(ctx)
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
			"type":    "error",
			"message": "Authentication required",
		})
		return
	}

	user, err := s.users.GetUserByID(token)
	if err != nil {
		ws.WriteJSON(map[string]interface{}{
			"type":    "error",
			"message": "Invalid or expired token",
		})
		return
	}

	username := user.Login // or user.Username, depending on your struct
	userID := token        // this is the MongoDB ID (hex string)

	// Создаем нового игрока, передавая соединение и userID
	player := s.game.AddPlayer(ws, userID, username)
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
		case "respawn": 
			s.game.RespawnPlayer(player.ID)
		case "ping":
			ws.WriteJSON(game.GameMessage{Type: "pong"})
		}
	}
}
