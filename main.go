package main

import (
	"fmt"
	"log"
	"net/http"
	"sync"

	"github.com/gorilla/websocket"
)

type GameHub struct {
	Games map[string]*GameRoom
	Mutex sync.Mutex
}

type GameRoom struct {
	GameID    string
	Players   map[*websocket.Conn]string
	Broadcast chan []byte
	Mutex     sync.Mutex
}

var hub = GameHub{
	Games: make(map[string]*GameRoom),
}

// Create a new game room
func (h *GameHub) CreateGame(gameID string) *GameRoom {
	h.Mutex.Lock()
	defer h.Mutex.Unlock()

	// If game already exists, return it
	if room, exists := h.Games[gameID]; exists {
		return room
	}

	room := &GameRoom{
		GameID:    gameID,
		Players:   make(map[*websocket.Conn]string),
		Broadcast: make(chan []byte),
	}

	h.Games[gameID] = room

	go room.ListenForMessages()

	return room
}

// Listen for messages and broadcast them
func (r *GameRoom) ListenForMessages() {
	for msg := range r.Broadcast {
		r.Mutex.Lock()
		for conn := range r.Players {
			err := conn.WriteMessage(websocket.TextMessage, msg)
			if err != nil {
				fmt.Println("Error sending message:", err)
				conn.Close()
				delete(r.Players, conn)
			}
		}
		r.Mutex.Unlock()
	}
}

// Add a player to a game
func (h *GameHub) AddPlayer(gameID string, conn *websocket.Conn, playerName string) {
	h.Mutex.Lock()
	defer h.Mutex.Unlock()

	room, exists := h.Games[gameID]
	if !exists {
		fmt.Println("Game not found")
		return
	}

	// Add player to room
	room.Mutex.Lock()
	room.Players[conn] = playerName
	room.Mutex.Unlock()

	fmt.Printf("Player %s joined game %s\n", playerName, gameID)

	// Notify all players
	message := []byte(fmt.Sprintf("%s joined the game!", playerName))
	room.Broadcast <- message

	// Start listening for player messages
	go handlePlayerMessages(conn, room)
}

// Handle player messages
func handlePlayerMessages(conn *websocket.Conn, room *GameRoom) {
	defer func() {
		room.Mutex.Lock()
		delete(room.Players, conn)
		room.Mutex.Unlock()
		conn.Close()
	}()

	for {
		_, msg, err := conn.ReadMessage()
		if err != nil {
			fmt.Println("Player disconnected:", err)
			break
		}
		room.Broadcast <- msg // Send received message to all players
	}
}

func handler(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintln(w, "Welcome to Monopoly Backend!")
}

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true // Allow all origins
	},
}

// WebSocket connection for starting a game
func StartGameWS(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		http.Error(w, "Could not open WebSocket", http.StatusBadRequest)
		return
	}

	// Get game ID from query params
	gameID := r.URL.Query().Get("gameId")
	playerName := r.URL.Query().Get("playerName")

	if gameID == "" || playerName == "" {
		conn.WriteMessage(websocket.TextMessage, []byte("Missing gameId or playerName"))
		conn.Close()
		return
	}

	hub.CreateGame(gameID) // Create or retrieve game
	hub.AddPlayer(gameID, conn, playerName)

	fmt.Println("Game started:", gameID)
}

// WebSocket connection for joining an existing game
func JoinGameWS(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		http.Error(w, "Could not open WebSocket", http.StatusBadRequest)
		return
	}

	gameID := r.URL.Query().Get("gameId")
	playerName := r.URL.Query().Get("playerName")

	if gameID == "" || playerName == "" {
		conn.WriteMessage(websocket.TextMessage, []byte("Missing gameId or playerName"))
		conn.Close()
		return
	}

	hub.AddPlayer(gameID, conn, playerName)
}

func main() {
	http.HandleFunc("/", handler)
	http.HandleFunc("/ws/start", StartGameWS)
	http.HandleFunc("/ws/join", JoinGameWS)

	port := "8080"
	fmt.Println("Server running on port:", port)
	log.Fatal(http.ListenAndServe(":"+port, nil))
}
