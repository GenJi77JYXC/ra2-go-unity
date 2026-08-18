# client/ — Unity 工程

这个目录用来放 Unity 客户端工程本体,目前还未创建,按下面步骤在**这个目录里**创建:

1. 打开 Unity Hub → New Project
2. Editor 版本选 2022.3 LTS(或更新的 LTS)
3. Template 选 2D(Core / URP 均可)
4. **Location 直接指向本仓库的 `client/` 目录**(不要新建子文件夹),Project name 随意
5. 创建完成后,Unity 会在这里生成 `Assets/`、`Packages/`、`ProjectSettings/` 等标准目录,根目录的 `.gitignore` 已经覆盖了 `client/Library`、`client/Temp` 等不需要入库的产物

## 推荐的 Scripts 目录结构(对照学习计划各 Phase)

创建好工程后,在 `Assets/Scripts/` 下建议按职责分层,而不是按 Phase 分:

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

1. 用 `System.Net.WebSockets.ClientWebSocket` 写一个最小的 `ReceiveLoop()`,连上 `server/` 跑起来的 `ws://localhost:8080/ws`
2. 收到消息先 `Debug.Log`,确认协议通了(注意此时 `server/network/server.go` 里的 WebSocket upgrade 还是 TODO 占位,要先按 `server/` 的 TODO 接入 `nhooyr.io/websocket` 才能真正连通)
3. 协议确认没问题后,再加 `GameManager` + `Vector3.Lerp` 渲染红色方块移动

详见仓库根目录的学习计划文档 Phase 1 部分。
