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

    private void Awake()
    {
        tilemap = GetComponent<Tilemap>();
    }

    public void Render(GameState state)
    {
        for (int y = 0; y < state.mapHeight; y++)
        {
            for (int x = 0; x < state.mapWidth; x++)
            {
                TileData tile = state.tiles[y * state.mapWidth + x];
                tilemap.SetTile(new Vector3Int(x, y, 0), tileAssets[tile.type]);
            }
        }
    }
}
