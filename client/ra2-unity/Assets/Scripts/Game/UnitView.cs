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

    // Harvesters are civilian machines parked in the middle of a fight, so
    // they read as a duller, larger version of their owner's colour: same
    // hue, so friend/foe is still the first thing you see, but obviously
    // not a tank. Real per-unit art is a Phase 8 problem.
    private const string HarvesterTemplate = "Harvester";
    private const float HarvesterTint = 0.6f;
    private const float HarvesterScale = 1.3f;

    [SerializeField] private SpriteRenderer bodySprite;
    [SerializeField] private HealthBar healthBar;
    [SerializeField] private SelectionCircle selectionCircle;

    public int UnitId { get; private set; }
    public int Owner { get; private set; }
    public HealthBar HealthBar => healthBar;
    public SelectionCircle SelectionCircle => selectionCircle;

    // myPlayerId comes from the server's seat assignment rather than a
    // constant, so "friendly" means whoever is actually at the keyboard.
    public string Template { get; private set; }
    public bool IsHarvester => Template == HarvesterTemplate;

    public void Initialize(int unitId, int owner, int myPlayerId, string template)
    {
        UnitId = unitId;
        Owner = owner;
        Template = template ?? "";

        Color color = owner == myPlayerId ? FriendlyColor : EnemyColor;
        if (IsHarvester)
        {
            color *= HarvesterTint;
            color.a = 1f;
            transform.localScale *= HarvesterScale;
        }
        bodySprite.color = color;
    }
}
