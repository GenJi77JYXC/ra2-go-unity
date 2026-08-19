using UnityEngine;
using UnityEngine.InputSystem;

// Phase 2: scroll-wheel zoom, edge-of-screen scroll, and middle-mouse drag
// pan. Attach directly to the Main Camera (must be Orthographic).
public class CameraController : MonoBehaviour
{
    [SerializeField] private float zoomSpeed = 1f; // orthographicSize change per scroll step
    [SerializeField] private float minZoom = 3f;
    [SerializeField] private float maxZoom = 20f;

    [SerializeField] private float edgeScrollSpeed = 15f;
    [SerializeField] private float edgeScrollMargin = 10f; // pixels from screen edge

    private Camera cam;
    private bool isDragging;
    private Vector3 dragOrigin;

    private void Awake()
    {
        cam = GetComponent<Camera>();
    }

    private void Update()
    {
        if (Mouse.current == null)
        {
            return;
        }

        HandleZoom();
        HandleEdgeScroll();
        HandleDragPan();
    }

    private void HandleZoom()
    {
        float scroll = Mouse.current.scroll.ReadValue().y;
        if (scroll == 0f)
        {
            return;
        }

        // Scroll delta magnitude varies wildly by device/platform (mouse
        // wheel notches vs. trackpad pixels), so only the direction is
        // used — each detected scroll event moves orthographicSize by a
        // fixed zoomSpeed step instead of being scaled by the raw delta.
        cam.orthographicSize = Mathf.Clamp(cam.orthographicSize - Mathf.Sign(scroll) * zoomSpeed, minZoom, maxZoom);
    }

    private void HandleEdgeScroll()
    {
        Vector2 mouse = Mouse.current.position.ReadValue();
        Vector3 move = Vector3.zero;

        if (mouse.x <= edgeScrollMargin) move.x -= 1f;
        else if (mouse.x >= Screen.width - edgeScrollMargin) move.x += 1f;

        if (mouse.y <= edgeScrollMargin) move.y -= 1f;
        else if (mouse.y >= Screen.height - edgeScrollMargin) move.y += 1f;

        if (move == Vector3.zero)
        {
            return;
        }

        transform.position += move.normalized * (edgeScrollSpeed * Time.deltaTime);
    }

    private void HandleDragPan()
    {
        if (Mouse.current.middleButton.wasPressedThisFrame)
        {
            isDragging = true;
            dragOrigin = cam.ScreenToWorldPoint(Mouse.current.position.ReadValue());
            return;
        }

        if (Mouse.current.middleButton.wasReleasedThisFrame)
        {
            isDragging = false;
            return;
        }

        if (!isDragging)
        {
            return;
        }

        // Orthographic X/Y don't depend on the z-distance passed in, so the
        // world point under the cursor is exact — moving the camera by the
        // delta keeps that same world point pinned under the cursor.
        Vector3 currentWorld = cam.ScreenToWorldPoint(Mouse.current.position.ReadValue());
        transform.position += dragOrigin - currentWorld;
    }
}
