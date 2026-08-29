using System;

// Mirrors server/network/message.go. Field names must match the Go JSON
// tags exactly (case-sensitive) — JsonUtility has no attribute-based
// name mapping like Go's `json:"..."` tags.

// Every server -> client message arrives in this envelope. Before Phase 6
// the socket only ever carried GameState, so it could be deserialized
// directly; now the same connection carries lobby traffic too. Payloads
// the server omitted come through as null.
[Serializable]
public class ServerMessage
{
    public string type; // "rooms", "room", "state", "result", "error"
    public RoomInfo[] rooms;
    public RoomInfo room;
    public GameState state;
    public MatchResult result;
    public string error;
}

// Sent once when a match is decided. outcome is already phrased from this
// client's side, so there's no need to compare winner against our own id.
[Serializable]
public class MatchResult
{
    public int winner; // 0 = draw
    public string outcome; // "win", "lose", "draw"
}

[Serializable]
public class RoomInfo
{
    public int id;
    public string name;
    public string state; // "waiting", "playing", "finished"
    public string victory; // "buildings", "conyard", "annihilation"

    public string mapName;

    // The other seat is the computer, so this room needs one human rather
    // than two and starts as soon as that one is ready.
    public bool vsAi;

    public PlayerInfo[] players;

    // Only set on a "room" message addressed to a member — this is how a
    // client learns which Owner in the world is theirs.
    public int yourPlayerId;
}

[Serializable]
public class PlayerInfo
{
    public int id;
    public string name;
    public bool ready;
}

[Serializable]
public class GameState
{
    public long tick;
    public bool isInitial;
    // Only populated on the one-off isInitial message a new connection
    // gets (server omits these from every regular per-tick update).
    public int mapWidth;
    public int mapHeight;
    public TileData[] tiles;
    public BuildOption[] buildMenu;

    // oreCells names every cell that can hold ore and arrives once, with
    // the map. ore carries the amounts in that same order and arrives
    // every frame — pairing them by index is what keeps a live ore field
    // affordable to broadcast.
    public OreCellData[] oreCells;
    public int[] ore;

    public UnitSnapshot[] units;
    public BuildingSnapshot[] buildings;
    public int money;
    public int power;

    // The structure being built right now. RA2 builds first and places
    // second, so this has no map position — it drives the build menu's
    // progress readout and its "ready to place" state. Empty type means
    // nothing is being built.
    public string pendingType;
    public float pendingProgress;
    public bool pendingReady;

    public QueueSnapshot[] queues;
}

// One production category's status. Categories are keyed by the building
// type that produces them and shared by every factory of that type, so
// two Barracks feed one queue rather than running in parallel.
[Serializable]
public class QueueSnapshot
{
    public string category;
    public string item;
    public float progress; // 0..1
    public int length;
}

// One entry in the build menu, sent once on join.
[Serializable]
public class BuildOption
{
    public string type;
    public int cost;
    public int width;
    public int height;
    public int power;
    public string[] produces;
    public string[] prerequisites;
}

[Serializable]
public class BuildingSnapshot
{
    public int id;
    public string type;
    public int owner;
    public int cellX;
    public int cellY;
    public int hp;
    public int maxHp;
    public bool isBuilt;

    // Marks the factory finished units of this category walk out of.
    public bool isPrimary;
}

[Serializable]
public class UnitSnapshot
{
    public int id;
    public double x;
    public double y;
    public int owner; // Phase 4: which player controls this unit
    public int hp;
    public int maxHp;

    // Unit template name ("Tank", "Infantry", "Harvester"). Drives how the
    // unit is drawn — until harvesters existed everything looked the same
    // and the client never needed to know.
    public string type;
}

// TerrainType mirrors the Go server's enum and must stay in the same
// order — it crosses the wire as a raw int.
public enum TerrainType
{
    Grass = 0,
    Road = 1,
    Water = 2,
    Cliff = 3,
    Ore = 4,
    OreDrill = 5,
}

[Serializable]
public class TileData
{
    public int type;
    public bool passable;
}

// OreCellData is one ore-field cell's position, sent once alongside the
// map. Its amount is GameState.ore at the same index.
[Serializable]
public class OreCellData
{
    public int x;
    public int y;
}

// Carries both lobby traffic ("listRooms", "createRoom", "joinRoom",
// "leaveRoom", "setReady") and in-game orders, kept in one class because
// JsonUtility has to commit to a single type per ToJson call.
[Serializable]
public class ClientCommand
{
    public string type;

    // lobby
    public int roomId;
    public string victory; // createRoom only
    public string mapName; // createRoom only
    public bool vsAi; // createRoom only
    public string playerName;
    public bool ready;

    public int[] unitIds;
    public double targetX;    // move
    public double targetY;    // move
    public int targetUnitId;  // attack

    public string itemType;   // build: building type; produce: unit type
    public int cellX;         // build
    public int cellY;         // build
    public int buildingId;    // produce / cancel: which building
}
