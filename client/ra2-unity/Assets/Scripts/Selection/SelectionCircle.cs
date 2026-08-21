using UnityEngine;

// The ring itself (sprite, color, sorting layer) is set up by hand on this
// GameObject in the Tank prefab, using the SelectionRing sprite from
// Tools > RA2 > Generate Unit UI Sprites — this script only shows/hides it.
public class SelectionCircle : MonoBehaviour
{
    private void Awake()
    {
        SetVisible(false);
    }

    public void SetVisible(bool visible)
    {
        gameObject.SetActive(visible);
    }
}
