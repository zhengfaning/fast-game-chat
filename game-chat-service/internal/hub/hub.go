package hub

import (
    "context"
    "log"
    "sync"
    
    "game-chat-service/internal/repository"
    "game-protocols/chat"
)

type Hub struct {
    users sync.Map // map[int32]*UserSession
    redis *repository.RedisClient
}

type UserSession struct {
    UserID int32
    GameID string
    // Here we would store the "Gateway Instance ID" if we need to push back via a specific Gateway
    // For Stage 2 local test, we assume Gateway connects via WS to GCS.
    // So GCS holds a "Client" connection which is actually the Gateway's connection?
    // Wait, the plan was: Gateway forwards payload to GCS.
    // If GCS is a WS server, then Gateway has a WS connection to GCS.
    // But Gateway maintains a POOL. Any connection in the pool can carry any user's message.
    // So GCS receives a message on *some* connection.
    // To PUSH message back to user, GCS needs to send it to the Gateway.
    // The Gateway needs to know which UserID.
    // So GCS sends {UserID, Payload} to Gateway.
    // Gateway reads it and routes to UserID.
    
    // In this "Reverse Proxy" mode:
    // GCS -> Gateway: "Please send this to User X".
    // Does the Gateway support this?
    // In Stage 1: Router.forwardToBackend just writes payload.
    // Stage 1: Router listens to readPump from Backend? No.
    // In Stage 1 router.go: forwardToBackend writes to conn.
    // BUT the Gateway Server has a readPump that reads from client.
    // Where is the loop reading from Backend?
    // Ah, Stage 1 implementation missed the "Read from Backend and Forward to Client" loop?
    // Let's check `game-gateway/internal/session/manager.go` or `server.go`.
    // Stage 1 `server.go` only has `readPump` (Client->Gateway) and `writePump` (Gateway->Client).
    // It does NOT have a loop reading from BackendConn.
    
    // CRITICAL FIX needed for Stage 1 or 2: Gateway needs to read from Backend Connection and forward to Client.
    // But Gateway uses a POOL of backend connections. 
    // If GCS writes to a random connection in the pool, how does Gateway know which Client to send to?
    // GCS must wrap the response in an Envelope with UserID (or SessionID).
    // Gateway reads from Backend, parses Envelope, finds Session, sends.
    
    // So for Stage 2, Hub doesn't hold "Connections". 
    // Hub holds "Which Gateway is User X on?" (via Redis).
    // To send message, GCS publishes to Redis or sends RPC to Gateway.
    // Since we are doing "Gateway connects to GCS via WS", GCS can write back to that WS.
    // But GCS needs to wrap it: "To: UserID, Payload: ..."
}

func NewHub(redis *repository.RedisClient) *Hub {
    return &Hub{
        redis: redis,
    }
}

func (h *Hub) HandleMessage(ctx context.Context, msg *chat.ChatRequest) {
    // Business logic
    // 1. Save DB
    // 2. Routing
    log.Printf("Received message from %d: %s", msg.Base.UserId, msg.Content)
}
