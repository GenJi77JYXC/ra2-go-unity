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

## 单位避让与占地

单位按格占地:要离开当前格,得先**预约**下一格,过渡期间同时占住两格,
所以谁也钻不进别人正在跨越的缝里。建筑同样占地,寻路会绕开而不是穿过去。

被挡住之后是分级处理的,直接重新寻路会让单位互相蹭来蹭去:

| 情况 | 处理 |
|---|---|
| 挡住不到 0.5 秒 | 原地等——大多数堵塞自己就散了 |
| 超过 0.5 秒 | 重新寻路,这次把其他单位占的格也算障碍 |
| 绕不开,挡路的是静止单位 | 让对方挪一格;它挪完会停 1 秒,不然刚让开就走回来 |
| 正面对堵 | 按单位 ID 决定谁让,保证只有一个人动 |
| 挡住满 3 秒 | 放弃这次指令 |

单位记着自己的最终目的地(`Goal`),所以让完路会自己接着走原来的行程。
一个例外:**只检查要进入的格,不检查当前所在的格**——建筑可以盖在单位头上,
被盖住的单位必须还能走出来。

实现见 [server/game/occupancy.go](server/game/occupancy.go),
测试见 [server/game/movement_test.go](server/game/movement_test.go)。

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
