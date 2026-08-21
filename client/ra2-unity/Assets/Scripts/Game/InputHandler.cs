using UnityEngine;
using UnityEngine.InputSystem;

// Right-click issues an order for whatever SelectionHandler currently has
// selected: attack if the click landed on an enemy unit, move otherwise.
// WebSocketClient.Send is fire-and-forget, so this never blocks Update().
public class InputHandler : MonoBehaviour
{
    [SerializeField] private WebSocketClient webSocketClient;
    [SerializeField] private Grid grid;
    [SerializeField] private SelectionHandler selectionHandler;

    private const int MyOwner = 1; // hardcoded until Phase 6 real player identity

    private void Update()
    {
        if (Mouse.current == null || !Mouse.current.rightButton.wasPressedThisFrame)
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

        UnitView clicked = HitTestUnit(worldPoint);
        if (clicked != null && clicked.Owner != MyOwner)
        {
            webSocketClient.Send(new ClientCommand
            {
                type = "attack",
                unitIds = unitIds,
                targetUnitId = clicked.UnitId,
            });
            return;
        }

        // worldPoint is in isometric-projected screen space (where the
        // tilemap is actually drawn); the server thinks in plain cell
        // coordinates, so invert the projection before sending.
        Vector2 cell = IsoCoordConverter.WorldToCell(grid, worldPoint);

        webSocketClient.Send(new ClientCommand
        {
            type = "move",
            unitIds = unitIds,
            targetX = cell.x,
            targetY = cell.y,
        });
    }

    private static UnitView HitTestUnit(Vector3 worldPoint)
    {
        Collider2D hit = Physics2D.OverlapPoint(worldPoint);
        return hit != null ? hit.GetComponent<UnitView>() : null;
    }
}
