using System.Collections.Generic;
using UnityEngine;

// Phase 1 step 3: spawn one GameObject per unit and smoothly follow the
// server's position. Snapshots only arrive at 20Hz, so stepping straight to
// the new position every message would look choppy — instead we keep a
// target position and Lerp toward it every rendered frame.
public class GameManager : MonoBehaviour
{
    [SerializeField] private WebSocketClient webSocketClient;
    [SerializeField] private GameObject unitPrefab;
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

        foreach (UnitSnapshot unit in state.units)
        {
            if (!units.ContainsKey(unit.id))
            {
                units[unit.id] = Instantiate(unitPrefab);
            }

            targetPositions[unit.id] = new Vector3((float)unit.x, (float)unit.y, 0f);
        }
    }
}
