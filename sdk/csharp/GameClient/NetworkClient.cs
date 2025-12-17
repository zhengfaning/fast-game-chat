using System;
using System.Net.WebSockets;
using System.Threading;
using System.Threading.Tasks;
using Google.Protobuf;
using System.Collections.Concurrent;
using System.Collections.Generic;
using Game.Protocols.Gateway;
using Game.Protocols.Common;

namespace GameClient
{
    public class NetworkClient
    {
        private ClientWebSocket _ws;
        private string _url;
        private CancellationTokenSource _cts;
        private BlockingCollection<Envelope> _sendQueue;
        
        // Router: RouteType -> Callback
        private Dictionary<Envelope.Types.RouteType, Action<Envelope>> _routes;

        public event Action<string> OnLog;
        public event Action OnConnected;
        public event Action OnDisconnected;

        public bool IsConnected => _ws != null && _ws.State == WebSocketState.Open;

        public NetworkClient(string url)
        {
            _url = url;
            _sendQueue = new BlockingCollection<Envelope>();
            _routes = new Dictionary<Envelope.Types.RouteType, Action<Envelope>>();
        }

        public void RegisterRoute(Envelope.Types.RouteType route, Action<Envelope> handler)
        {
            _routes[route] = handler;
        }

        public async Task ConnectAsync()
        {
            _ws = new ClientWebSocket();
            _cts = new CancellationTokenSource();

            try
            {
                await _ws.ConnectAsync(new Uri(_url), CancellationToken.None);
                OnLog?.Invoke("Connected to Gateway");
                OnConnected?.Invoke();
                
                // Start loops
                _ = ReceiveLoop();
                _ = SendLoop();
            }
            catch (Exception ex)
            {
                OnLog?.Invoke($"Connection failed: {ex.Message}");
            }
        }

        public void Send(Envelope envelope)
        {
            if (!IsConnected) return;
            _sendQueue.Add(envelope);
        }

        private async Task SendLoop()
        {
            foreach (var envelope in _sendQueue.GetConsumingEnumerable(_cts.Token))
            {
                try
                {
                    byte[] data = envelope.ToByteArray();
                    await _ws.SendAsync(new ArraySegment<byte>(data), WebSocketMessageType.Binary, true, _cts.Token);
                }
                catch (Exception ex)
                {
                    OnLog?.Invoke($"Send error: {ex.Message}");
                }
            }
        }

        private async Task ReceiveLoop()
        {
            var buffer = new byte[8192];
            try
            {
                while (_ws.State == WebSocketState.Open && !_cts.IsCancellationRequested)
                {
                    var result = await _ws.ReceiveAsync(new ArraySegment<byte>(buffer), _cts.Token);
                    if (result.MessageType == WebSocketMessageType.Close)
                    {
                        await _ws.CloseAsync(WebSocketCloseStatus.NormalClosure, string.Empty, CancellationToken.None);
                        OnDisconnected?.Invoke();
                        break;
                    }

                    if (result.Count > 0)
                    {
                        var data = new byte[result.Count];
                        Array.Copy(buffer, data, result.Count);
                        ProcessMessage(data);
                    }
                }
            }
            catch (Exception ex)
            {
                OnLog?.Invoke($"Receive error: {ex.Message}");
                OnDisconnected?.Invoke();
            }
        }

        private void ProcessMessage(byte[] data)
        {
            try
            {
                var envelope = Envelope.Parser.ParseFrom(data);
                if (_routes.TryGetValue(envelope.Route, out var handler))
                {
                    // Dispatch to main thread in Unity? 
                    // For pure C# SDK, just invoke. Unity users handle sync context.
                    handler(envelope);
                }
                else
                {
                    OnLog?.Invoke($"Unhandled route: {envelope.Route}");
                }
            }
            catch (Exception ex)
            {
                OnLog?.Invoke($"Parse error: {ex.Message}");
            }
        }
        
        public void Disconnect()
        {
            _cts?.Cancel();
            _ws?.Dispose();
            _ws = null;
        }
    }
}
