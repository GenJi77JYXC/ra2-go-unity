using System;

// Mirrors server/network/message.go. Field names must match the Go JSON
// tags exactly (case-sensitive) — JsonUtility has no attribute-based
// name mapping like Go's `json:"..."` tags.

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
}

// type is game.TerrainType as a raw int (Grass=0, Road=1, Water=2,
// Cliff=3, Ore=4) — must stay in the same order as the Go server's enum.
[Serializable]
public class TileData
{
    public int type;
    public bool passable;
}

[Serializable]
public class ClientCommand
{
    public string type;
    public int[] unitIds;
    public double targetX;    // move
    public double targetY;    // move
    public int targetUnitId;  // attack

    public string itemType;   // build: building type; produce: unit type
    public int cellX;         // build
    public int cellY;         // build
    public int buildingId;    // produce / cancel: which building
}
