# ra2-go-unity

RA2 风格 RTS 学习项目 —— Go 服务端 + Unity 客户端。

完整学习计划见仓库根目录下的
《RA2 风格 RTS 学习计划（Go 服务端 + Unity 客户端）— 梳理版.md》。

## 目录结构

```
.
├── server/     Go 服务端(权威游戏逻辑、WebSocket)
│   ├── main.go     只负责建大厅并监听——tick 循环在每个房间里
│   ├── network/    连接管理、房间/大厅、消息收发
│   └── game/       World/Unit/Building 等游戏状态与规则
└── client/     Unity 客户端(渲染、输入)
    ├── README.md          如何在此处创建 Unity 工程
    └── ra2-unity/         Unity 工程本体(Assets/Packages/ProjectSettings 等)
        └── Assets/Scenes/ Menu(大厅) + SampleScene(对局)
```

## 当前进度

- [x] Go / Unity 环境
- [x] Git 仓库
- [x] 目录骨架
- [x] Phase 1: Hello Tank(协议打通 + 渲染)
- [x] Phase 2: 单坦克移动(命令回路)
- [x] Phase 3: 等距地图渲染 + A* 寻路
- [x] Phase 4: 多单位 + 框选 + 战斗
- [x] Phase 5: 建造系统
- [x] Phase 6: 多人在线
- [ ] Phase 7: AI 对手

## 胜负条件

学习计划没有规定判负标准(只提到房间有 `Waiting/Playing/Finished` 三态),
这里按原版提供三种,由创建房间的人选择:

| 条件 | 判负标准 | 对应原版 |
|---|---|---|
| `buildings` | 所有建筑被摧毁(单位还在也算输) | RA2 默认 |
| `conyard` | 主基地被摧毁 | Short Game |
| `annihilation` | 建筑和单位全灭 | 全灭规则 |

实现见 [server/game/victory.go](server/game/victory.go)。

## 已知问题(暂缓处理)

- **单位寻路时互相没有碰撞/避让**:多个单位一起收到移动指令时,最终目的地会被分散到不同格子(见 `nearbyPassableCells`),但**移动过程中**互相之间没有任何感知,路径重叠时会直接穿模。真正的局部避让(steering / reciprocal velocity obstacles 之类)工程量不小,且不在学习计划任何一个 Phase 的里程碑范围内。**已排期在 Phase 6 之后处理。** 代码里的标记见 [server/game/world.go](server/game/world.go) `Unit.update` 方法上的 `TODO(known gap, deferred)` 注释。

## 快速开始

```bash
cd server
go run .
```

Unity 端从 **Menu 场景**启动(游戏场景需要大厅创建的持久连接对象,不能直接 Play)。
双人对战需要两个客户端,可以打包一个 exe 再配合编辑器 Play。

只验证服务端协议的话,不必开 Unity:

```bash
# 连上后手动发大厅/游戏指令，例如 {"type":"listRooms"}
websocat ws://localhost:8080/ws
```

Unity 工程见 `client/README.md`。
