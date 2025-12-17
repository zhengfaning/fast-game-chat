package protocol

import (
	"fmt"
	"sync/atomic"
    "time"
	
	"github.com/gorilla/websocket"
)

// WSConn WebSocket 连接的协议包装器
type WSConn struct {
	conn      *websocket.Conn
	nextSeq   uint32 // 原子计数器，用于生成序列号
}

// NewWSConn 创建新的 WebSocket 协议连接
func NewWSConn(conn *websocket.Conn) *WSConn {
	return &WSConn{
		conn:    conn,
		nextSeq: 0,
	}
}

// ReadPacket 从 WebSocket 读取数据包
func (c *WSConn) ReadPacket() (*Packet, error) {
	// 从 WebSocket 读取二进制消息
	messageType, data, err := c.conn.ReadMessage()
	if err != nil {
		return nil, err
	}
	
	// 验证消息类型
	if messageType != websocket.BinaryMessage {
		return nil, fmt.Errorf("expected binary message, got type %d", messageType)
	}
	
	// 解码数据包
	return Decode(data)
}

// WritePacket 写入数据包到 WebSocket
func (c *WSConn) WritePacket(pkt *Packet) error {
	// 编码数据包
	data := pkt.Encode()
	
	// 通过 WebSocket 发送二进制消息
	return c.conn.WriteMessage(websocket.BinaryMessage, data)
}

// SendRequest 发送请求并自动生成序列号
// 返回使用的序列号（用于匹配响应）
func (c *WSConn) SendRequest(route RouteType, payload []byte) (uint32, error) {
	seq := c.NextSeq()
	pkt := NewPacketWithSeq(route, seq, payload)
	return seq, c.WritePacket(pkt)
}

// SendResponse 发送响应（使用接收到的序列号）
func (c *WSConn) SendResponse(route RouteType, seq uint32, payload []byte) error {
	pkt := NewPacketWithSeq(route, seq, payload)
	return c.WritePacket(pkt)
}

// NextSeq 获取下一个序列号（线程安全）
func (c *WSConn) NextSeq() uint32 {
	return atomic.AddUint32(&c.nextSeq, 1)
}

// Close 关闭连接
func (c *WSConn) Close() error {
	return c.conn.Close()
}

// SetReadLimit 设置读取限制
func (c *WSConn) SetReadLimit(limit int64) {
	c.conn.SetReadLimit(limit)
}

// SetReadDeadline 设置读取超时
func (c *WSConn) SetReadDeadline(t time.Time) error {
    return c.conn.SetReadDeadline(t)
}
