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
    [SerializeField] private GameObject unitPrefab;
    [SerializeField] private MapRenderer mapRenderer;
    [SerializeField] private Grid grid;
    [SerializeField] private Sprite buildingSprite; // WhiteDiamond
    [SerializeField] private Sprite barSprite;      // WhiteSquare, for building health bars
    [SerializeField] private float lerpSpeed = 10f;

    private readonly Dictionary<int, UnitView> units = new();
    private readonly Dictionary<int, Vector3> targetPositions = new();
    private readonly Dictionary<int, BuildingView> buildings = new();

    // Economy and the build catalog, surfaced for BuildPanel/
    // BuildPlacementHandler — GameManager is the only thing that parses
    // snapshots, so everything else reads game state through it.
    public int Money { get; private set; }
    public int Power { get; private set; }
    public BuildOption[] BuildMenu { get; private set; } = new BuildOption[0];
    public IEnumerable<BuildingView> Buildings => buildings.Values;

    // The structure currently being built (no map position until placed).
    public string PendingType { get; private set; } = "";
    public float PendingProgress { get; private set; }
    public bool PendingReady { get; private set; }

    private QueueSnapshot[] queues = new QueueSnapshot[0];

    // Production queues are per building type, not per building, so the
    // panel for any Barracks shows the same queue.
    public QueueSnapshot FindQueue(string category)
    {
        foreach (QueueSnapshot queue in queues)
        {
            if (queue.category == category)
            {
                return queue;
            }
        }
        return null;
    }

    public BuildOption FindBuildOption(string type)
    {
        foreach (BuildOption option in BuildMenu)
        {
            if (option.type == type)
            {
                return option;
            }
        }
        return null;
    }

    public BuildingView BuildingAtCell(int cellX, int cellY)
    {
        foreach (BuildingView building in buildings.Values)
        {
            if (building.Contains(cellX, cellY))
            {
                return building;
            }
        }
        return null;
    }

    // Which Owner in the world this client controls, assigned by the
    // server when the player took a seat. Read from the session rather
    // than stored, so there's one source of truth for it.
    public int MyPlayerId => session != null ? session.MyPlayerId : 0;

    private LobbyController session;

    // The connection lives on the persistent NetworkSession object, which
    // an Inspector field in this scene can't reach — hence the lookup.
    private void Start()
    {
        if (NetworkSession.Instance == null)
        {
            Debug.LogError("[GameManager] no NetworkSession — start from the Menu scene.");
            return;
        }

        session = NetworkSession.Instance.Lobby;
        session.RegisterGameManager(this); // replays anything buffered during the scene load
    }

    private void OnDestroy()
    {
        if (session != null)
        {
            session.UnregisterGameManager(this);
        }
    }

    // Everything in the scene sends through here, so only this one script
    // has to know where the connection lives.
    public void Send(ClientCommand cmd)
    {
        if (session != null)
        {
            session.Send(cmd);
        }
    }

    private void Update()
    {
        foreach (var kv in targetPositions)
        {
            Transform t = units[kv.Key].transform;
            t.position = Vector3.Lerp(t.position, kv.Value, lerpSpeed * Time.deltaTime);
        }
    }

    // Called by LobbyController once it has unwrapped the envelope — this
    // no longer subscribes to raw messages, since the same socket now also
    // carries lobby traffic that means nothing here.
    public void HandleState(GameState state)
    {
        if (state == null)
        {
            return;
        }

        if (state.isInitial)
        {
            mapRenderer.Render(state);
            BuildMenu = state.buildMenu ?? new BuildOption[0];
        }

        Money = state.money;
        Power = state.power;
        PendingType = state.pendingType ?? "";
        PendingProgress = state.pendingProgress;
        PendingReady = state.pendingReady;
        queues = state.queues ?? new QueueSnapshot[0];

        SyncBuildings(state);

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
                view.Initialize(unit.id, unit.owner, MyPlayerId);
                units[unit.id] = view;
            }

            // Position first, cosmetics second: a unit that renders in the
            // wrong place is far more broken than one missing a health bar,
            // so nothing optional gets to run ahead of this.
            //
            // Units live in the server's plain cell-space coordinates;
            // CellToWorld is what lines them up with the isometric tile
            // grid MapRenderer draws.
            targetPositions[unit.id] = IsoCoordConverter.CellToWorld(grid, unit.x, unit.y);

            view.HealthBar.SetHP(unit.hp, unit.maxHp);
        }

        RemoveDeadUnits(seenIds);
    }

    // Buildings appear (placed), update (construction progress), and
    // disappear (cancelled) the same way units do — the snapshot is the
    // full authoritative list every tick, so anything missing from it is
    // gone.
    private void SyncBuildings(GameState state)
    {
        BuildingSnapshot[] snapshots = state.buildings ?? new BuildingSnapshot[0];
        var seen = new HashSet<int>();

        foreach (BuildingSnapshot snapshot in snapshots)
        {
            seen.Add(snapshot.id);

            if (buildings.TryGetValue(snapshot.id, out BuildingView view))
            {
                view.Refresh(snapshot);
                continue;
            }

            // Footprint size comes from the build menu; the pre-placed
            // Construction Yard isn't in that menu (it can't be built), so
            // fall back to its known 3x3 size.
            BuildOption option = FindBuildOption(snapshot.type);
            int width = option?.width ?? 3;
            int height = option?.height ?? 3;

            buildings[snapshot.id] = BuildingView.Create(grid, buildingSprite, barSprite, snapshot, width, height, MyPlayerId);
        }

        List<int> goneIds = buildings.Keys.Where(id => !seen.Contains(id)).ToList();
        foreach (int id in goneIds)
        {
            Destroy(buildings[id].gameObject);
            buildings.Remove(id);
        }
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
