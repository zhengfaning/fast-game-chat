using System;
using Google.Protobuf;
using Game.Protocols.Chat;
using Game.Protocols.Common;
using Game.Protocols.Gateway;

namespace GameClient.Modules
{
    public class ChatManager
    {
        private NetworkClient _client;
        
        public event Action<ChatResponse> OnMessageReceived;
        
        public ChatManager(NetworkClient client)
        {
            _client = client;
            _client.RegisterRoute(Envelope.Types.RouteType.Chat, HandleChatEnvelope);
        }

        public void SendText(int receiverId, string content)
        {
            var req = new ChatRequest
            {
                Base = new MessageBase 
                { 
                    GameId = "mmo", // Configurable
                    Timestamp = DateTimeOffset.UtcNow.ToUnixTimeSeconds()
                },
                ReceiverId = receiverId,
                Content = content,
                Type = ChatRequest.Types.MessageType.Text
            };

            var envelope = new Envelope
            {
                Route = Envelope.Types.RouteType.Chat,
                GameId = "mmo",
                Payload = ByteString.CopyFrom(req.ToByteArray())
            };
            
            _client.Send(envelope);
        }

        private void HandleChatEnvelope(Envelope envelope)
        {
            // The payload is a ChatResponse (wrapped by GCS logic we added) -- wait.
            // Our GCS Transport logic:
            // "Payload: respBytes" where respBytes = proto.Marshal(resp) -> ChatResponse.
            // But verify: `resp` is `ChatResponse`.
            // Yes.
            
            // Wait, GCS transport sends `respEnvelope` where payload is `respBytes`.
            // `respBytes` is `ChatResponse`.
            // BUT: `NetworkClient` parses `Envelope`.
            // This handler receives `Envelope`.
            // So we unmarshal payload to `ChatResponse`.
            
            try 
            {
                // In C# Protobuf, we need a wrapper or parse from bytes.
                // ChatResponse doesn't have a parser in the generated code? It should.
                // Wait, response could be ChatResponse or MessageBroadcast?
                // Currently GCS only returns ChatResponse for the specific request.
                // We haven't implemented Broadcast yet.
                // Let's assume ChatResponse for now.
                
                // Note: The logic in GCS sends ChatResponse.
                
                // However, user A sending to user B.
                // A gets ChatResponse (ack).
                // B gets... MessageBroadcast?
                // We haven't implemented Broadcast logic in GCS Hub yet!
                // We only implemented Request->Response.
                // So verification will be A->A.
                
                var resp = ChatResponse.Parser.ParseFrom(envelope.Payload);
                OnMessageReceived?.Invoke(resp);
            }
            catch (Exception ex)
            {
                Console.WriteLine($"Chat parse error: {ex.Message}");
            }
        }
    }
}
