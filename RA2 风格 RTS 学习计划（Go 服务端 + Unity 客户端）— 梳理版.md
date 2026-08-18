# RA2 风格 RTS 学习计划（Go 服务端 + Unity 客户端）

> 本文档在原始计划基础上做了结构梳理，并加入了关键风险提示和改进建议（见每个 Phase 末尾的「⚠️ 注意事项」）。总体架构和阶段划分予以保留，因为思路本身是合理的。

------

## 一、核心设计原则

| 原则                  | 说明                                                         |
| --------------------- | ------------------------------------------------------------ |
| 服务端权威            | 所有游戏逻辑在 Go 侧执行，Unity 只负责渲染和输入采集         |
| 命令-状态模型         | 客户端发 `Command`，服务端推 `State Snapshot`                |
| 固定 Tick Rate        | 服务端 20 tick/s（50ms），客户端渲染帧率不限                 |
| JSON 通信             | 前期用 JSON 便于调试，Chrome DevTools / websocat 都能直接抓包看 |
| 自写 WebSocket 客户端 | 用 `System.Net.WebSockets.ClientWebSocket`，不依赖第三方插件，学习底层协议 |

**架构图**

```
Unity Client (渲染/输入/UI)
        │  WebSocket (JSON)
Go Server
  ├─ NetworkLayer（房间管理/连接/广播）
  ├─ Game Simulation（命令处理/Tick/战斗/建造/寻路/AI）
  └─ Data Layer（RULES.INI 解析/地图/单位模板）
```

------

## 二、总体阶段规划

| Phase | 内容                   | 预估周期 |
| ----- | ---------------------- | -------- |
| 1     | 基础骨架（Hello Tank） | 2-3 周   |
| 2     | 单坦克移动（命令回路） | 3-4 周   |
| 3     | 等距地图渲染 + A* 寻路 | 4-6 周   |
| 4     | 多单位 + 框选 + 战斗   | 4-6 周   |
| 5     | 建造系统               | 4-6 周   |
| 6     | 多人在线               | 4-6 周   |
| 7     | AI 对手                | 4-6 周   |
| 8     | 打磨 + RA2 资源接入    | 6-8 周   |
| 9     | 可选深度方向           | 不限     |

**总周期预估 12-18 个月**。这是一个诚实但偏乐观的估计——历史经验是，个人独立开发的 RTS 项目最容易在 Phase 3-4（等距渲染、框选战斗）卡住而放弃，因为这两个阶段第一次要同时处理"数学正确性"（坐标转换、寻路）和"手感"（拖框、点击判定）两类问题。

**⚠️ 节奏建议**：如果某个阶段实际耗时超过预估的 1.5 倍，直接削减该阶段的功能范围往前走，不要死磕到"完美"再进入下一阶段。先出可运行的 build，回头再补。

------

## 三、环境准备

| 软件                     | 版本要求     | 用途     | 验证命令        |
| ------------------------ | ------------ | -------- | --------------- |
| Go                       | ≥ 1.21       | 服务端   | `go version`    |
| Unity Hub + Editor       | ≥ 2022.3 LTS | 客户端   | Unity Hub 查看  |
| Git                      | 任意         | 版本管理 | `git --version` |
| VS Code / GoLand / Rider | 任意         | 编辑器   | —               |

```bash
mkdir ra2-go-unity && cd ra2-go-unity
git init
# .gitignore 需覆盖 Go 编译产物 + Unity Library/Temp/Obj/Build 等目录
```

Git 仓库从 Phase 1 就建，`server/` 和 `client/` 两个目录放在同一个 repo 里，方便对照提交历史。

------

## 四、Phase 1：基础骨架（Hello Tank）

**目标**：验证 Go ↔ Unity 通信链路。Go 启动 WebSocket 服务器，每 tick 推送一个位置递增的坦克状态；Unity 收到后让方块移动。

### Go 服务端要点

- 项目结构：`main.go` + `network/`（server.go / client.go / message.go）+ `game/world.go`
- WebSocket 库选 `nhooyr.io/websocket`（而非 `gorilla/websocket`）：更现代、支持 context、内置 ping/pong，且 gorilla 已归档不再维护
- 消息结构：`GameState`（服务端→客户端，含 tick + units）、`ClientCommand`（客户端→服务端，含 type/unitIds/targetX/targetY 等）
- 主循环：`time.NewTicker(50 * time.Millisecond)`，每 tick 调用 `world.Tick()` 后 `srv.Broadcast(world.Snapshot())`
- 用 `websocat ws://localhost:8080/ws` 在写 Unity 代码之前先验证服务端是否正常输出 JSON 流

### Unity 客户端要点

- 用 `System.Net.WebSockets.ClientWebSocket`（.NET 内置，Unity 2022.3+ 自带，无需装包）
- `ReceiveLoop()` 异步循环接收，通过 C# `event` 把 JSON 字符串抛给 `GameManager`，解耦网络层和游戏层
- `GameState` / `UnitSnapshot` / `ClientCommand` 用 `[Serializable]` 标注，配合 `JsonUtility.FromJson<T>()`，字段名必须和 Go 端 JSON tag **完全一致（大小写敏感）**
- `GameManager` 收到快照后用 `Vector3.Lerp` 平滑插值移动 GameObject，避免 20Hz tick rate 造成的顿挫感
- 摄像机设 Orthographic，Size=10，Position (0,0,-10)

### 里程碑验证

```
Go 终端：tick=1 → tick=2 → tick=3 ...（持续输出）
Unity 屏幕：一个红色方块从左往右匀速移动
websocat：能连上并看到 JSON 流
```

### ⚠️ 注意事项

1. **建议进一步拆分**：可以先让 Go 端裸推数字、Unity 端只打印日志（不做渲染），确认协议本身通了，再加渲染逻辑。这样一旦出问题，排查范围更小，不会在"到底是网络问题还是渲染问题"上纠结。
2. **消息分片**：`ClientWebSocket.ReceiveAsync` 对超过 buffer 大小的消息会分片返回（`EndOfMessage == false`），Phase 1 消息很小可以先忽略，但**建 Phase 3 加入全量地图快照后必须处理**，否则大地图数据会被截断。

------

## 五、Phase 2：单坦克移动（命令回路）

**目标**：完整闭环——Unity 右键点击地面 → 发 `move` 命令 → Go 处理 → 坦克移动 → 状态同步回 Unity。

### Go 侧

- `Unit` 结构体加入 `TargetX/TargetY/HasTarget/State` 字段
- `updateUnit()`：计算到目标点的距离，按 `Speed * dt` 步进，到达阈值内视为完成
- `HandleCommand()`：处理 `move` 类型命令，设置目标点

### Unity 侧

- `CameraController`：滚轮缩放 + 边缘滚屏 + 中键拖拽平移
- `InputHandler`：右键 → `ScreenToWorldPoint` → 构造 `ClientCommand` → `wsClient.SendMessage()`（fire-and-forget，不 await，避免阻塞 `Update()`）
- 可选：点击位置指示器（半透明圆点，自动淡出销毁）

### 里程碑验证

```
右键点击地图某处 → Console 打印发送日志 → Go 终端打印收到日志
→ 红色方块向目标点移动，到达后停止
→ 滚轮缩放、中键拖拽正常
```

### ⚠️ 注意事项

**命令校验缺失**：当前 `HandleCommand` 没有检查"发起命令的客户端是否有权操作这些 `unitIds`"。哪怕是单机学习阶段，也建议现在就加上 `unit.Owner == 命令发起者的玩家ID` 的校验（Phase 2 可以先给所有单位一个固定 Owner=1 占位），因为：

- Phase 6 加入多人后这是**必须项**，现在不加，届时要回头改所有命令处理函数
- 提前建立"服务端永远不信任客户端输入"的习惯，这是服务端权威架构的核心

------

## 六、Phase 3：等距地图渲染 + A* 寻路

**目标**：等距视角地图渲染 + 寻路。坦克遇水域/悬崖自动绕行。

### Go 侧

- `GameMap` / `Tile` 数据结构：`TerrainType`（Grass/Road/Water/Cliff/Ore）+ `Passable`
- **首次连接发全量地图，后续 tick 只发单位变化**（`GameState.IsInitial` 字段区分），避免 64×64 地图每 tick 都传输造成带宽浪费
- A* 寻路：`container/heap` 实现优先队列，4 方向邻居 + 欧几里得距离作为启发函数
- 坐标转换：世界坐标 → 格子坐标（`WorldToCell`）用于寻路，格子坐标 → 世界坐标（`CellCenterWorld`）用于生成路径点

### Unity 侧

- Isometric Tilemap（Window → 2D → Tile Palette，Grid 类型选 Isometric，Cell Size 设为 (1, 0.5, 1)）

- `MapRenderer`：根据服务端 `TileData` 数组用 `Tilemap.SetTile()` 铺地形

- ```
  IsoCoordConverter
  ```

  ：等距坐标核心公式

  - `screenX = (cellX - cellY) * tileWidth / 2`
  - `screenY = (cellX + cellY) * tileHeight / 2`

### 里程碑验证

```
屏幕：等距地图（草地/湖泊/公路/悬崖）
右键点击地图另一端 → 坦克沿 A* 路径绕开湖泊和悬崖
右键点击湖中心 → 坦克不动（不可通行）
```

### ⚠️ 注意事项

1. **启发函数应改为曼哈顿距离**：当前用 4 方向移动 + 欧几里得距离，虽然依然 admissible（不会导致非最优路径），但对纯 4 方向网格来说曼哈顿距离效率更高、扩展节点更少。如果后续开放 8 方向对角线移动（`dirs` 数组里注释掉的那部分），记得同步把对角线代价改成 `√2` 并重新评估启发函数选择（八方向下欧几里得距离更合适）。
2. **地图分片处理**：呼应 Phase 1 的提醒，全量地图数据（4096 个 tile 的 JSON）大概率会触发 `ClientWebSocket` 的消息分片，Phase 3 必须实现分片消息的拼接逻辑，否则地图数据会被截断导致渲染错乱或解析失败。

------

## 七、Phase 4：多单位 + 框选 + 战斗

**目标**：框选多单位、右键攻击、伤害计算、血条、死亡。

### Go 侧

- 单位模板/武器模板/弹头模板（硬编码 map，Phase 8 再替换为 INI 解析）
- 伤害公式：`damage = weapon.Damage * warhead.Verses[targetArmor] / 100`
- `updateCombat()`：距离判断、攻击冷却、追击逻辑、死亡单位清理

### Unity 侧

- `SelectionHandler`：左键拖拽画框选中矩形内单位，或单击选中最近单位
- `SelectionCircle`：选中高亮圈的显隐控制
- 右键判断：点中敌方单位则发 `attack` 命令，点空地则发 `move` 命令
- `HealthBar`：World Space Canvas + Slider，按血量比例变色（绿/黄/红），满血时隐藏

### 里程碑验证

```
框选 3 辆我方坦克 → 绿色高亮圈
右键敌方坦克 → 坦克移动过去，进入射程后自动开火
敌方血条减少，HP 归零后消失
Go 终端打印每次开火的伤害日志
```

### ⚠️ 注意事项

**命令校验（承接 Phase 2）**：这一阶段引入了 `attack` 命令和真正的多单位归属，是把 Phase 2 埋下的"命令校验"补上的最后时机——一旦 Phase 5/6 建造系统和多人对战上线，没有校验的命令处理函数会成为最容易出 bug（甚至被"作弊"）的地方。建议在这里统一加一层：所有命令处理前先验证 `unitIds` 中的单位 `Owner` 字段。

------

## 八、Phase 5：建造系统

**目标**：基地车/主基地、造建筑、造兵、电力/资金/科技树。

### Go 侧

- `Building` 结构体：位置、占地格数、建造进度、生产队列
- `Player` 结构体：金钱、电力、已建建筑集合
- 科技树依赖：`Prerequisites(unitType)` 返回前置建筑列表，建造/生产前检查
- 建造流程：验证位置合法（可通行、不重叠）→ 验证科技树和金钱 → 扣钱 → 创建 Building（`IsBuilt=false`）→ 计时完成后解锁
- 生产队列：`updateProduction()` 按 `BuildTime` 计时，完成后 `spawnUnit()`

### Unity 侧

- `BuildPanel`：建造按钮面板，点击后进入放置模式
- `BuildPlacementHandler`：半透明预览方块跟随鼠标并对齐格子，点击确认发送 `build` 命令

### 里程碑验证

```
点击基地车 → 建造面板出现 → 选电厂 → 半透明预览跟随鼠标
点地面 → 扣钱 → 地基出现 → 进度条走完 → 建筑完成
点兵营 → 生产面板 → 选大兵 → 等待后诞生单位
HUD 显示金钱/电力
```

### ⚠️ 注意事项

**电力不足时的降速逻辑**（原计划提到但代码未展开）：`BuildTime -= dt`，若电力不足则改为 `dt * 0.5`。这个分支逻辑容易被漏掉测试——建议专门写一个测试用例：电厂被摧毁后，正在建造中的建筑速度应明显变慢而不是报错或卡死。

------

## 九、Phase 6：多人在线

**目标**：房间创建/加入/开始，双人对战状态同步。

### Go 侧

- `RoomManager` + `Room` + `Player`：房间生命周期管理（Waiting/Playing/Finished）
- 每个 `Room` 独立持有一个 `Game` 实例，广播时遍历房间内玩家逐一 `Send()`
- 简化版本先不做战争迷雾，所有玩家收到相同全量状态，靠 `owner` 字段区分归属

### Unity 侧

- 新场景 `Menu.unity`：昵称输入、服务器地址输入、房间列表、创建/加入房间

### 里程碑验证

```
两个客户端连接同一服务器 → A 创建房间 → B 加入 → 双方准备 → 开始
A 移动坦克，B 实时看到同步 → 双方战斗正常
```

### ⚠️ 注意事项

1. **`sync.RWMutex` 的粒度**：`Room.mu` 保护的是房间级状态（玩家列表），而 `Game`/`World` 内部状态如果也被多个 goroutine（tick 循环 + 多个客户端的 `OnMessage` 回调）并发读写，需要单独的锁或者把所有状态变更收敛到 tick 循环所在的 goroutine 里处理（推荐后者，通过 channel 把命令传给主循环，避免到处加锁）。
2. **命令校验在这里是硬性要求**：Phase 2/4 提到的 owner 校验，到这一步如果还没加，双人对战时一方就能控制另一方的单位，这是必须在合并到本阶段前修复的问题。

------

## 十、Phase 7：AI 对手

**目标**：单机可玩，AI 会发展经济、生产军队、发动攻击。

### Go 侧

- `AIPlayer`：目标驱动架构（`Goal` 按优先级排序，`Status: pending/active/completed`）
- 每秒评估一次局势（`assess`），刷新目标列表（`reassessGoals`），执行最高优先级目标（`executeTopGoal`）
- 目标类型覆盖经济（发电/矿场）和军事（兵营/重工/训练/进攻）两条线
- 攻击编队：坦克数达到阈值后集结，发 `attack_move` 命令（沿路径遇敌自动攻击）

### 里程碑验证

```
选择「对抗 AI」→ AI 依次造电厂/兵营/矿场/重工
30-60秒内开始生产坦克，60-90秒后集结进攻
电力不足自动补电厂，矿场被拆自动重建
```

### ⚠️ 注意事项

**目标系统的状态机边界**：`Goal.Status` 只有 pending/active/completed 三态，没有"failed"或"blocked"状态。例如 `GoalBuildPower` 如果找不到可建造的空地会一直保持 `pending` 重复尝试，这在地图拥挤时可能导致 AI 卡住不动。建议加一个失败计数或超时机制，多次失败后降级到其他目标（比如清理占地或换个建筑类型）。

------

## 十一、Phase 8：打磨 + RA2 资源接入

**目标**：占位素材换成真实 RA2 精灵图/体素。

### 流程

- 用 XCC Mixer 从 RA2 安装目录提取 SHP 精灵序列帧、AUD 音效
- Go 侧写简单的 INI 解析器，解析 `RULES.INI` 中的单位/武器/弹头真实数值，替换 Phase 4 硬编码的模板表
- Unity 侧 `SHPLoader` 按方向和帧序号切分 sprite sheet；`UnitAnimator` 驱动状态机切换动画

### ⚠️ 注意事项

**法律边界**：RA2 是 EA 拥有版权的商业游戏，提取其精灵图/音效资源仅适合个人学习和本地验证效果使用。**不建议将这部分资源提交到公开的代码仓库或以任何形式对外分发**，包括开源仓库的资产目录里直接放入提取出的 SHP/PNG/AUD 文件。如果计划长期维护或开源这个项目，Phase 8 可以考虑用风格类似但完全原创/开源授权的像素素材替代。

------

## 十二、Phase 9：可选深度方向

| 方向                        | 学习重点                                         | 预估周期 |
| --------------------------- | ------------------------------------------------ | -------- |
| 性能优化（100+单位不卡）    | Go: ECS/内存池/pprof；Unity: DOTS/GPU Instancing | 4-8 周   |
| 锁步同步替代服务端权威      | 定点数、回滚（rollback）                         | 6-10 周  |
| AI 进阶（难度分级/多策略）  | 行为树、GOAP                                     | 6-12 周  |
| 地图编辑器                  | Unity Editor 扩展                                | 6-8 周   |
| GAAS 化（账号/排行榜/匹配） | PostgreSQL/Redis/gRPC/JWT                        | 6-12 周  |
| 移动端适配                  | 触控操作、UI 缩放                                | 4-6 周   |
| 战役系统                    | Lua 嵌入 Go、剧情脚本                            | 8-12 周  |
| Mod 支持                    | Go plugin 系统、热加载                           | 6-10 周  |

这一阶段完全可选，建议根据个人兴趣挑 1-2 个方向深入，而不是全部尝试——到 Phase 8 结束时已经是一个完整可玩的 RTS，边际学习收益开始集中在少数几个专精方向上。

------

## 十三、跨阶段的通用建议汇总

把前面各阶段分散提到的注意事项汇总一下，作为贯穿整个项目的检查清单：

1. **命令校验要尽早加**：Phase 2 引入命令系统时就该加 `Owner` 校验的占位逻辑，不要等到 Phase 6 多人联机时才补——那时候要改的地方更多，也更容易漏改。
2. **大消息的分片处理**：Phase 3 全量地图快照会触发 `ClientWebSocket` 的消息分片，Phase 1 写 `ReceiveLoop` 时如果图省事跳过了分片拼接，这里必须回头补上。
3. **并发安全收敛到主循环**：多个 goroutine（tick 循环 + 每个客户端连接的读协程）同时改 `World` 状态是最容易出 data race 的地方。推荐做法是所有客户端命令先塞进一个 channel，由 tick 循环所在的单一 goroutine 消费处理，尽量避免到处加锁。
4. **每阶段出一个可运行 build**：哪怕全是色块，能跑起来比"看起来完美但还没验证"更重要。
5. **Git 从第一天开始**：`server/` 和 `client/` 同一个仓库，方便对照两侧改动的时间线。
6. **卡壳超过预期 1.5 倍就简化范围**：比如寻路先做直线移动，A* 放到功能跑通后再补。
7. **RA2 资源仅用于个人学习**，不进入公开仓库或对外分发的版本。

------

## 十四、技术依赖速查

### Go 依赖

| 包                     | 用途             | 引入阶段 |
| ---------------------- | ---------------- | -------- |
| `net/http`             | HTTP 服务器      | P1       |
| `nhooyr.io/websocket`  | WebSocket 服务端 | P1       |
| `encoding/json`        | JSON 编解码      | P1       |
| `container/heap`       | A* 优先队列      | P3       |
| `crypto/rand`          | 房间 ID 生成     | P6       |
| `bufio`/`os`/`strings` | INI 文件解析     | P8       |
| `math`                 | 几何计算         | 贯穿全程 |

### Unity 依赖（均为内置，无需装第三方包）

| 功能                                    | 用途              | 引入阶段 |
| --------------------------------------- | ----------------- | -------- |
| `System.Net.WebSockets.ClientWebSocket` | WebSocket 客户端  | P1       |
| `JsonUtility`                           | JSON 编解码       | P1       |
| 2D Tilemap + Isometric Grid             | 等距地图渲染      | P3       |
| uGUI (Canvas/Button/Panel)              | UI 组件           | P4-6     |
| Input（旧版即可）                       | 鼠标键盘输入      | P1-2     |
| Addressables（可选）                    | 大型资源管理      | P8+      |
| Shader Graph（可选）                    | 战争迷雾/水面特效 | P9       |

------

## 十五、每阶段学习收获对照表

| 阶段 | Go 侧学会                             | Unity 侧学会                              | 通用技能                |
| ---- | ------------------------------------- | ----------------------------------------- | ----------------------- |
| P1   | WebSocket server、goroutine、JSON tag | ClientWebSocket 用法、Sprite 基础         | TCP/WS 协议、消息序列化 |
| P2   | Tick 循环、命令模式、状态机           | ScreenToWorldPoint、Lerp 插值、摄像机控制 | 游戏循环、输入处理      |
| P3   | 地图数据结构、A* 寻路、heap 用法      | 等距 Tilemap、等距数学                    | 空间数据结构、寻路算法  |
| P4   | 战斗公式、伤害系统                    | 框选、血条 UI                             | 游戏平衡、UI 开发       |
| P5   | 建造逻辑、资源管理、科技树            | 建造面板、放置预览                        | 经济系统、RTS 核心循环  |
| P6   | 房间管理、并发安全                    | 大厅 UI、多场景                           | 多人架构、sync.RWMutex  |
| P7   | 目标驱动 AI、态势评估                 | —                                         | AI 设计、启发式搜索     |
| P8   | 二进制文件解析（SHP/INI）             | SHP 动画加载                              | 逆向工程、资源管线      |
| P9   | ECS、pprof、gRPC                      | DOTS、Editor 扩展                         | 性能优化、工具开发      |

------

*本文档基于原始计划梳理整理，保留了原计划的架构决策和阶段划分，重点补充了命令校验、并发安全、消息分片、法律边界等在原计划中未充分展开的风险点。*