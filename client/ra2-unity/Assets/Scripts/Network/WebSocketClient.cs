using System;
using System.IO;
using System.Net.WebSockets;
using System.Text;
using System.Threading;
using System.Threading.Tasks;
using UnityEngine;

// Owns the connection only — it knows nothing about GameState/GameManager.
// Raw JSON strings go out through OnMessage so the network layer stays
// decoupled from the game layer (per the learning plan's Phase 1 design).
public class WebSocketClient : MonoBehaviour
{
    [SerializeField] private string url = "ws://localhost:8080/ws";

    public event Action OnConnected;
    public event Action<string> OnDisconnected;
    public event Action<string> OnMessage;

    private ClientWebSocket socket;
    private CancellationTokenSource cts;

    private async void Start()
    {
        cts = new CancellationTokenSource();
        socket = new ClientWebSocket();

        try
        {
            await socket.ConnectAsync(new Uri(url), cts.Token);
        }
        catch (Exception e)
        {
            Debug.LogError($"[WebSocketClient] connect to {url} failed: {e.Message}");
            return;
        }

        Debug.Log($"[WebSocketClient] connected to {url}");
        OnConnected?.Invoke();

        await ReceiveLoop(cts.Token);
    }

    // Unity installs a SynchronizationContext on the main thread, so these
    // awaits post their continuations back to it automatically — no manual
    // main-thread dispatch needed to fire OnMessage safely.
    private async Task ReceiveLoop(CancellationToken token)
    {
        var buffer = new byte[8192];

        while (socket.State == WebSocketState.Open && !token.IsCancellationRequested)
        {
            using var ms = new MemoryStream();
            WebSocketReceiveResult result;

            try
            {
                // A message can arrive split across multiple frames
                // (EndOfMessage == false) once it doesn't fit in one
                // buffer — Phase 1's tiny GameState never does, but
                // Phase 3's full map snapshot will, so handle it now.
                do
                {
                    result = await socket.ReceiveAsync(new ArraySegment<byte>(buffer), token);

                    if (result.MessageType == WebSocketMessageType.Close)
                    {
                        await socket.CloseAsync(WebSocketCloseStatus.NormalClosure, "", token);
                        OnDisconnected?.Invoke("server closed the connection");
                        return;
                    }

                    ms.Write(buffer, 0, result.Count);
                } while (!result.EndOfMessage);
            }
            catch (Exception e)
            {
                OnDisconnected?.Invoke(e.Message);
                return;
            }

            OnMessage?.Invoke(Encoding.UTF8.GetString(ms.ToArray()));
        }
    }

    private void OnDestroy()
    {
        cts?.Cancel();
        socket?.Dispose();
    }
}
