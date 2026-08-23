using UnityEngine;

// Attached to the Tank prefab in the Editor (not via AddComponent anymore
// — see the setup instructions for building the Collider/HealthBar/
// SelectionCircle hierarchy by hand). Owns the collider InputHandler
// hit-tests against for attack-vs-move, and tints the unit's own sprite
// by owner.
public class UnitView : MonoBehaviour
{
    // Blue/red rather than reusing the health bar's green — a friendly
    // unit tinted green would fight for meaning with a green "full HP"
    // bar sitting right above it.
    private static readonly Color FriendlyColor = new(0.25f, 0.55f, 0.95f);
    private static readonly Color EnemyColor = new(0.85f, 0.2f, 0.2f);

    [SerializeField] private SpriteRenderer bodySprite;
    [SerializeField] private HealthBar healthBar;
    [SerializeField] private SelectionCircle selectionCircle;

    public int UnitId { get; private set; }
    public int Owner { get; private set; }
    public HealthBar HealthBar => healthBar;
    public SelectionCircle SelectionCircle => selectionCircle;

    // myPlayerId comes from the server's seat assignment rather than a
    // constant, so "friendly" means whoever is actually at the keyboard.
    public void Initialize(int unitId, int owner, int myPlayerId)
    {
        UnitId = unitId;
        Owner = owner;
        bodySprite.color = owner == myPlayerId ? FriendlyColor : EnemyColor;
    }
}
