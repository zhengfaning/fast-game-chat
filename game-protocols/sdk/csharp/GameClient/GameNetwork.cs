using System;
using System.IO;
using System.Net.WebSockets;
using System.Threading;
using System.Threading.Tasks;
using System.Text;
using Google.Protobuf;

namespace GameClient.Network
{
    public enum RouteType : byte
    {
        Game = 1,
        Chat = 2,
        System = 3
    }

    public class Packet
    {
        public const uint MagicNumber = 0x12345678;
        public const int HeaderSize = 16;

        public RouteType Route { get; set; }
        public byte Flags { get; set; }
        public uint Sequence { get; set; }
        public byte[] Payload { get; set; }

        public byte[] Encode()
        {
            using (var ms = new MemoryStream())
            using (var writer = new BinaryWriter(ms))
            {
                WriteBigEndian(writer, MagicNumber);
                writer.Write((byte)Route);
                writer.Write(Flags);
                writer.Write((ushort)0); // Reserved
                WriteBigEndian(writer, (uint)Payload.Length);
                WriteBigEndian(writer, Sequence);
                writer.Write(Payload);
                
                return ms.ToArray();
            }
        }
        
        public static Packet DecodeHeader(byte[] header)
        {
            if (header.Length < HeaderSize) throw new Exception("Header too short");
            
            using (var ms = new MemoryStream(header))
            using (var reader = new BinaryReader(ms))
            {
                uint magic = ReadBigEndianUInt32(reader);
                if (magic != MagicNumber) throw new Exception($"Invalid magic: {magic:X}");
                
                var pkt = new Packet();
                pkt.Route = (RouteType)reader.ReadByte();
                pkt.Flags = reader.ReadByte();
                reader.ReadUInt16(); // Reserved Header bytes
                uint len = ReadBigEndianUInt32(reader);
                
                // Read Length but don't store it, Payload length is implicit in structure
                
                pkt.Sequence = ReadBigEndianUInt32(reader);
                
                return pkt;
            }
        }
        
        private static void WriteBigEndian(BinaryWriter w, uint val)
        {
            byte[] bytes = BitConverter.GetBytes(val);
            if (BitConverter.IsLittleEndian) Array.Reverse(bytes);
            w.Write(bytes);
        }
        
        private static uint ReadBigEndianUInt32(BinaryReader r)
        {
            byte[] bytes = r.ReadBytes(4);
            if (BitConverter.IsLittleEndian) Array.Reverse(bytes);
            return BitConverter.ToUInt32(bytes, 0);
        }
    }

    public class GameNetworkClient
    {
        private ClientWebSocket _ws;
        private uint _seq = 0;
        
        public event Action<Packet> OnPacketReceived;
        public event Action OnConnected;
        
        public async Task ConnectAsync(string url)
        {
            _ws = new ClientWebSocket();
            await _ws.ConnectAsync(new Uri(url), CancellationToken.None);
            OnConnected?.Invoke();
            _ = ReceiveLoop();
        }
        
        public async Task SendAsync(RouteType route, IMessage protoMessage)
        {
            var payload = protoMessage.ToByteArray();
            var pkt = new Packet 
            {
                Route = route,
                Flags = 0,
                Sequence = ++_seq,
                Payload = payload
            };
            
            var data = pkt.Encode();
            await _ws.SendAsync(new ArraySegment<byte>(data), WebSocketMessageType.Binary, true, CancellationToken.None);
        }
        
        private async Task ReceiveLoop()
        {
            var buffer = new byte[1024 * 1024]; // 1MB buffer
            while (_ws.State == WebSocketState.Open)
            {
                try 
                {
                    var result = await _ws.ReceiveAsync(new ArraySegment<byte>(buffer), CancellationToken.None);
                    if (result.MessageType == WebSocketMessageType.Close) break;
                    
                    byte[] data = new byte[result.Count];
                    Array.Copy(buffer, data, result.Count);
                    
                    if (data.Length < Packet.HeaderSize) continue;

                    var pkt = Packet.DecodeHeader(data);
                    
                    // Extract Payload
                    if (data.Length > Packet.HeaderSize)
                    {
                        pkt.Payload = new byte[data.Length - Packet.HeaderSize];
                        Array.Copy(data, Packet.HeaderSize, pkt.Payload, 0, pkt.Payload.Length);
                    }
                    else
                    {
                         pkt.Payload = new byte[0];
                    }
                    
                    OnPacketReceived?.Invoke(pkt);
                }
                catch (Exception ex)
                {
                    Console.WriteLine($"Receive error: {ex.Message}");
                    break;
                }
            }
        }
    }
}
