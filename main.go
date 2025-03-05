package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sync"

	"github.com/gorilla/websocket"
)

type GameEvent struct {
	Event   string      `json:"event"`
	GameID  string      `json:"gameId"`
	Payload interface{} `json:"payload"`
}

type Player struct {
	Name       string   `json:"name"`
	Balance    int      `json:"balance"`
	Position   int      `json:"position"`
	Properties []string `json:"properties"`
	JailTurns  int      `json:"jailTurns"`
}

type GameState struct {
	Players map[string]*Player
	Turn    string
}

type GameRoom struct {
	ID        string
	Players   map[*websocket.Conn]string
	GameState GameState
	Mutex     sync.RWMutex
}

type GameHub struct {
	Rooms map[string]*GameRoom
	Mutex sync.RWMutex
}

var hub = GameHub{Rooms: make(map[string]*GameRoom)}

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

func handleWebSocket(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		fmt.Println("WebSocket upgrade failed:", err)
		return
	}
	roomID := r.URL.Query().Get("gameId")
	playerName := r.URL.Query().Get("name")
	if roomID == "" || playerName == "" {
		conn.Close()
		return
	}

	hub.Mutex.Lock()
	room, exists := hub.Rooms[roomID]
	if !exists {
		room = &GameRoom{
			ID:      roomID,
			Players: make(map[*websocket.Conn]string),
			GameState: GameState{
				Players: make(map[string]*Player),
			},
		}
		hub.Rooms[roomID] = room
	}
	room.Players[conn] = playerName
	room.GameState.Players[playerName] = &Player{Name: playerName, Balance: 1500, Position: 0}
	hub.Mutex.Unlock()

	fmt.Println("Player joined:", playerName)

	defer func() {
		hub.Mutex.Lock()
		delete(room.Players, conn)
		hub.Mutex.Unlock()
		conn.Close()
		fmt.Println("Player disconnected:", playerName)
	}()

	for {
		_, msg, err := conn.ReadMessage()
		if err != nil {
			fmt.Println("Read error:", err)
			break
		}
		var event GameEvent
		if err := json.Unmarshal(msg, &event); err != nil {
			fmt.Println("Invalid JSON format:", err)
			continue
		}
		handleGameEvent(room, event, conn)
	}
}

func handleGameEvent(room *GameRoom, event GameEvent, conn *websocket.Conn) {
	room.Mutex.Lock()
	defer room.Mutex.Unlock()

	switch event.Event {
	case "ROLL_DICE":
		HandleRollDiceEvent(room, event)
	case "BUY_PROPERTY":
		HandleBuyPropertyEvent(room, event)
	case "END_TURN":
		HandleEndTurnEvent(room, event)
	default:
		fmt.Println("Unknown event:", event.Event)
	}
}

func HandleRollDiceEvent(room *GameRoom, event GameEvent) {
	payload, _ := event.Payload.(map[string]interface{})
	playerName := payload["player"].(string)
	roll := int(payload["diceRoll"].(float64))

	room.GameState.Players[playerName].Position += roll
	SendGameEventToAll(room, "ROLL_DICE", event.GameID, payload)
}

func HandleBuyPropertyEvent(room *GameRoom, event GameEvent) {
	payload, _ := event.Payload.(map[string]interface{})
	playerName := payload["player"].(string)
	propertyName := payload["property"].(string)

	player := room.GameState.Players[playerName]
	player.Properties = append(player.Properties, propertyName)
	SendGameEventToAll(room, "BUY_PROPERTY", event.GameID, payload)
}

func HandleEndTurnEvent(room *GameRoom, event GameEvent) {
	// Rotate turn among players
	for name := range room.GameState.Players {
		if name != room.GameState.Turn {
			room.GameState.Turn = name
			break
		}
	}
	SendGameEventToAll(room, "END_TURN", event.GameID, map[string]string{"nextTurn": room.GameState.Turn})
}

func SendGameEventToAll(room *GameRoom, eventType string, gameID string, payload interface{}) {
	message, _ := json.Marshal(GameEvent{Event: eventType, GameID: gameID, Payload: payload})
	room.Mutex.RLock()
	defer room.Mutex.RUnlock()
	for conn := range room.Players {
		if err := conn.WriteMessage(websocket.TextMessage, message); err != nil {
			fmt.Println("Error sending message:", err)
		}
	}
}

func main() {
	http.HandleFunc("/ws", handleWebSocket)
	fmt.Println("WebSocket server started on ws://localhost:8080/ws")
	http.ListenAndServe(":8080", nil)
}
