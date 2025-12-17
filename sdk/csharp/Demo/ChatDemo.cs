using UnityEngine;
using GameClient;
using GameClient.Modules;
using Game.Protocols.Chat;
using System;

public class ChatDemo : MonoBehaviour
{
    private NetworkClient _client;
    private ChatManager _chat;

    // UI References
    // public InputField messageInput;
    // public Text chatLog;

    async void Start()
    {
        // 1. Init Client
        _client = new NetworkClient("ws://localhost:8080/ws");
        _chat = new ChatManager(_client);

        // 2. Subscribe Events
        _client.OnConnected += () => Debug.Log("Connected!");
        _client.OnLog += (msg) => Debug.Log($"[Net] {msg}");
        
        _chat.OnMessageReceived += HandleMessage;

        // 3. Connect
        await _client.ConnectAsync();
        
        // 4. Send Hello
        _chat.SendText(1002, "Hello from Unity!");
    }

    private void HandleMessage(ChatResponse resp)
    {
        // Dispatch to UI thread if needed (Unity usually requires this)
        // NetworkClient callback might be on thread pool.
        UnityMainThreadDispatcher.Instance().Enqueue(() => {
            Debug.Log($"Received: {resp.Success} - {resp.MessageId}");
            // chatLog.text += ...
        });
    }

    void OnDestroy()
    {
        _client?.Disconnect();
    }
    
    // Test helper
    public void SendChatMessage(string text)
    {
        _chat.SendText(1002, text);
    }
}

// Simple Dispatcher stub
public class UnityMainThreadDispatcher : MonoBehaviour {
    public static UnityMainThreadDispatcher Instance() => null; // Stub
    public void Enqueue(Action action) { action(); }
}
