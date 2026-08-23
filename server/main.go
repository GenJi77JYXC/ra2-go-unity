package main

import "server/network"

// The tick loop used to live here, driving one global world. It now lives
// in each Room (see network/room.go) — one goroutine per match, still the
// single writer to its own world — so main just wires up the lobby and
// serves.
func main() {
	rooms := network.NewRoomManager()
	network.NewServer(rooms).ListenAndServe(":8080")
}
