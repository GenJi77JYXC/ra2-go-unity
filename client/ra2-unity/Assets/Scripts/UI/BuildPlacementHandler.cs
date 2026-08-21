using UnityEngine;
using UnityEngine.InputSystem;

// Placement mode: a translucent preview of the finished structure follows
// the mouse, snapped to the isometric grid, tinted by whether the spot
// looks legal. Left-click confirms and sends the place command;
// right-click or Escape backs out (the structure stays built and ready,
// it just isn't sited yet).
//
// This is only ever entered for a structure the server already reports as
// ready — building happens first, siting second. The legality tint is a
// client-side guess from map data the client already has; the server
// re-checks in handlePlaceCommand and is the one that actually decides, so
// a green preview it rejects simply does nothing.
public class BuildPlacementHandler : MonoBehaviour
{
    [SerializeField] private WebSocketClient webSocketClient;
    [SerializeField] private GameManager gameManager;
    [SerializeField] private MapRenderer mapRenderer;
    [SerializeField] private Grid grid;
    [SerializeField] private BuildPanel buildPanel;
    [SerializeField] private Sprite previewSprite; // WhiteDiamond

    private static readonly Color ValidColor = new(0.3f, 0.9f, 0.3f, 0.45f);
    private static readonly Color InvalidColor = new(0.9f, 0.3f, 0.3f, 0.45f);

    public bool IsPlacing => pending != null;

    private BuildOption pending;
    private GameObject preview;
    private SpriteRenderer previewRenderer;

    // Built once up front rather than lazily on first use: a lazily
    // created preview leaves a window where pending is set but preview
    // isn't, and Update dereferences it unconditionally.
    private void Awake()
    {
        preview = new GameObject("BuildPreview");
        preview.transform.SetParent(transform, false);

        previewRenderer = preview.AddComponent<SpriteRenderer>();
        previewRenderer.sprite = previewSprite;
        previewRenderer.sortingLayerName = "Units";
        previewRenderer.sortingOrder = 20; // above everything while placing

        preview.SetActive(false);
    }

    public void Begin(BuildOption option)
    {
        pending = option;
        preview.transform.localScale = new Vector3(option.width, option.height, 1f);
        preview.SetActive(true);
    }

    public void Cancel()
    {
        pending = null;
        preview.SetActive(false);
    }

    private void Update()
    {
        if (pending == null || Mouse.current == null)
        {
            return;
        }

        if (Keyboard.current != null && Keyboard.current.escapeKey.wasPressedThisFrame)
        {
            Cancel();
            return;
        }

        if (Mouse.current.rightButton.wasPressedThisFrame)
        {
            Cancel();
            return;
        }

        Vector2Int cell = MouseCell();
        bool valid = CanPlace(cell);

        preview.transform.position = IsoCoordConverter.CellToWorld(
            grid, cell.x + pending.width / 2f, cell.y + pending.height / 2f);
        previewRenderer.color = valid ? ValidColor : InvalidColor;

        // Clicking the panel (e.g. picking a different structure) must not
        // also drop this one on whatever cell happens to be behind it.
        if (buildPanel.MouseOverPanel)
        {
            return;
        }

        if (Mouse.current.leftButton.wasPressedThisFrame && valid)
        {
            // No itemType: the server already knows which structure is
            // waiting to be sited, and taking its word for it means the
            // client can't ask for something it never paid for.
            webSocketClient.Send(new ClientCommand
            {
                type = "place",
                cellX = cell.x,
                cellY = cell.y,
            });
            Cancel();
        }
    }

    private Vector2Int MouseCell()
    {
        Vector3 world = Camera.main.ScreenToWorldPoint(Mouse.current.position.ReadValue());
        Vector2 cell = IsoCoordConverter.WorldToCell(grid, world);
        return new Vector2Int(Mathf.FloorToInt(cell.x), Mathf.FloorToInt(cell.y));
    }

    private bool CanPlace(Vector2Int origin)
    {
        for (int y = origin.y; y < origin.y + pending.height; y++)
        {
            for (int x = origin.x; x < origin.x + pending.width; x++)
            {
                if (!mapRenderer.IsPassable(x, y) || gameManager.BuildingAtCell(x, y) != null)
                {
                    return false;
                }
            }
        }
        return true;
    }

}
