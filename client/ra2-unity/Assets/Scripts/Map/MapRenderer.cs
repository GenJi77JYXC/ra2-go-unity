using UnityEngine;
using UnityEngine.Tilemaps;

// Phase 3: lays out the tile grid from the server's one-off isInitial
// GameState (see GameManager.HandleMessage). Attach directly to the
// Tilemap GameObject (Grid > Tilemap, Cell Layout = Isometric).
//
// The ore field is the one part of the map that changes while playing, so
// it's redrawn from every frame's amounts rather than laid down once —
// see UpdateOre.
[RequireComponent(typeof(Tilemap))]
public class MapRenderer : MonoBehaviour
{
    // Indexed by TerrainType — Grass, Road, Water, Cliff, Ore, OreDrill,
    // in that exact order. Assign 6 Tile assets in the Inspector.
    [SerializeField] private TileBase[] tileAssets;

    // A worked-out ore cell fades toward this instead of vanishing, so a
    // thinning field reads as thinning rather than as a sudden hole.
    private static readonly Color SparseOre = new(0.55f, 0.5f, 0.35f);

    private Tilemap tilemap;

    // Passability is kept around after rendering so the build preview can
    // tint itself without another round trip to the server (which still
    // makes the real call — see BuildPlacementHandler).
    private bool[] passable;
    private int width;
    private int height;

    // Ore field bookkeeping. lastAmounts exists purely so a frame where
    // nothing was mined costs nothing to draw — most frames don't change
    // a single cell.
    private OreCellData[] oreCells;
    private int[] lastAmounts;
    private int oreCapacity = 1;

    private void Awake()
    {
        tilemap = GetComponent<Tilemap>();
    }

    public void Render(GameState state)
    {
        width = state.mapWidth;
        height = state.mapHeight;
        passable = new bool[width * height];

        for (int y = 0; y < height; y++)
        {
            for (int x = 0; x < width; x++)
            {
                TileData tile = state.tiles[y * width + x];
                tilemap.SetTile(new Vector3Int(x, y, 0), tileAssets[tile.type]);
                passable[y * width + x] = tile.passable;
            }
        }

        oreCells = state.oreCells;
        lastAmounts = null;

        // Capacity isn't on the wire, and every cell starts full, so the
        // richest cell in the opening frame is it. Deriving it beats
        // hardcoding a server constant that could drift out of step.
        oreCapacity = 1;
        if (state.ore != null)
        {
            foreach (int amount in state.ore)
            {
                oreCapacity = Mathf.Max(oreCapacity, amount);
            }
        }

        UpdateOre(state);
    }

    // UpdateOre reshades the ore field for the current frame: richer cells
    // draw brighter, and a cell that's been worked out reverts to plain
    // ground. Called on every state message, not just the initial one.
    public void UpdateOre(GameState state)
    {
        if (oreCells == null || state.ore == null)
        {
            return;
        }

        int count = Mathf.Min(oreCells.Length, state.ore.Length);
        bool first = lastAmounts == null || lastAmounts.Length != count;
        if (first)
        {
            lastAmounts = new int[count];
        }

        for (int i = 0; i < count; i++)
        {
            int amount = state.ore[i];
            if (!first && lastAmounts[i] == amount)
            {
                continue;
            }
            lastAmounts[i] = amount;

            var position = new Vector3Int(oreCells[i].x, oreCells[i].y, 0);
            tilemap.SetTile(position, tileAssets[(int)(amount > 0 ? TerrainType.Ore : TerrainType.Grass)]);

            if (amount <= 0)
            {
                continue;
            }

            // SetTile resets the flags, so unlocking the colour has to come
            // after it — otherwise the tint is silently ignored.
            tilemap.SetTileFlags(position, TileFlags.None);
            float density = Mathf.Clamp01((float)amount / oreCapacity);
            tilemap.SetColor(position, Color.Lerp(SparseOre, Color.white, density));
        }
    }

    public bool IsPassable(int x, int y)
    {
        if (passable == null || x < 0 || x >= width || y < 0 || y >= height)
        {
            return false;
        }
        return passable[y * width + x];
    }
}
