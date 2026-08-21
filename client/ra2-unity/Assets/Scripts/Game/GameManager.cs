using System.Collections.Generic;
using System.Linq;
using UnityEngine;

// Spawns one GameObject per unit and smoothly follows the server's
// position. Snapshots only arrive at 20Hz, so stepping straight to the new
// position every message would look choppy — instead we keep a target
// position and Lerp toward it every rendered frame. Also the single
// consumer of the one-off isInitial map snapshot (Phase 3), forwarded to
// MapRenderer, and (Phase 4) pushes HP into each unit's HealthBar and
// destroys the GameObject for any unit that drops out of the snapshot.
public class GameManager : MonoBehaviour
{
    [SerializeField] private WebSocketClient webSocketClient;
    [SerializeField] private GameObject unitPrefab;
    [SerializeField] private MapRenderer mapRenderer;
    [SerializeField] private Grid grid;
    [SerializeField] private float lerpSpeed = 10f;

    private readonly Dictionary<int, UnitView> units = new();
    private readonly Dictionary<int, Vector3> targetPositions = new();

    private void OnEnable()
    {
        webSocketClient.OnMessage += HandleMessage;
    }

    private void OnDisable()
    {
        webSocketClient.OnMessage -= HandleMessage;
    }

    private void Update()
    {
        foreach (var kv in targetPositions)
        {
            Transform t = units[kv.Key].transform;
            t.position = Vector3.Lerp(t.position, kv.Value, lerpSpeed * Time.deltaTime);
        }
    }

    private void HandleMessage(string json)
    {
        GameState state = JsonUtility.FromJson<GameState>(json);

        if (state.isInitial)
        {
            mapRenderer.Render(state);
        }

        var seenIds = new HashSet<int>();

        foreach (UnitSnapshot unit in state.units)
        {
            seenIds.Add(unit.id);

            if (!units.TryGetValue(unit.id, out UnitView view))
            {
                // UnitView (and the HealthBar/SelectionCircle/Collider it
                // wires up to) is now hand-built into the Tank prefab —
                // see the setup instructions — so this is a plain
                // GetComponent, not AddComponent.
                view = Instantiate(unitPrefab).GetComponent<UnitView>();
                view.Initialize(unit.id, unit.owner);
                units[unit.id] = view;
            }

            view.HealthBar.SetHP(unit.hp, unit.maxHp);

            // Units live in the server's plain cell-space coordinates;
            // CellToWorld is what lines them up with the isometric tile
            // grid MapRenderer draws.
            targetPositions[unit.id] = IsoCoordConverter.CellToWorld(grid, unit.x, unit.y);
        }

        RemoveDeadUnits(seenIds);
    }

    // A unit that dropped out of state.units died and was already removed
    // server-side (see World.removeDeadUnits) — nothing more to wait for,
    // just clean up its GameObject.
    private void RemoveDeadUnits(HashSet<int> seenIds)
    {
        List<int> deadIds = units.Keys.Where(id => !seenIds.Contains(id)).ToList();
        foreach (int id in deadIds)
        {
            Destroy(units[id].gameObject);
            units.Remove(id);
            targetPositions.Remove(id);
        }
    }
}
