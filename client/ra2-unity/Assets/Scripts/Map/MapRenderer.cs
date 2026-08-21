using UnityEngine;
using UnityEngine.Tilemaps;

// Phase 3: lays out the tile grid from the server's one-off isInitial
// GameState (see GameManager.HandleMessage). Attach directly to the
// Tilemap GameObject (Grid > Tilemap, Cell Layout = Isometric).
[RequireComponent(typeof(Tilemap))]
public class MapRenderer : MonoBehaviour
{
    // Indexed by TerrainType (see TileData) — Grass, Road, Water, Cliff,
    // Ore, in that exact order. Assign 5 Tile assets in the Inspector.
    [SerializeField] private TileBase[] tileAssets;

    private Tilemap tilemap;

    // Passability is kept around after rendering so the build preview can
    // tint itself without another round trip to the server (which still
    // makes the real call — see BuildPlacementHandler).
    private bool[] passable;
    private int width;
    private int height;

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
