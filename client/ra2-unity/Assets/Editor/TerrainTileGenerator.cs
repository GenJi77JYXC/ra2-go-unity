using UnityEditor;
using UnityEngine;
using UnityEngine.Tilemaps;

// Editor-only utility. Generates the placeholder terrain Tile assets (no
// real art). Each tile's texture is a diamond (transparent corners) sized
// to exactly match the Grid's Cell Size (1, 0.5, 1) — a plain solid-color
// rectangle would still have a 1x1-ish bounding box that overlaps its
// screen-neighbors in isometric layout, which is what made water visibly
// bleed into the grass tiles next to it. Real isometric tile art avoids
// that by drawing an actual diamond with transparent corners; this does
// the same thing procedurally instead of needing imported art.
public static class TerrainTileGenerator
{
    private const string OutputDir = "Assets/Tiles";

    // Texture aspect must match TileWidth:TileHeight (1:0.5 = 2:1) from
    // IsoCoordConverter / the Grid's Cell Size, or the diamond will be the
    // wrong shape for the grid.
    private const int Width = 32;
    private const int Height = 16;

    // Order matches game.TerrainType / TileData.type exactly.
    private static readonly (string name, Color color)[] Terrains =
    {
        ("Grass", new Color(0.35f, 0.65f, 0.25f)),
        ("Road", new Color(0.55f, 0.55f, 0.55f)),
        ("Water", new Color(0.2f, 0.45f, 0.85f)),
        ("Cliff", new Color(0.5f, 0.35f, 0.2f)),
        ("Ore", new Color(0.9f, 0.75f, 0.15f)),
        // The drill is the fixture ore grows back from — dark and metallic
        // so it stands out against the field it sits in the middle of.
        ("OreDrill", new Color(0.28f, 0.3f, 0.34f)),
    };

    [MenuItem("Tools/RA2/Generate Terrain Tiles")]
    private static void Generate()
    {
        if (!AssetDatabase.IsValidFolder(OutputDir))
        {
            AssetDatabase.CreateFolder("Assets", "Tiles");
        }

        foreach ((string name, Color color) in Terrains)
        {
            string path = $"{OutputDir}/{name}.asset";
            AssetDatabase.DeleteAsset(path);

            Texture2D texture = DiamondTexture(color);
            texture.name = name + "Texture";

            // pixelsPerUnit = Width makes the sprite exactly Width/Width =
            // 1 unit wide and Height/Width = 0.5 unit tall — matching the
            // Grid's Cell Size precisely.
            Sprite sprite = Sprite.Create(texture, new Rect(0, 0, Width, Height), new Vector2(0.5f, 0.5f), Width);
            sprite.name = name + "Sprite";

            var tile = ScriptableObject.CreateInstance<Tile>();
            tile.sprite = sprite;

            // White, because the colour is already baked into the texture —
            // which also leaves the per-cell tint free for MapRenderer to
            // shade ore cells by how much they still hold.
            tile.color = Color.white;

            AssetDatabase.CreateAsset(tile, path);
            AssetDatabase.AddObjectToAsset(sprite, tile);
            AssetDatabase.AddObjectToAsset(texture, tile);
        }

        AssetDatabase.SaveAssets();
        AssetDatabase.Refresh();
        Debug.Log($"Generated {Terrains.Length} diamond terrain tiles in {OutputDir} — "
            + "order for MapRenderer.tileAssets: Grass, Road, Water, Cliff, Ore, OreDrill.");
    }

    // A diamond is the Manhattan-distance-<=0.5 region of the rect,
    // exactly the classic isometric tile mask.
    private static Texture2D DiamondTexture(Color color)
    {
        var texture = new Texture2D(Width, Height, TextureFormat.RGBA32, false);
        var pixels = new Color[Width * Height];

        for (int y = 0; y < Height; y++)
        {
            for (int x = 0; x < Width; x++)
            {
                float nx = (x + 0.5f) / Width - 0.5f;
                float ny = (y + 0.5f) / Height - 0.5f;
                bool inside = Mathf.Abs(nx) + Mathf.Abs(ny) <= 0.5f;
                pixels[y * Width + x] = inside ? color : new Color(0f, 0f, 0f, 0f);
            }
        }

        texture.SetPixels(pixels);
        texture.Apply();
        return texture;
    }
}
