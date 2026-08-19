using UnityEngine;
using UnityEngine.InputSystem;

// Phase 2: right-click on the ground issues a move command for unit 1.
// WebSocketClient.Send is fire-and-forget, so this never blocks Update().
public class InputHandler : MonoBehaviour
{
    [SerializeField] private WebSocketClient webSocketClient;

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

        webSocketClient.Send(new ClientCommand
        {
            type = "move",
            unitIds = new[] { ControlledUnitId },
            targetX = worldPoint.x,
            targetY = worldPoint.y,
        });
    }
}
