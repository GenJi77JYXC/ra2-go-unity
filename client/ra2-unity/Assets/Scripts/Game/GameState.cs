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
    public UnitSnapshot[] units;
}

[Serializable]
public class UnitSnapshot
{
    public int id;
    public double x;
    public double y;
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
    public double targetX;
    public double targetY;
}
