using UnityEngine;

// Drives a two-quad bar (dark backing + colored fill). Units get theirs
// from the hand-built Tank prefab; buildings, which have no prefab at all,
// build one through CreateFor — either way this script owns the sizing.
//
// It deliberately cancels out the parent's scale: the Tank is scaled
// (0.6, 0.5) to fit the isometric tile and buildings are scaled by their
// footprint, so a bar inheriting that came out squashed and sitting inside
// the sprite instead of floating above it. Sizing in world units here
// means every bar is the same readable size regardless of its owner.
public class HealthBar : MonoBehaviour
{
    [SerializeField] private SpriteRenderer fill;
    [SerializeField] private SpriteRenderer background;

    [SerializeField] private float barWidth = 0.8f;
    [SerializeField] private float barHeight = 0.12f;
    [SerializeField] private float heightAboveUnit = 0.5f;

    private float currentRatio = 1f;
    private bool selected;

    // Builds a bar from scratch under parent, for callers that aren't
    // working from a prefab. Assigning the serialized fields directly is
    // fine from inside the class, and keeps the prefab path unchanged.
    public static HealthBar CreateFor(Transform parent, Sprite barSprite, float heightAbove)
    {
        var obj = new GameObject("HealthBar");
        obj.transform.SetParent(parent, false);

        var bar = obj.AddComponent<HealthBar>();
        bar.heightAboveUnit = heightAbove;
        bar.background = CreateQuad(obj.transform, barSprite, "Background", Color.black, 10);
        bar.fill = CreateQuad(obj.transform, barSprite, "Fill", Color.green, 11);
        bar.ApplyVisual();
        return bar;
    }

    private static SpriteRenderer CreateQuad(Transform parent, Sprite sprite, string name, Color color, int sortingOrder)
    {
        var obj = new GameObject(name);
        obj.transform.SetParent(parent, false);

        var renderer = obj.AddComponent<SpriteRenderer>();
        renderer.sprite = sprite;
        renderer.color = color;
        renderer.sortingLayerName = "Units";
        renderer.sortingOrder = sortingOrder;
        return renderer;
    }

    private void Awake()
    {
        ApplyVisual();
    }

    public void SetHP(int hp, int maxHp)
    {
        currentRatio = maxHp > 0 ? Mathf.Clamp01((float)hp / maxHp) : 0f;
        ApplyVisual();
    }

    public void SetSelected(bool isSelected)
    {
        selected = isSelected;
        ApplyVisual();
    }

    private void ApplyVisual()
    {
        bool damaged = currentRatio > 0f && currentRatio < 1f;
        gameObject.SetActive(selected || damaged);

        Vector3 parentScale = transform.parent != null ? transform.parent.lossyScale : Vector3.one;
        float sx = Mathf.Approximately(parentScale.x, 0f) ? 1f : parentScale.x;
        float sy = Mathf.Approximately(parentScale.y, 0f) ? 1f : parentScale.y;

        // Undo the parent's scale so the children below measure in world
        // units, and lift the bar clear of the sprite it belongs to.
        transform.localScale = new Vector3(1f / sx, 1f / sy, 1f);
        transform.localPosition = new Vector3(0f, heightAboveUnit / sy, 0f);

        background.transform.localScale = new Vector3(barWidth, barHeight, 1f);

        // Shrink from the right, keeping the left edge fixed — the usual
        // health bar behavior — rather than scaling from the center.
        float width = barWidth * currentRatio;
        fill.transform.localScale = new Vector3(width, barHeight, 1f);
        fill.transform.localPosition = new Vector3(-(barWidth - width) / 2f, 0f, 0f);

        fill.color = currentRatio > 0.5f ? Color.green : currentRatio > 0.25f ? Color.yellow : Color.red;
    }
}
