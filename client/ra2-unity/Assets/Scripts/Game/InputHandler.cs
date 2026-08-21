using UnityEngine;
using UnityEngine.InputSystem;

// Phase 2: right-click on the ground issues a move command for unit 1.
// WebSocketClient.Send is fire-and-forget, so this never blocks Update().
public class InputHandler : MonoBehaviour
{
    [SerializeField] private WebSocketClient webSocketClient;
    [SerializeField] private Grid grid;

    // Hardcoded until Phase 4 adds real unit selection.
    private const int ControlledUnitId = 1;

    private void Update()
    {
        if (Mouse.current == null || !Mouse.current.rightButton.wasPressedThisFrame)
        {
            return;
        }

        // Orthographic camera: X/Y are the same regardless of the input
        // z-distance, so the default position z (0) is fine here.
        Vector3 screenPos = Mouse.current.position.ReadValue();
        Vector3 worldPoint = Camera.main.ScreenToWorldPoint(screenPos);

        // worldPoint is in isometric-projected screen space (where the
        // tilemap is actually drawn); the server thinks in plain cell
        // coordinates, so invert the projection before sending.
        Vector2 cell = IsoCoordConverter.WorldToCell(grid, worldPoint);

        webSocketClient.Send(new ClientCommand
        {
            type = "move",
            unitIds = new[] { ControlledUnitId },
            targetX = cell.x,
            targetY = cell.y,
        });
    }
}
