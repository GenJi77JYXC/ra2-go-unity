using UnityEngine;

// Visual for one building. Unlike units (which use the hand-built Tank
// prefab), buildings are drawn entirely from code: they're flat tinted
// diamonds sized to their footprint, with no per-type art to hand-place.
public class BuildingView : MonoBehaviour
{
    private const int MyOwner = 1; // hardcoded until Phase 6 real player identity

    private static readonly Color FriendlyColor = new(0.30f, 0.45f, 0.75f);
    private static readonly Color EnemyColor = new(0.70f, 0.30f, 0.30f);

    public int BuildingId { get; private set; }
    public string BuildingType { get; private set; }
    public int Owner { get; private set; }
    public int CellX { get; private set; }
    public int CellY { get; private set; }
    public int Width { get; private set; }
    public int Height { get; private set; }
    public bool IsBuilt { get; private set; }

    // The factory finished units of this category walk out of. Production
    // queues are shared per building type, so exactly one factory of each
    // type carries this.
    public bool IsPrimary { get; private set; }

    private SpriteRenderer sprite;
    private SpriteRenderer primaryMarker;
    private Color baseColor;

    public HealthBar HealthBar { get; private set; }

    public static BuildingView Create(Grid grid, Sprite diamond, Sprite barSprite, BuildingSnapshot snapshot, int width, int height)
    {
        var obj = new GameObject($"Building_{snapshot.type}_{snapshot.id}");
        var view = obj.AddComponent<BuildingView>();
        view.Initialize(grid, diamond, barSprite, snapshot, width, height);
        return view;
    }

    private void Initialize(Grid grid, Sprite diamond, Sprite barSprite, BuildingSnapshot snapshot, int width, int height)
    {
        BuildingId = snapshot.id;
        BuildingType = snapshot.type;
        Owner = snapshot.owner;
        CellX = snapshot.cellX;
        CellY = snapshot.cellY;
        Width = width;
        Height = height;

        sprite = gameObject.AddComponent<SpriteRenderer>();
        sprite.sprite = diamond;
        sprite.sortingLayerName = "Units";
        sprite.sortingOrder = -2; // under units, over terrain

        baseColor = snapshot.owner == MyOwner ? FriendlyColor : EnemyColor;

        // The footprint's center in cell space is half its size past its
        // lower-left corner; the diamond sprite is exactly one cell, so
        // scaling by the footprint size covers it.
        transform.position = IsoCoordConverter.CellToWorld(grid, snapshot.cellX + width / 2f, snapshot.cellY + height / 2f);
        transform.localScale = new Vector3(width, height, 1f);

        // Clear the top of the diamond: an NxN footprint stands
        // N * cellHeight / 2 world units tall from its center, plus a
        // little margin. Cell size comes from the Grid so this tracks any
        // change to the tilemap's dimensions.
        float halfHeight = height * grid.cellSize.y / 2f;
        HealthBar = HealthBar.CreateFor(transform, barSprite, halfHeight + 0.2f);
        primaryMarker = CreatePrimaryMarker(barSprite, halfHeight);

        sprite.color = baseColor;
        Refresh(snapshot);
    }

    // A small flag above the structure. Buildings always arrive complete
    // now (they're only placed once built), so there's no construction
    // fade to conflict with this.
    private SpriteRenderer CreatePrimaryMarker(Sprite barSprite, float halfHeight)
    {
        var obj = new GameObject("PrimaryMarker");
        obj.transform.SetParent(transform, false);

        // Undo the footprint scaling on the parent so the flag is the same
        // size on a 2x2 and a 3x3.
        obj.transform.localScale = new Vector3(0.25f / Width, 0.25f / Height, 1f);
        obj.transform.localPosition = new Vector3(0f, (halfHeight + 0.45f) / Height, 0f);

        var renderer = obj.AddComponent<SpriteRenderer>();
        renderer.sprite = barSprite;
        renderer.color = new Color(1f, 0.85f, 0.2f);
        renderer.sortingLayerName = "Units";
        renderer.sortingOrder = 12;
        return renderer;
    }

    public void Refresh(BuildingSnapshot snapshot)
    {
        IsBuilt = snapshot.isBuilt;
        IsPrimary = snapshot.isPrimary;

        primaryMarker.gameObject.SetActive(snapshot.isPrimary);
        HealthBar.SetHP(snapshot.hp, snapshot.maxHp);
    }

    // Footprint containment is checked in cell space rather than with a
    // collider: the isometric footprint is a diamond, which a BoxCollider2D
    // can't match, and the server already thinks in cells anyway.
    public bool Contains(int cellX, int cellY)
    {
        return cellX >= CellX && cellX < CellX + Width &&
               cellY >= CellY && cellY < CellY + Height;
    }
}
