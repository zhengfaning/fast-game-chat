package repository

import (
    "context"
    "fmt"
    
    "game-protocols/chat"
)

func (db *Database) SaveMessage(ctx context.Context, req *chat.ChatRequest) (int64, error) {
    // 简单实现：将 protobuf 序列化后存储（实际应该拆分字段）
    // 或者只存储元数据
    
    // 假设表结构：
    // CREATE TABLE messages (id BIGSERIAL PRIMARY KEY, game_id VARCHAR, sender_id INT, receiver_id INT, content TEXT, created_at TIMESTAMP);
    
    // 这里我们先模拟一下
    var id int64
    query := `
        INSERT INTO messages (game_id, sender_id, receiver_id, channel_id, content, created_at)
        VALUES ($1, $2, $3, $4, $5, NOW())
        RETURNING id
    `
    // assuming Base is always present
    gameID := req.Base.GameId
    senderID := req.Base.UserId
    
    err := db.Pool.QueryRow(ctx, query, 
        gameID, senderID, req.ReceiverId, req.ChannelId, req.Content,
    ).Scan(&id)

    if err != nil {
        return 0, fmt.Errorf("failed to insert message: %w", err)
    }

    return id, nil
}
