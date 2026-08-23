using System.Collections.Generic;
using UnityEngine;
using UnityEngine.InputSystem;

// Left-click drag draws a selection rectangle; releasing selects every
// owned unit inside it. A small drag (effectively a click) selects the
// single nearest owned unit under the cursor instead, and clicking empty
// ground clears the selection. InputHandler reads SelectedUnitIds when a
// right-click issues a move/attack order.
public class SelectionHandler : MonoBehaviour
{
    private const float ClickDragThreshold = 5f; // pixels
    private const float ClickSelectRadius = 40f; // pixels

    [SerializeField] private GameManager gameManager;
    [SerializeField] private BuildPanel buildPanel;
    [SerializeField] private BuildPlacementHandler placementHandler;
    [SerializeField] private Grid grid;

    public HashSet<int> SelectedUnitIds { get; } = new();

    // Buildings are selected one at a time (there's no drag-select for
    // them) — 0 means none. BuildPanel reads this to decide whether to
    // show a production panel.
    public int SelectedBuildingId { get; private set; }

    private Vector2 dragStartScreen;
    private bool isDragging;

    private void Update()
    {
        if (Mouse.current == null)
        {
            return;
        }

        // While placing a structure, or when the cursor is over a panel,
        // clicks belong to the UI rather than to the map.
        if (placementHandler.IsPlacing || buildPanel.MouseOverPanel)
        {
            isDragging = false;
            return;
        }

        if (Mouse.current.leftButton.wasPressedThisFrame)
        {
            dragStartScreen = Mouse.current.position.ReadValue();
            isDragging = true;
            return;
        }

        if (Mouse.current.leftButton.wasReleasedThisFrame && isDragging)
        {
            isDragging = false;
            Vector2 dragEndScreen = Mouse.current.position.ReadValue();

            if (Vector2.Distance(dragStartScreen, dragEndScreen) < ClickDragThreshold)
            {
                SelectNearest(dragEndScreen);
            }
            else
            {
                SelectInRect(dragStartScreen, dragEndScreen);
            }
        }
    }

    private void SelectNearest(Vector2 screenPos)
    {
        UnitView[] units = FindObjectsByType<UnitView>(FindObjectsInactive.Exclude);

        // A click landing inside a building's footprint selects that
        // building instead of a unit — checked in cell space, since an
        // isometric footprint is a diamond that no BoxCollider2D matches.
        Vector3 world = Camera.main.ScreenToWorldPoint(screenPos);
        Vector2 cell = IsoCoordConverter.WorldToCell(grid, world);
        BuildingView building = gameManager.BuildingAtCell(
            Mathf.FloorToInt(cell.x), Mathf.FloorToInt(cell.y));

        if (building != null)
        {
            SelectedBuildingId = building.BuildingId;
            SelectedUnitIds.Clear();
            ApplySelectionVisuals(units);
            return;
        }

        SelectedBuildingId = 0;

        UnitView nearest = null;
        float nearestDist = float.MaxValue;

        foreach (UnitView unit in units)
        {
            if (unit.Owner != gameManager.MyPlayerId)
            {
                continue;
            }

            float dist = Vector2.Distance(screenPos, Camera.main.WorldToScreenPoint(unit.transform.position));
            if (dist < nearestDist)
            {
                nearestDist = dist;
                nearest = unit;
            }
        }

        SelectedUnitIds.Clear();
        if (nearest != null && nearestDist <= ClickSelectRadius)
        {
            SelectedUnitIds.Add(nearest.UnitId);
        }

        ApplySelectionVisuals(units);
    }

    private void SelectInRect(Vector2 startScreen, Vector2 endScreen)
    {
        Rect rect = Rect.MinMaxRect(
            Mathf.Min(startScreen.x, endScreen.x), Mathf.Min(startScreen.y, endScreen.y),
            Mathf.Max(startScreen.x, endScreen.x), Mathf.Max(startScreen.y, endScreen.y));

        UnitView[] units = FindObjectsByType<UnitView>(FindObjectsInactive.Exclude);

        SelectedBuildingId = 0; // a drag-select is always about units
        SelectedUnitIds.Clear();
        foreach (UnitView unit in units)
        {
            if (unit.Owner != gameManager.MyPlayerId)
            {
                continue;
            }

            if (rect.Contains(Camera.main.WorldToScreenPoint(unit.transform.position)))
            {
                SelectedUnitIds.Add(unit.UnitId);
            }
        }

        ApplySelectionVisuals(units);
    }

    private void ApplySelectionVisuals(UnitView[] units)
    {
        foreach (UnitView unit in units)
        {
            bool selected = SelectedUnitIds.Contains(unit.UnitId);
            unit.SelectionCircle.SetVisible(selected);
            unit.HealthBar.SetSelected(selected);
        }

        // Buildings have no selection ring (the production panel is the
        // feedback), but their bar follows the same damaged-or-selected
        // rule units use.
        foreach (BuildingView building in gameManager.Buildings)
        {
            building.HealthBar.SetSelected(building.BuildingId == SelectedBuildingId);
        }
    }

    // IMGUI rather than a Canvas — this is a screen-space overlay with no
    // world position, drawn only while actively dragging, so OnGUI's
    // immediate-mode drawing is simpler than standing up UI infrastructure
    // for it.
    private void OnGUI()
    {
        if (!isDragging || Mouse.current == null)
        {
            return;
        }

        Rect rect = GuiRectFromScreenPoints(dragStartScreen, Mouse.current.position.ReadValue());
        DrawRectOutline(rect, 2f, Color.white);
    }

    private static Rect GuiRectFromScreenPoints(Vector2 a, Vector2 b)
    {
        // Mouse/Input space has Y increasing upward from the bottom; OnGUI
        // space has Y increasing downward from the top, so both need
        // flipping before building a Rect from them.
        float aY = Screen.height - a.y;
        float bY = Screen.height - b.y;

        return Rect.MinMaxRect(
            Mathf.Min(a.x, b.x), Mathf.Min(aY, bY),
            Mathf.Max(a.x, b.x), Mathf.Max(aY, bY));
    }

    private static Texture2D whiteTexture;

    private static void DrawRectOutline(Rect rect, float thickness, Color color)
    {
        if (whiteTexture == null)
        {
            whiteTexture = new Texture2D(1, 1);
            whiteTexture.SetPixel(0, 0, Color.white);
            whiteTexture.Apply();
        }

        Color previous = GUI.color;
        GUI.color = color;

        GUI.DrawTexture(new Rect(rect.x, rect.y, rect.width, thickness), whiteTexture);
        GUI.DrawTexture(new Rect(rect.x, rect.yMax - thickness, rect.width, thickness), whiteTexture);
        GUI.DrawTexture(new Rect(rect.x, rect.y, thickness, rect.height), whiteTexture);
        GUI.DrawTexture(new Rect(rect.xMax - thickness, rect.y, thickness, rect.height), whiteTexture);

        GUI.color = previous;
    }
}
