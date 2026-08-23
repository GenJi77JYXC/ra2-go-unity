using UnityEngine;

// The one object that outlives scene changes. The WebSocket connection has
// to survive the trip from the menu into a match — reconnecting at the
// scene boundary would drop the player's seat — so it lives here under
// DontDestroyOnLoad instead of in either scene.
//
// Inspector references can't cross scenes, so anything in the game scene
// reaches the connection through Instance rather than a serialized field.
// That's the standard cost of splitting scenes: wiring that used to be
// drag-and-drop becomes a runtime lookup.
public class NetworkSession : MonoBehaviour
{
    public static NetworkSession Instance { get; private set; }

    [SerializeField] private WebSocketClient socket;
    [SerializeField] private LobbyController lobby;

    public WebSocketClient Socket => socket;
    public LobbyController Lobby => lobby;

    private void Awake()
    {
        // Returning to the menu loads a scene that contains this object
        // again; the copy that has been alive all along wins, and the new
        // one removes itself before its own components start up.
        if (Instance != null && Instance != this)
        {
            Destroy(gameObject);
            return;
        }

        Instance = this;
        DontDestroyOnLoad(gameObject);
    }

    private void OnDestroy()
    {
        if (Instance == this)
        {
            Instance = null;
        }
    }
}
