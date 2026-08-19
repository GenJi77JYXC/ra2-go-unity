using System;

// Mirrors server/network/message.go. Field names must match the Go JSON
// tags exactly (case-sensitive) — JsonUtility has no attribute-based
// name mapping like Go's `json:"..."` tags.

[Serializable]
public class GameState
{
    public long tick;
    public bool isInitial;
    public UnitSnapshot[] units;
}

[Serializable]
public class UnitSnapshot
{
    public int id;
    public double x;
    public double y;
}

[Serializable]
public class ClientCommand
{
    public string type;
    public int[] unitIds;
    public double targetX;
    public double targetY;
}
