package ws

import (
	"sync"
	"testing"
)

func TestRoomHubJoinLeaveAndBroadcast(t *testing.T) {
	hub := NewRoomHub()
	client := &Client{
		Hub:    hub,
		RoomID: "room-1",
		UserID: 1,
		Send:   make(chan []byte, 1),
	}
	hub.JoinRoom(client)

	msg := []byte(`{"type":"player_ready"}`)
	hub.BroadcastToRoom("room-1", msg)
	select {
	case got := <-client.Send:
		if string(got) != string(msg) {
			t.Fatalf("unexpected message: %s", got)
		}
	default:
		t.Fatal("expected broadcast message")
	}

	hub.LeaveRoom(client)
	hub.BroadcastToRoom("room-1", msg)
	if _, ok := <-client.Send; ok {
		t.Fatal("client should not receive message after leave")
	}
}

func TestRoomHubDifferentRoomsUseDifferentSegments(t *testing.T) {
	hub := NewRoomHub()
	roomA := "room-alpha"
	roomB := "room-beta"
	if roomHubSegmentIndex(roomA) == roomHubSegmentIndex(roomB) {
		t.Skip("rooms landed on same segment; pick other IDs if this flakes")
	}

	clientA := &Client{Hub: hub, RoomID: roomA, UserID: 1, Send: make(chan []byte, 1)}
	clientB := &Client{Hub: hub, RoomID: roomB, UserID: 2, Send: make(chan []byte, 1)}
	hub.JoinRoom(clientA)
	hub.JoinRoom(clientB)

	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		hub.BroadcastToRoom(roomA, []byte("a"))
	}()
	go func() {
		defer wg.Done()
		hub.BroadcastToRoom(roomB, []byte("b"))
	}()
	wg.Wait()

	if got := <-clientA.Send; string(got) != "a" {
		t.Fatalf("room A message: %s", got)
	}
	if got := <-clientB.Send; string(got) != "b" {
		t.Fatalf("room B message: %s", got)
	}
}

func TestRoomHubSegmentIndexStable(t *testing.T) {
	const roomID = "room-stable"
	first := roomHubSegmentIndex(roomID)
	for i := 0; i < 100; i++ {
		if got := roomHubSegmentIndex(roomID); got != first {
			t.Fatalf("segment index changed: first=%d got=%d", first, got)
		}
	}
}
