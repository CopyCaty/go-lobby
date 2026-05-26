package ws

import "testing"

func TestMapHubSubscriptionsAndBroadcast(t *testing.T) {
	hub := NewMapHub()
	first := &MapClient{Send: make(chan []byte, 1)}
	second := &MapClient{Send: make(chan []byte, 1)}
	hub.Join(first)
	hub.Join(second)

	hub.ReplaceSubscriptions(first, []string{"cn:6:10:20", "cn:6:10:21"})
	hub.ReplaceSubscriptions(second, []string{"cn:6:10:21"})
	hub.BroadcastToChunk("cn:6:10:20", []byte(`{"type":"mine.cell_opened"}`))

	select {
	case <-first.Send:
	default:
		t.Fatal("first client should receive subscribed chunk broadcast")
	}
	select {
	case <-second.Send:
		t.Fatal("second client should not receive unrelated chunk broadcast")
	default:
	}
}

func TestMapHubReplaceSubscriptions(t *testing.T) {
	hub := NewMapHub()
	client := &MapClient{Send: make(chan []byte, 1)}
	hub.Join(client)
	hub.ReplaceSubscriptions(client, []string{"cn:6:10:20"})
	hub.ReplaceSubscriptions(client, []string{"cn:6:10:21"})

	hub.BroadcastToChunk("cn:6:10:20", []byte(`old`))
	select {
	case <-client.Send:
		t.Fatal("client should be removed from old chunk subscription")
	default:
	}

	hub.BroadcastToChunk("cn:6:10:21", []byte(`new`))
	select {
	case <-client.Send:
	default:
		t.Fatal("client should receive new chunk subscription")
	}
}

func TestMapHubLeaveCleansSubscriptions(t *testing.T) {
	hub := NewMapHub()
	client := &MapClient{Send: make(chan []byte, 1)}
	hub.Join(client)
	hub.ReplaceSubscriptions(client, []string{"cn:6:10:20", "cn:6:10:21"})
	hub.Leave(client)

	if got := hub.ClientSubscriptions(client); len(got) != 0 {
		t.Fatalf("subscriptions should be cleaned after leave, got %v", got)
	}
	hub.BroadcastToChunk("cn:6:10:20", []byte(`message`))
	select {
	case _, ok := <-client.Send:
		if ok {
			t.Fatal("left client should not receive broadcast")
		}
	default:
		t.Fatal("left client send channel should be closed")
	}
}
