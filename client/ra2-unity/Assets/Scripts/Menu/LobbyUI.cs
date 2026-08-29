using UnityEngine;

// The menu scene's only real content: connect, browse rooms, ready up.
// It holds no state of its own — LobbyController (on the persistent
// session object) owns all of it, and this just draws it and sends the
// player's clicks back.
public class LobbyUI : MonoBehaviour
{
    [SerializeField] private string defaultServerUrl = "ws://localhost:8080/ws";

    // Must match the names in server/game/maps.go.
    private static readonly string[] MapNames = { "crossroads", "ridge" };
    private static readonly string[] MapLabels = { "Crossroads", "Ridge" };

    // Must match the strings in server/game/victory.go.
    private static readonly string[] VictoryConditions = { "buildings", "conyard", "annihilation" };
    private static readonly string[] VictoryLabels =
    {
        "All structures",   // lose your last building
        "Construction Yard", // short game: the yard alone decides it
        "Annihilation",      // everything, units included
    };

    private LobbyController lobby;
    private string serverUrl;
    private int victoryIndex;
    private int mapIndex;

    // Whether the next room created plays against the computer. Local UI
    // state — the room itself is told once, at creation.
    private bool vsComputer;

    private void Awake()
    {
        serverUrl = defaultServerUrl;
    }

    // The session lives across scenes, so it can't be an Inspector
    // reference — it's looked up here instead.
    private LobbyController Lobby
    {
        get
        {
            if (lobby == null && NetworkSession.Instance != null)
            {
                lobby = NetworkSession.Instance.Lobby;
            }
            return lobby;
        }
    }

    private void OnGUI()
    {
        LobbyController controller = Lobby;
        if (controller == null)
        {
            GUI.Label(new Rect(20f, 20f, 400f, 20f), "No NetworkSession in the scene.");
            return;
        }

        var panel = new Rect(Screen.width / 2f - 200f, 60f, 400f, 340f);
        GUI.Box(panel, "RA2 LOBBY");

        if (!controller.IsConnected)
        {
            DrawConnect(controller, panel);
        }
        else if (controller.CurrentRoom == null)
        {
            DrawRoomList(controller, panel);
        }
        else
        {
            DrawRoom(controller, panel);
        }

        if (controller.LastError != "")
        {
            GUI.Label(new Rect(panel.x + 16f, panel.yMax - 34f, 368f, 20f), controller.LastError);
        }
    }

    private void DrawConnect(LobbyController controller, Rect panel)
    {
        GUI.Label(new Rect(panel.x + 16f, panel.y + 34f, 100f, 20f), "Server");
        serverUrl = GUI.TextField(new Rect(panel.x + 90f, panel.y + 34f, 290f, 22f), serverUrl);

        GUI.Label(new Rect(panel.x + 16f, panel.y + 66f, 100f, 20f), "Name");
        controller.PlayerName = GUI.TextField(
            new Rect(panel.x + 90f, panel.y + 66f, 290f, 22f), controller.PlayerName);

        GUI.enabled = !controller.Connecting;
        if (GUI.Button(new Rect(panel.x + 90f, panel.y + 102f, 290f, 28f),
                controller.Connecting ? "Connecting…" : "Connect"))
        {
            controller.Connect(serverUrl);
        }
        GUI.enabled = true;
    }

    private void DrawRoomList(LobbyController controller, Rect panel)
    {
        DrawLastOutcome(controller, panel);

        GUI.Label(new Rect(panel.x + 16f, panel.y + 30f, 200f, 20f), $"Signed in as {controller.PlayerName}");

        if (GUI.Button(new Rect(panel.x + 220f, panel.y + 28f, 80f, 24f), "Refresh"))
        {
            controller.Send(new ClientCommand { type = "listRooms" });
        }
        if (GUI.Button(new Rect(panel.x + 306f, panel.y + 28f, 78f, 24f), "Create"))
        {
            controller.Send(new ClientCommand
            {
                type = "createRoom",
                victory = VictoryConditions[victoryIndex],
                mapName = MapNames[mapIndex],
                vsAi = vsComputer,
            });
        }

        // Whoever creates the room picks the rule; clicking cycles it.
        GUI.Label(new Rect(panel.x + 16f, panel.y + 58f, 90f, 20f), "Defeat when");
        if (GUI.Button(new Rect(panel.x + 110f, panel.y + 56f, 186f, 22f), $"{VictoryLabels[victoryIndex]} destroyed"))
        {
            victoryIndex = (victoryIndex + 1) % VictoryConditions.Length;
        }

        // Both settings belong to the room and are fixed at creation, so
        // they sit together on the same row as the Create button above.
        vsComputer = GUI.Toggle(new Rect(panel.x + 304f, panel.y + 58f, 84f, 20f), vsComputer, " vs AI");

        // Map, like the rule, belongs to the room and is fixed at creation.
        GUI.Label(new Rect(panel.x + 16f, panel.y + 82f, 90f, 20f), "Map");
        if (GUI.Button(new Rect(panel.x + 110f, panel.y + 80f, 278f, 22f), MapLabels[mapIndex]))
        {
            mapIndex = (mapIndex + 1) % MapNames.Length;
        }

        RoomInfo[] rooms = controller.Rooms;
        if (rooms.Length == 0)
        {
            GUI.Label(new Rect(panel.x + 16f, panel.y + 112f, 360f, 20f), "No rooms yet — create one.");
        }

        for (int i = 0; i < rooms.Length; i++)
        {
            RoomInfo room = rooms[i];
            var row = new Rect(panel.x + 16f, panel.y + 110f + i * 30f, 368f, 26f);

            GUI.Label(new Rect(row.x, row.y + 3f, 260f, 20f),
                $"{room.name}  [{room.state}]  {PlayerCount(room)}/{Seats(room)}"
                + $"{(room.vsAi ? " vs AI" : "")}  · {MapLabel(room.mapName)}");

            // The server enforces this too; disabling the button just
            // avoids a pointless round trip. A room against the computer
            // has no seat to join — its second one is already taken.
            GUI.enabled = room.state == "waiting" && PlayerCount(room) < Seats(room);
            if (GUI.Button(new Rect(row.xMax - 70f, row.y, 70f, 24f), "Join"))
            {
                controller.Send(new ClientCommand { type = "joinRoom", roomId = room.id });
            }
            GUI.enabled = true;
        }
    }

    private void DrawRoom(LobbyController controller, Rect panel)
    {
        RoomInfo room = controller.CurrentRoom;

        GUI.Label(new Rect(panel.x + 16f, panel.y + 30f, 360f, 20f),
            $"{room.name}  —  you are player {room.yourPlayerId}");
        GUI.Label(new Rect(panel.x + 16f, panel.y + 48f, 360f, 20f),
            $"{MapLabel(room.mapName)}  ·  defeat when {VictoryLabel(room.victory)} destroyed");

        PlayerInfo[] players = room.players ?? new PlayerInfo[0];
        for (int i = 0; i < players.Length; i++)
        {
            GUI.Label(new Rect(panel.x + 16f, panel.y + 78f + i * 24f, 360f, 20f),
                $"{players[i].name}   {(players[i].ready ? "READY" : "waiting…")}");
        }

        string status;
        if (room.vsAi)
        {
            status = "The computer holds the other seat — ready up to start.";
        }
        else
        {
            status = players.Length < 2 ? "Waiting for another player…" : "Both here — ready up to start.";
        }
        GUI.Label(new Rect(panel.x + 16f, panel.y + 78f + players.Length * 24f + 8f, 360f, 20f), status);

        bool iAmReady = IsReady(room.yourPlayerId, players);
        if (GUI.Button(new Rect(panel.x + 16f, panel.yMax - 76f, 180f, 28f), iAmReady ? "Not ready" : "Ready"))
        {
            controller.Send(new ClientCommand { type = "setReady", ready = !iAmReady });
        }
        if (GUI.Button(new Rect(panel.x + 204f, panel.yMax - 76f, 180f, 28f), "Leave room"))
        {
            controller.Send(new ClientCommand { type = "leaveRoom" });
            controller.Send(new ClientCommand { type = "listRooms" });
        }
    }

    // Shown above the room list after returning from a finished match.
    private void DrawLastOutcome(LobbyController controller, Rect panel)
    {
        string outcome = controller.LastOutcome;
        if (outcome == "")
        {
            return;
        }

        string text = outcome switch
        {
            "win" => "★ You won the last match",
            "lose" => "You lost the last match",
            _ => "The last match was a draw",
        };
        GUI.Label(new Rect(panel.x + 16f, panel.yMax - 58f, 368f, 20f), text);
    }

    // Seats is how many humans a room holds: one when the computer has
    // taken the other, two otherwise.
    private static int Seats(RoomInfo room)
    {
        return room.vsAi ? 1 : 2;
    }

    private static string MapLabel(string name)
    {
        for (int i = 0; i < MapNames.Length; i++)
        {
            if (MapNames[i] == name)
            {
                return MapLabels[i];
            }
        }
        return name;
    }

    private static string VictoryLabel(string condition)
    {
        for (int i = 0; i < VictoryConditions.Length; i++)
        {
            if (VictoryConditions[i] == condition)
            {
                return VictoryLabels[i];
            }
        }
        return condition;
    }

    private static int PlayerCount(RoomInfo room)
    {
        return room.players == null ? 0 : room.players.Length;
    }

    private static bool IsReady(int playerId, PlayerInfo[] players)
    {
        foreach (PlayerInfo p in players)
        {
            if (p.id == playerId)
            {
                return p.ready;
            }
        }
        return false;
    }
}
