using System.Collections.Generic;
using UnityEngine;
using UnityEngine.InputSystem;

// HUD (money/power), the build menu, and the production panel for a
// selected factory — all IMGUI, like Phase 4's selection rectangle.
// A real game would use uGUI/UI Toolkit here; OnGUI keeps this to one
// self-contained script with no Canvas hierarchy to hand-build, and can be
// converted the same way the health bars were once the behavior is settled.
public class BuildPanel : MonoBehaviour
{
    private const int MyOwner = 1; // hardcoded until Phase 6 real player identity

    [SerializeField] private WebSocketClient webSocketClient;
    [SerializeField] private GameManager gameManager;
    [SerializeField] private SelectionHandler selectionHandler;
    [SerializeField] private BuildPlacementHandler placementHandler;

    private const float PanelWidth = 190f;
    private const float ButtonHeight = 30f;
    private const float Margin = 10f;

    // True while the cursor is over any panel, so SelectionHandler and
    // InputHandler can ignore clicks that were meant for the UI rather
    // than the map. OnGUI coordinates are top-left origin, mouse input is
    // bottom-left, hence the flip.
    public bool MouseOverPanel { get; private set; }

    private Rect panelRect;
    private Rect productionRect;

    private void Update()
    {
        if (Mouse.current == null)
        {
            return;
        }

        Vector2 mouse = Mouse.current.position.ReadValue();
        Vector2 guiPoint = new(mouse.x, Screen.height - mouse.y);
        MouseOverPanel = panelRect.Contains(guiPoint) || productionRect.Contains(guiPoint);
    }

    private void OnGUI()
    {
        DrawHud();
        DrawBuildMenu();
        DrawProduction();
    }

    private void DrawHud()
    {
        var rect = new Rect(Margin, Margin, 260f, 26f);
        GUI.Box(rect, "");

        int power = gameManager.Power;
        string powerText = power < 0 ? $"POWER {power} (LOW)" : $"POWER +{power}";
        GUI.Label(new Rect(rect.x + 8f, rect.y + 4f, rect.width - 16f, 20f),
            $"CREDITS  {gameManager.Money}      {powerText}");
    }

    private void DrawBuildMenu()
    {
        BuildOption[] menu = gameManager.BuildMenu;
        if (menu.Length == 0)
        {
            panelRect = Rect.zero;
            return;
        }

        float height = menu.Length * (ButtonHeight + 4f) + 56f;
        panelRect = new Rect(Margin, 50f, PanelWidth, height);
        GUI.Box(panelRect, "BUILD");

        HashSet<string> completed = CompletedBuildingTypes();
        string pending = gameManager.PendingType;

        for (int i = 0; i < menu.Length; i++)
        {
            BuildOption option = menu[i];
            var buttonRect = new Rect(panelRect.x + 6f, panelRect.y + 24f + i * (ButtonHeight + 4f),
                PanelWidth - 12f, ButtonHeight);

            bool isPending = option.type == pending;

            // One structure at a time: while something is building, every
            // other button is dead. Cost isn't a gate — construction
            // charges as it goes, so ordering with an empty wallet just
            // stalls until income catches up.
            GUI.enabled = HasPrerequisites(option, completed) && (pending == "" || isPending);

            if (GUI.Button(buttonRect, BuildButtonLabel(option, isPending)))
            {
                OnBuildButton(option, isPending);
            }

            GUI.enabled = true;
        }

        var cancelRect = new Rect(panelRect.x + 6f, panelRect.yMax - 28f, PanelWidth - 12f, 22f);
        GUI.enabled = pending != "";
        if (GUI.Button(cancelRect, "Cancel construction"))
        {
            // No buildingId: that's what tells the server this cancels the
            // pending structure rather than a factory's unit queue.
            webSocketClient.Send(new ClientCommand { type = "cancel" });
        }
        GUI.enabled = true;
    }

    private string BuildButtonLabel(BuildOption option, bool isPending)
    {
        if (!isPending)
        {
            return $"{option.type}  ${option.cost}";
        }
        return gameManager.PendingReady
            ? $"{option.type}  READY"
            : $"{option.type}  {Mathf.RoundToInt(gameManager.PendingProgress * 100f)}%";
    }

    // Clicking a structure starts building it; clicking it again once it's
    // ready is what opens placement. Building first and siting second is
    // the original game's flow — the structure is paid for and finished
    // before the player ever picks a spot.
    private void OnBuildButton(BuildOption option, bool isPending)
    {
        if (!isPending)
        {
            webSocketClient.Send(new ClientCommand { type = "build", itemType = option.type });
            return;
        }

        if (gameManager.PendingReady)
        {
            placementHandler.Begin(option);
        }
    }

    private void DrawProduction()
    {
        BuildingView factory = SelectedFactory();
        if (factory == null)
        {
            productionRect = Rect.zero;
            return;
        }

        BuildOption option = gameManager.FindBuildOption(factory.BuildingType);
        string[] produces = option?.produces ?? new string[0];
        if (produces.Length == 0)
        {
            productionRect = Rect.zero;
            return;
        }

        float height = produces.Length * (ButtonHeight + 4f) + 108f;
        productionRect = new Rect(Screen.width - PanelWidth - Margin, 50f, PanelWidth, height);
        GUI.Box(productionRect, factory.BuildingType);

        // The queue belongs to the building *type*, not this building —
        // every Barracks shows the same one.
        QueueSnapshot queue = gameManager.FindQueue(factory.BuildingType);

        for (int i = 0; i < produces.Length; i++)
        {
            string unitType = produces[i];
            var buttonRect = new Rect(productionRect.x + 6f, productionRect.y + 24f + i * (ButtonHeight + 4f),
                PanelWidth - 12f, ButtonHeight);

            // Show how many are on order, so queuing several gives visible
            // feedback instead of looking like nothing happened. Only the
            // head of the queue is actually building, so its type carries
            // the count.
            int queued = queue != null && queue.item == unitType ? queue.length : 0;
            string label = queued > 0 ? $"Train {unitType}  x{queued}" : $"Train {unitType}";

            if (GUI.Button(buttonRect, label))
            {
                webSocketClient.Send(new ClientCommand
                {
                    type = "produce",
                    buildingId = factory.BuildingId,
                    itemType = unitType,
                });
            }
        }

        // Which factory the finished unit walks out of. Only worth
        // offering when there's more than one to choose between.
        var primaryRect = new Rect(productionRect.x + 6f, productionRect.yMax - 78f, PanelWidth - 12f, 22f);
        if (factory.IsPrimary)
        {
            GUI.Label(primaryRect, "  ★ Primary (units exit here)");
        }
        else if (GUI.Button(primaryRect, "Set as primary"))
        {
            webSocketClient.Send(new ClientCommand { type = "setPrimary", buildingId = factory.BuildingId });
        }

        var statusRect = new Rect(productionRect.x + 6f, productionRect.yMax - 50f, PanelWidth - 12f, 18f);
        GUI.Label(statusRect, queue != null
            ? $"Building {queue.item}  {Mathf.RoundToInt(queue.progress * 100f)}%"
            : "Idle");

        var cancelRect = new Rect(productionRect.x + 6f, productionRect.yMax - 28f, PanelWidth - 12f, 22f);
        if (GUI.Button(cancelRect, "Cancel last order"))
        {
            webSocketClient.Send(new ClientCommand { type = "cancel", buildingId = factory.BuildingId });
        }
    }

    private BuildingView SelectedFactory()
    {
        int id = selectionHandler.SelectedBuildingId;
        if (id == 0)
        {
            return null;
        }

        foreach (BuildingView building in gameManager.Buildings)
        {
            if (building.BuildingId == id && building.Owner == MyOwner && building.IsBuilt)
            {
                return building;
            }
        }
        return null;
    }

    private HashSet<string> CompletedBuildingTypes()
    {
        var types = new HashSet<string>();
        foreach (BuildingView building in gameManager.Buildings)
        {
            if (building.Owner == MyOwner && building.IsBuilt)
            {
                types.Add(building.BuildingType);
            }
        }
        return types;
    }

    private static bool HasPrerequisites(BuildOption option, HashSet<string> completed)
    {
        if (option.prerequisites == null)
        {
            return true;
        }

        foreach (string required in option.prerequisites)
        {
            if (!completed.Contains(required))
            {
                return false;
            }
        }
        return true;
    }
}
