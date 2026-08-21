using UnityEngine;

// Background/Fill are hand-built children in the Tank prefab (see
// client/README or the setup instructions that came with this change) —
// this script only reads and adjusts Fill, it doesn't create anything.
// Reading Fill's own starting localScale as "full width" means the
// prefab's dimensions are the only source of truth for bar size; this
// script doesn't need to know or duplicate that number.
public class HealthBar : MonoBehaviour
{
    [SerializeField] private SpriteRenderer fill;

    private Vector3 fullScale;
    private float currentRatio = 1f;
    private bool selected;

    private void Awake()
    {
        fullScale = fill.transform.localScale;
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

        // Shrink from the right, keeping the left edge fixed — the usual
        // health bar behavior — rather than scaling from the center.
        float width = fullScale.x * currentRatio;
        fill.transform.localScale = new Vector3(width, fullScale.y, fullScale.z);
        fill.transform.localPosition = new Vector3(-(fullScale.x - width) / 2f, 0f, 0f);

        fill.color = currentRatio > 0.5f ? Color.green : currentRatio > 0.25f ? Color.yellow : Color.red;
    }
}
