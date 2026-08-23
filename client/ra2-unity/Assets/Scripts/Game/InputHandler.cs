using UnityEngine;
using UnityEngine.InputSystem;

// Right-click issues an order for whatever SelectionHandler currently has
// selected: attack if the click landed on an enemy unit or building, move
// otherwise. WebSocketClient.Send is fire-and-forget, so this never blocks
// Update().
public class InputHandler : MonoBehaviour
{
    [SerializeField] private Grid grid;
    [SerializeField] private SelectionHandler selectionHandler;
    [SerializeField] private GameManager gameManager;
    [SerializeField] private BuildPanel buildPanel;
    [SerializeField] private BuildPlacementHandler placementHandler;


    private void Update()
    {
        if (Mouse.current == null || !Mouse.current.rightButton.wasPressedThisFrame)
        {
            return;
        }

        // While placing a structure the right button backs out of
        // placement (BuildPlacementHandler handles it), and clicks over a
        // panel belong to the UI — neither should issue an order.
        if (placementHandler.IsPlacing || buildPanel.MouseOverPanel)
        {
            return;
        }

        if (selectionHandler.SelectedUnitIds.Count == 0)
        {
            return; // nothing selected, nothing to command
        }

        int[] unitIds = new int[selectionHandler.SelectedUnitIds.Count];
        selectionHandler.SelectedUnitIds.CopyTo(unitIds);

        Vector3 screenPos = Mouse.current.position.ReadValue();
        Vector3 worldPoint = Camera.main.ScreenToWorldPoint(screenPos);

        UnitView clickedUnit = HitTestUnit(worldPoint);
        if (clickedUnit != null && clickedUnit.Owner != gameManager.MyPlayerId)
        {
            SendAttack(unitIds, clickedUnit.UnitId);
            return;
        }

        // worldPoint is in isometric-projected screen space (where the
        // tilemap is actually drawn); the server thinks in plain cell
        // coordinates, so invert the projection before sending.
        Vector2 cell = IsoCoordConverter.WorldToCell(grid, worldPoint);

        // Buildings are hit-tested in cell space rather than with a
        // collider — their isometric footprint is a diamond that no
        // BoxCollider2D matches.
        BuildingView clickedBuilding = gameManager.BuildingAtCell(
            Mathf.FloorToInt(cell.x), Mathf.FloorToInt(cell.y));
        if (clickedBuilding != null && clickedBuilding.Owner != gameManager.MyPlayerId)
        {
            SendAttack(unitIds, clickedBuilding.BuildingId);
            return;
        }

        gameManager.Send(new ClientCommand
        {
            type = "move",
            unitIds = unitIds,
            targetX = cell.x,
            targetY = cell.y,
        });
    }

    // targetUnitId carries a building's id just as happily as a unit's —
    // the server keeps both in one ID space, so an attack order doesn't
    // need to say which kind it means.
    private void SendAttack(int[] unitIds, int targetId)
    {
        gameManager.Send(new ClientCommand
        {
            type = "attack",
            unitIds = unitIds,
            targetUnitId = targetId,
        });
    }

    private static UnitView HitTestUnit(Vector3 worldPoint)
    {
        Collider2D hit = Physics2D.OverlapPoint(worldPoint);
        return hit != null ? hit.GetComponent<UnitView>() : null;
    }
}
