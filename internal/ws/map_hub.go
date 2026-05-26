package ws

import "sync"

type MapHub struct {
	mu      sync.RWMutex
	chunks  map[string]map[*MapClient]bool
	clients map[*MapClient]map[string]bool
}

func NewMapHub() *MapHub {
	return &MapHub{
		chunks:  make(map[string]map[*MapClient]bool),
		clients: make(map[*MapClient]map[string]bool),
	}
}

func (h *MapHub) Join(client *MapClient) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.clients[client] == nil {
		h.clients[client] = make(map[string]bool)
	}
}

func (h *MapHub) Leave(client *MapClient) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.removeClientLocked(client)
	client.closeSend()
}

func (h *MapHub) ReplaceSubscriptions(client *MapClient, chunkIDs []string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.removeClientSubscriptionsLocked(client)
	next := make(map[string]bool, len(chunkIDs))
	for _, chunkID := range chunkIDs {
		if next[chunkID] {
			continue
		}
		next[chunkID] = true
		if h.chunks[chunkID] == nil {
			h.chunks[chunkID] = make(map[*MapClient]bool)
		}
		h.chunks[chunkID][client] = true
	}
	h.clients[client] = next
}

func (h *MapHub) BroadcastChunkEvent(event ChunkEvent) {
	data, err := EncodeChunkEvent(event)
	if err != nil {
		return
	}
	h.BroadcastToChunk(event.ChunkID, data)
}

func (h *MapHub) BroadcastToChunk(chunkID string, message []byte) {
	h.mu.Lock()
	defer h.mu.Unlock()
	clients := h.chunks[chunkID]
	if clients == nil {
		return
	}
	for client := range clients {
		select {
		case client.Send <- message:
		default:
			h.removeClientLocked(client)
			client.closeSend()
		}
	}
}

func (h *MapHub) ClientSubscriptions(client *MapClient) []string {
	h.mu.RLock()
	defer h.mu.RUnlock()
	subscriptions := h.clients[client]
	result := make([]string, 0, len(subscriptions))
	for chunkID := range subscriptions {
		result = append(result, chunkID)
	}
	return result
}

func (h *MapHub) removeClientLocked(client *MapClient) {
	h.removeClientSubscriptionsLocked(client)
	delete(h.clients, client)
}

func (h *MapHub) removeClientSubscriptionsLocked(client *MapClient) {
	for chunkID := range h.clients[client] {
		clients := h.chunks[chunkID]
		if clients == nil {
			continue
		}
		delete(clients, client)
		if len(clients) == 0 {
			delete(h.chunks, chunkID)
		}
	}
}
