# ra2-go-unity

RA2 风格 RTS 学习项目 —— Go 服务端 + Unity 客户端。

完整学习计划见仓库根目录下的
《RA2 风格 RTS 学习计划（Go 服务端 + Unity 客户端）— 梳理版.md》。

## 目录结构

```
.
├── server/     Go 服务端(权威游戏逻辑、WebSocket)
│   ├── main.go
│   ├── network/    连接管理、消息收发
│   └── game/       World/Unit 等游戏状态
└── client/     Unity 客户端(渲染、输入)
    ├── README.md          如何在此处创建 Unity 工程
    └── ra2-unity/         Unity 工程本体(Assets/Packages/ProjectSettings 等)
```

## 当前进度

- [x] Go / Unity 环境
- [x] Git 仓库
- [x] 目录骨架
- [x] Phase 1: Hello Tank(协议打通 + 渲染)
- [x] Phase 2: 单坦克移动(命令回路)
- [x] Phase 3: 等距地图渲染 + A* 寻路
- [x] Phase 4: 多单位 + 框选 + 战斗
- [ ] Phase 5: 建造系统

## 已知问题(暂缓处理)

- **单位寻路时互相没有碰撞/避让**:多个单位一起收到移动指令时,最终目的地会被分散到不同格子(见 `nearbyPassableCells`),但**移动过程中**互相之间没有任何感知,路径重叠时会直接穿模。真正的局部避让(steering / reciprocal velocity obstacles 之类)工程量不小,且不在学习计划 Phase 4 的里程碑范围内,先记录下来,后续视情况单独处理。代码里的标记见 [server/game/world.go](server/game/world.go) `Unit.update` 方法上的 `TODO(known gap, deferred)` 注释。

## 快速开始(Phase 1)

```bash
cd server
go run .
# 另开一个终端验证 WebSocket 输出(先于 Unity 代码验证协议):
# websocat ws://localhost:8080/ws
```

Unity 工程见 `client/README.md`。
