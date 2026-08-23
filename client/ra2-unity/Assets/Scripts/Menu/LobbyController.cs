using System;
using UnityEngine;
using UnityEngine.SceneManagement;

// Lobby state and the single place the server-message envelope is
// unwrapped. Lives on the persistent NetworkSession object, so it keeps
// running across the menu/game scene boundary.
//
// It draws nothing — LobbyUI (menu scene) renders it, GameManager (game
// scene) consumes the snapshots.
public class LobbyController : MonoBehaviour
{
    [SerializeField] private WebSocketClient socket;
    [SerializeField] private string menuScene = "Menu";
    [SerializeField] private string gameScene = "SampleScene";

    public string PlayerName { get; set; } = "Player";
    public RoomInfo[] Rooms { get; private set; } = new RoomInfo[0];
    public RoomInfo CurrentRoom { get; private set; }
    public string LastError { get; private set; } = "";

    // How the last match ended, kept so the menu can show it after the
    // scene change — "win", "lose", "draw", or "" before the first match.
    public string LastOutcome { get; private set; } = "";
    public bool Connecting { get; private set; }
    public bool IsConnected => socket.IsConnected;

    // Which Owner in the world this client controls, straight from the
    // server's seat assignment.
    public int MyPlayerId { get; private set; }

    // Raised when lobby state changes, so LobbyUI can repaint without
    // polling every field.
    public event Action OnLobbyChanged;

    private GameManager gameManager;

    // The server starts broadcasting the moment both players ready up, but
    // the game scene loads asynchronously — for a while there is nobody to
    // hand snapshots to. The newest one is worth keeping (it supersedes
    // whatever came before), and the initial one is worth keeping
    // separately because it is the only message that ever carries the map
    // and build menu: lose it and the match renders on an empty grid.
    private GameState pendingInitialState;
    private GameState pendingLatestState;

    private void OnEnable()
    {
        socket.OnMessage += HandleMessage;
        socket.OnConnected += HandleConnected;
        socket.OnDisconnected += HandleDisconnected;
    }

    private void OnDisable()
    {
        socket.OnMessage -= HandleMessage;
        socket.OnConnected -= HandleConnected;
        socket.OnDisconnected -= HandleDisconnected;
    }

    public void Connect(string url)
    {
        Connecting = true;
        LastError = "";
        socket.Connect(url);
    }

    public void Send(ClientCommand cmd)
    {
        cmd.playerName = PlayerName;
        socket.Send(cmd);
    }

    // Called by GameManager once the game scene is up. Anything that
    // arrived while it was loading gets replayed immediately, initial
    // first so the map exists before the units that stand on it.
    public void RegisterGameManager(GameManager manager)
    {
        gameManager = manager;

        if (pendingInitialState != null)
        {
            gameManager.HandleState(pendingInitialState);
            pendingInitialState = null;
        }
        if (pendingLatestState != null)
        {
            gameManager.HandleState(pendingLatestState);
            pendingLatestState = null;
        }
    }

    public void UnregisterGameManager(GameManager manager)
    {
        if (gameManager == manager)
        {
            gameManager = null;
        }
    }

    private void HandleConnected()
    {
        Connecting = false;
        LastError = "";
        Send(new ClientCommand { type = "listRooms" });
        OnLobbyChanged?.Invoke();
    }

    private void HandleDisconnected(string reason)
    {
        Connecting = false;
        CurrentRoom = null;
        MyPlayerId = 0;
        LastError = $"disconnected: {reason}";

        ReturnToMenu();
        OnLobbyChanged?.Invoke();
    }

    private void HandleMessage(string json)
    {
        ServerMessage message = JsonUtility.FromJson<ServerMessage>(json);

        switch (message.type)
        {
            case "rooms":
                Rooms = message.rooms ?? new RoomInfo[0];
                OnLobbyChanged?.Invoke();
                break;

            case "room":
                HandleRoomUpdate(message.room);
                break;

            case "state":
                HandleState(message.state);
                break;

            case "result":
                // Only record it. The "room" message that follows flips
                // the room to finished, and that is what drives the trip
                // back to the menu — clearing state here would make that
                // message look like it changed nothing.
                LastOutcome = message.result != null ? message.result.outcome : "";
                OnLobbyChanged?.Invoke();
                break;

            case "error":
                LastError = message.error;
                OnLobbyChanged?.Invoke();
                break;
        }
    }

    private void HandleState(GameState state)
    {
        if (state == null)
        {
            return;
        }

        if (gameManager != null)
        {
            gameManager.HandleState(state);
            return;
        }

        if (state.isInitial)
        {
            pendingInitialState = state;
        }
        else
        {
            pendingLatestState = state;
        }
    }

    private void HandleRoomUpdate(RoomInfo room)
    {
        bool wasPlaying = CurrentRoom != null && CurrentRoom.state == "playing";

        CurrentRoom = room;
        MyPlayerId = room.yourPlayerId;
        LastError = "";

        bool playing = room.state == "playing";
        if (playing && !wasPlaying)
        {
            LastOutcome = ""; // clear the previous match's banner
            LoadScene(gameScene);
        }
        else if (!playing && wasPlaying)
        {
            // The match ended, usually because the other player left.
            ReturnToMenu();
        }

        OnLobbyChanged?.Invoke();
    }

    private void ReturnToMenu()
    {
        pendingInitialState = null;
        pendingLatestState = null;
        gameManager = null;

        // Give up the seat so the finished room can be cleaned up. Skipped
        // when the socket is already gone, which is the other way we get
        // here.
        if (CurrentRoom != null && socket.IsConnected)
        {
            Send(new ClientCommand { type = "leaveRoom" });
            Send(new ClientCommand { type = "listRooms" });
        }
        CurrentRoom = null;

        if (SceneManager.GetActiveScene().name != menuScene)
        {
            LoadScene(menuScene);
        }
    }

    // Always async: a synchronous LoadScene blocks Unity's main thread for
    // the whole load, and ReceiveLoop's continuations are posted to that
    // thread — so the client would stop reading the socket entirely while
    // the server keeps pushing 20 snapshots a second at it. That backed up
    // far enough to trip the server's write timeout and drop the
    // connection mid-match. Loading across frames keeps the message pump
    // running.
    private void LoadScene(string sceneName)
    {
        SceneManager.LoadSceneAsync(sceneName);
    }
}
