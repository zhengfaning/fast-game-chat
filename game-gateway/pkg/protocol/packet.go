package protocol

import (
	"encoding/binary"
	"fmt"
	"io"
)

// 协议设计 (增强版):
// +-------+-------+-------+----------+--------+----------+-----------+
// | Magic | Route | Flags | Reserved | Length | Sequence |  Payload  |
// |(4byte)|(1byte)|(1byte)| (2 byte) |(4 byte)| (4 byte) |  (变长)    |
// +-------+-------+-------+----------+--------+----------+-----------+
//
// Magic: 0x12345678 (魔数，用于校验)
// Route: 路由类型 (1=GAME, 2=CHAT, 3=SYSTEM)
// Flags: 标志位 (bit0=压缩, bit1=加密, bit2-7=保留)
// Reserved: 保留字段，用于未来扩展
// Length: Payload 长度（不包含头部）
// Sequence: 序列号（用于请求-响应匹配，客户端生成）
// Payload: Protobuf 编码的业务消息

const (
	MagicNumber uint32 = 0x12345678
	HeaderSize  int    = 16 // 4 + 1 + 1 + 2 + 4 + 4
	
	// 最大数据包大小 (16MB)
	MaxPacketSize uint32 = 16 * 1024 * 1024
)

// RouteType 路由类型
type RouteType byte

const (
	RouteUnknown RouteType = 0
	RouteGame    RouteType = 1
	RouteChat    RouteType = 2
	RouteSystem  RouteType = 3
)

// Flags 标志位
type Flags byte

const (
	FlagNone       Flags = 0
	FlagCompressed Flags = 1 << 0 // bit 0: 是否压缩
	FlagEncrypted  Flags = 1 << 1 // bit 1: 是否加密
	// bits 2-7: 保留
)

// HasFlag 检查是否包含特定标志
func (f Flags) HasFlag(flag Flags) bool {
	return f&flag != 0
}

// SetFlag 设置标志
func (f *Flags) SetFlag(flag Flags) {
	*f |= flag
}

// ClearFlag 清除标志
func (f *Flags) ClearFlag(flag Flags) {
	*f &^= flag
}

// Packet 网络数据包
type Packet struct {
	Route    RouteType
	Flags    Flags
	Sequence uint32  // 序列号，用于请求-响应匹配
	Payload  []byte  // Protobuf 编码的业务数据
}

// Encode 编码数据包为二进制
// 返回: [Magic(4)][Route(1)][Flags(1)][Reserved(2)][Length(4)][Seq(4)][Payload]
func (p *Packet) Encode() []byte {
	payloadLen := len(p.Payload)
	totalLen := HeaderSize + payloadLen
	
	buf := make([]byte, totalLen)
	
	// Magic (4 bytes)
	binary.BigEndian.PutUint32(buf[0:4], MagicNumber)
	
	// Route (1 byte)
	buf[4] = byte(p.Route)
	
	// Flags (1 byte)
	buf[5] = byte(p.Flags)
	
	// Reserved (2 bytes) - 保留为 0
	buf[6] = 0
	buf[7] = 0
	
	// Length (4 bytes)
	binary.BigEndian.PutUint32(buf[8:12], uint32(payloadLen))
	
	// Sequence (4 bytes)
	binary.BigEndian.PutUint32(buf[12:16], p.Sequence)
	
	// Payload
	copy(buf[16:], p.Payload)
	
	return buf
}

// DecodeHeader 只解码包头（16字节）
// 用于 Gateway 快速路由决策
func DecodeHeader(data []byte) (route RouteType, flags Flags, payloadLen uint32, seq uint32, err error) {
	if len(data) < HeaderSize {
		return 0, 0, 0, 0, fmt.Errorf("data too short: %d < %d", len(data), HeaderSize)
	}
	
	// 检查 Magic
	magic := binary.BigEndian.Uint32(data[0:4])
	if magic != MagicNumber {
		return 0, 0, 0, 0, fmt.Errorf("invalid magic: 0x%X", magic)
	}
	
	// 读取 Route
	route = RouteType(data[4])
	
	// 读取 Flags
	flags = Flags(data[5])
	
	// 跳过 Reserved (data[6:8])
	
	// 读取 Length
	payloadLen = binary.BigEndian.Uint32(data[8:12])
	
	// 读取 Sequence
	seq = binary.BigEndian.Uint32(data[12:16])
	
	// 安全检查
	if payloadLen > MaxPacketSize {
		return 0, 0, 0, 0, fmt.Errorf("payload too large: %d > %d", payloadLen, MaxPacketSize)
	}
	
	return route, flags, payloadLen, seq, nil
}

// Decode 完整解码数据包
func Decode(data []byte) (*Packet, error) {
	route, flags, payloadLen, seq, err := DecodeHeader(data)
	if err != nil {
		return nil, err
	}
	
	// 检查数据完整性
	expectedLen := HeaderSize + int(payloadLen)
	if len(data) < expectedLen {
		return nil, fmt.Errorf("incomplete packet: got %d, expected %d", len(data), expectedLen)
	}
	
	// 提取 Payload
	payload := make([]byte, payloadLen)
	copy(payload, data[16:16+payloadLen])
	
	return &Packet{
		Route:    route,
		Flags:    flags,
		Sequence: seq,
		Payload:  payload,
	}, nil
}

// ReadPacket 从 io.Reader 读取完整数据包
// 适用于 TCP 长连接
func ReadPacket(r io.Reader) (*Packet, error) {
	// 1. 读取固定头部
	header := make([]byte, HeaderSize)
	if _, err := io.ReadFull(r, header); err != nil {
		return nil, fmt.Errorf("read header: %w", err)
	}
	
	// 2. 解析头部
	route, flags, payloadLen, seq, err := DecodeHeader(header)
	if err != nil {
		return nil, err
	}
	
	// 3. 读取 Payload
	payload := make([]byte, payloadLen)
	if payloadLen > 0 {
		if _, err := io.ReadFull(r, payload); err != nil {
			return nil, fmt.Errorf("read payload: %w", err)
		}
	}
	
	return &Packet{
		Route:    route,
		Flags:    flags,
		Sequence: seq,
		Payload:  payload,
	}, nil
}

// WritePacket 写入完整数据包到 io.Writer
func WritePacket(w io.Writer, pkt *Packet) error {
	data := pkt.Encode()
	_, err := w.Write(data)
	return err
}

// NewPacket 创建新数据包
func NewPacket(route RouteType, payload []byte) *Packet {
	return &Packet{
		Route:    route,
		Flags:    FlagNone,
		Sequence: 0,
		Payload:  payload,
	}
}

// NewPacketWithSeq 创建带序列号的数据包
func NewPacketWithSeq(route RouteType, seq uint32, payload []byte) *Packet {
	return &Packet{
		Route:    route,
		Flags:    FlagNone,
		Sequence: seq,
		Payload:  payload,
	}
}

// IsValid 检查数据包是否有效
func (p *Packet) IsValid() bool {
	return p.Route >= RouteGame && p.Route <= RouteSystem
}

// String 返回数据包的字符串表示
func (p *Packet) String() string {
	return fmt.Sprintf("Packet{Route:%d, Flags:0x%02X, Seq:%d, PayloadLen:%d}",
		p.Route, p.Flags, p.Sequence, len(p.Payload))
}
