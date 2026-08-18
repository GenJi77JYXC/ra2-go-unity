package main

import (
	"time"

	"server/game"
	"server/network"
)

func main() {
	world := game.NewWorld()
	srv := network.NewServer(world)

	go srv.ListenAndServe(":8080")

	ticker := time.NewTicker(50 * time.Millisecond) // 20 tick/s, per 学习计划 Phase 1
	defer ticker.Stop()

	for range ticker.C {
		world.Tick()
		srv.Broadcast(world.Snapshot())
	}
}
