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
    private const int MyOwner = 1; // hardcoded until Phase 6 real player identity
    private const float ClickDragThreshold = 5f; // pixels
    private const float ClickSelectRadius = 40f; // pixels

    public HashSet<int> SelectedUnitIds { get; } = new();

    private Vector2 dragStartScreen;
    private bool isDragging;

    private void Update()
    {
        if (Mouse.current == null)
        {
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

        UnitView nearest = null;
        float nearestDist = float.MaxValue;

        foreach (UnitView unit in units)
        {
            if (unit.Owner != MyOwner)
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

        SelectedUnitIds.Clear();
        foreach (UnitView unit in units)
        {
            if (unit.Owner != MyOwner)
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
