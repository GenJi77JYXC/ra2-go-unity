using UnityEditor;
using UnityEngine;

// Editor-only utility. Phase 4's health bar and selection ring need two
// plain placeholder sprites — a solid square and a ring with the middle
// punched transparent. Unlike TerrainTileGenerator's tiles (consumed by
// code, via MapRenderer's tileAssets array), these two are meant to be
// dragged onto SpriteRenderers by hand in the Tank prefab, which is why
// they're saved as their own standalone .asset files here instead of
// being generated inline at runtime.
public static class UnitUIGenerator
{
    private const string OutputDir = "Assets/Sprites";
    private const int Size = 32;

    [MenuItem("Tools/RA2/Generate Unit UI Sprites")]
    private static void Generate()
    {
        if (!AssetDatabase.IsValidFolder(OutputDir))
        {
            AssetDatabase.CreateFolder("Assets", "Sprites");
        }

        SaveSprite("WhiteSquare", SolidTexture(), Size, Size);
        SaveSprite("SelectionRing", RingTexture(), Size, Size);

        // Buildings sit on the isometric grid, so their footprint is a
        // diamond like the terrain tiles — same 2:1 texture aspect, so a
        // uniform scale of N covers an NxN block of cells exactly.
        SaveSprite("WhiteDiamond", DiamondTexture(), DiamondWidth, DiamondHeight);

        AssetDatabase.SaveAssets();
        AssetDatabase.Refresh();
        Debug.Log($"Generated WhiteSquare, SelectionRing and WhiteDiamond sprites in {OutputDir}.");
    }

    private static void SaveSprite(string name, Texture2D texture, int width, int height)
    {
        string path = $"{OutputDir}/{name}.asset";
        AssetDatabase.DeleteAsset(path);

        texture.name = name + "Texture";
        // pixelsPerUnit = width makes the sprite exactly 1 world unit wide,
        // so a diamond ends up 1 x 0.5 — one cell at the Grid's Cell Size.
        Sprite sprite = Sprite.Create(texture, new Rect(0, 0, width, height), new Vector2(0.5f, 0.5f), width);
        sprite.name = name;

        // The Sprite is the asset's primary object (not wrapped in
        // anything), so it shows up directly as a draggable Sprite in the
        // object picker for any SpriteRenderer.sprite field.
        AssetDatabase.CreateAsset(sprite, path);
        AssetDatabase.AddObjectToAsset(texture, sprite);
    }

    private static Texture2D SolidTexture()
    {
        var texture = new Texture2D(Size, Size, TextureFormat.RGBA32, false);
        var pixels = new Color[Size * Size];
        for (int i = 0; i < pixels.Length; i++)
        {
            pixels[i] = Color.white;
        }
        texture.SetPixels(pixels);
        texture.Apply();
        return texture;
    }

    private const int DiamondWidth = 32;
    private const int DiamondHeight = 16;

    private static Texture2D DiamondTexture()
    {
        var texture = new Texture2D(DiamondWidth, DiamondHeight, TextureFormat.RGBA32, false);
        var pixels = new Color[DiamondWidth * DiamondHeight];

        for (int y = 0; y < DiamondHeight; y++)
        {
            for (int x = 0; x < DiamondWidth; x++)
            {
                float nx = (x + 0.5f) / DiamondWidth - 0.5f;
                float ny = (y + 0.5f) / DiamondHeight - 0.5f;
                bool inside = Mathf.Abs(nx) + Mathf.Abs(ny) <= 0.5f;
                pixels[y * DiamondWidth + x] = inside ? Color.white : new Color(0f, 0f, 0f, 0f);
            }
        }

        texture.SetPixels(pixels);
        texture.Apply();
        return texture;
    }

    private const float OuterRadius = 0.5f;
    private const float InnerRadius = 0.42f;

    private static Texture2D RingTexture()
    {
        var texture = new Texture2D(Size, Size, TextureFormat.RGBA32, false);
        var pixels = new Color[Size * Size];

        for (int y = 0; y < Size; y++)
        {
            for (int x = 0; x < Size; x++)
            {
                float nx = (x + 0.5f) / Size - 0.5f;
                float ny = (y + 0.5f) / Size - 0.5f;
                float dist = Mathf.Sqrt(nx * nx + ny * ny);
                bool inRing = dist <= OuterRadius && dist >= InnerRadius;
                pixels[y * Size + x] = inRing ? Color.white : new Color(0f, 0f, 0f, 0f);
            }
        }

        texture.SetPixels(pixels);
        texture.Apply();
        return texture;
    }
}
