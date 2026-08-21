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

        SaveSprite("WhiteSquare", SolidTexture());
        SaveSprite("SelectionRing", RingTexture());

        AssetDatabase.SaveAssets();
        AssetDatabase.Refresh();
        Debug.Log($"Generated WhiteSquare and SelectionRing sprites in {OutputDir}.");
    }

    private static void SaveSprite(string name, Texture2D texture)
    {
        string path = $"{OutputDir}/{name}.asset";
        AssetDatabase.DeleteAsset(path);

        texture.name = name + "Texture";
        Sprite sprite = Sprite.Create(texture, new Rect(0, 0, Size, Size), new Vector2(0.5f, 0.5f), Size);
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
