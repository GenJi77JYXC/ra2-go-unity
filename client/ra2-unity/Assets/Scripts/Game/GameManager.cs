using System.Collections.Generic;
using UnityEngine;

// Spawns one GameObject per unit and smoothly follows the server's
// position. Snapshots only arrive at 20Hz, so stepping straight to the new
// position every message would look choppy — instead we keep a target
// position and Lerp toward it every rendered frame. Also the single
// consumer of the one-off isInitial map snapshot (Phase 3), forwarded to
// MapRenderer.
public class GameManager : MonoBehaviour
{
    [SerializeField] private WebSocketClient webSocketClient;
    [SerializeField] private GameObject unitPrefab;
    [SerializeField] private MapRenderer mapRenderer;
    [SerializeField] private Grid grid;
    [SerializeField] private float lerpSpeed = 10f;

    private readonly Dictionary<int, GameObject> units = new();
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
            GameObject unit = units[kv.Key];
            unit.transform.position = Vector3.Lerp(unit.transform.position, kv.Value, lerpSpeed * Time.deltaTime);
        }
    }

    private void HandleMessage(string json)
    {
        GameState state = JsonUtility.FromJson<GameState>(json);

        if (state.isInitial)
        {
            mapRenderer.Render(state);
        }

        foreach (UnitSnapshot unit in state.units)
        {
            if (!units.ContainsKey(unit.id))
            {
                units[unit.id] = Instantiate(unitPrefab);
            }

            // Units live in the server's plain cell-space coordinates;
            // CellToWorld is what lines them up with the isometric tile
            // grid MapRenderer draws.
            targetPositions[unit.id] = IsoCoordConverter.CellToWorld(grid, unit.x, unit.y);
        }
    }
}
