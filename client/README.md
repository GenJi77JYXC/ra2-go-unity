# client/ — Unity 工程

Unity 客户端工程本体在 **`client/ra2-unity/`**。

> Unity Hub 的 "Location" 只是父目录,实际工程会自动建在 `<Location>/<Project name>/` 下,没法让它直接落在 `client/` 里——所以选 Location 为 `client/`、Project name 为 `ra2-unity` 之后,工程实际路径就是 `client/ra2-unity/`。根目录的 `.gitignore` 已经按这个路径覆盖了 `client/ra2-unity/Library`、`client/ra2-unity/Temp` 等不需要入库的产物。

当前用的 Editor 版本:**6000.5.8f1**。

如果要重新创建:

1. 打开 Unity Hub → New Project
2. Editor 版本选已装的 6000.5.8f1(或更新的 LTS)
3. Template 选 2D(Core / URP 均可)
4. Location 选本仓库的 `client/` 目录,Project name 填 `ra2-unity`
5. 创建完成后,Unity 会在 `client/ra2-unity/` 生成 `Assets/`、`Packages/`、`ProjectSettings/` 等标准目录

## 推荐的 Scripts 目录结构(对照学习计划各 Phase)

创建好工程后,在 `ra2-unity/Assets/Scripts/` 下建议按职责分层,而不是按 Phase 分:

```
Assets/Scripts/
├── Network/     ClientWebSocket 封装、ReceiveLoop、消息收发事件
├── Game/        GameManager、GameState/UnitSnapshot/ClientCommand(需与 Go 端 JSON tag 完全一致)
├── Camera/      CameraController(缩放/边缘滚屏/拖拽,Phase 2)
├── Map/         MapRenderer、IsoCoordConverter(Phase 3)
├── Selection/   SelectionHandler、SelectionCircle(Phase 4)
├── Combat/      HealthBar 等(Phase 4)
├── UI/          BuildPanel、BuildPlacementHandler(Phase 5)
└── Menu/        房间/大厅 UI(Phase 6)
```

## Phase 1 第一步

按学习计划的建议,先不写渲染代码:

1. 用 `System.Net.WebSockets.ClientWebSocket` 写一个最小的 `ReceiveLoop()`,连上 `server/` 跑起来的 `ws://localhost:8080/ws`(服务端 WebSocket 已经实现好了,直接 `go run .` 启动即可)
2. 收到消息先 `Debug.Log`,确认协议通了
3. 协议确认没问题后,再加 `GameManager` + `Vector3.Lerp` 渲染红色方块移动

详见仓库根目录的学习计划文档 Phase 1 部分。
