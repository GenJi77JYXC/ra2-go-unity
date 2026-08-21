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

	ticker := time.NewTicker(game.TickInterval) // 20 tick/s
	defer ticker.Stop()

	dt := game.TickInterval.Seconds()

	for range ticker.C {
		drainCommands(srv, world)
		srv.DrainNewClients()
		world.Tick(dt)
		srv.Broadcast(world.Snapshot())
	}
}

// drainCommands applies every command queued since the last tick. This is
// the only place client-issued commands touch World, keeping the tick loop
// the sole writer to it.
func drainCommands(srv *network.Server, world *game.World) {
	for {
		select {
		case cmd := <-srv.Commands():
			world.HandleCommand(cmd)
		default:
			return
		}
	}
}
