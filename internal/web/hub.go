package web

import (
	"context"
	"sync"

	"github.com/rs/zerolog/log"
)

type Hub struct {
	clients    map[chan []byte]struct{}
	broadcast  chan []byte
	register   chan chan []byte
	unregister chan chan []byte
	mu         sync.RWMutex
}

func NewHub() *Hub {
	return &Hub{
		clients:    make(map[chan []byte]struct{}),
		broadcast:  make(chan []byte),
		register:   make(chan chan []byte),
		unregister: make(chan chan []byte),
	}
}

func (h *Hub) Run(ctx context.Context) {
	log.Info().Str("stage", "web_hub").Msg("hub started")
	for {
		select {
		case <-ctx.Done():
			return
		case client := <-h.register:
			h.mu.Lock()
			h.clients[client] = struct{}{}
			h.mu.Unlock()
			log.Debug().Str("stage", "web_hub").Msg("client registered")
		case client := <-h.unregister:
			h.mu.Lock()
			if _, ok := h.clients[client]; ok {
				delete(h.clients, client)
				close(client)
			}
			h.mu.Unlock()
			log.Debug().Str("stage", "web_hub").Msg("client unregistered")
		case message := <-h.broadcast:
			h.mu.RLock()
			for client := range h.clients {
				select {
				case client <- message:
				default:
					log.Warn().Str("stage", "web_hub").Msg("client channel full, dropping message")
				}
			}
			h.mu.RUnlock()
		}
	}
}

func (h *Hub) Broadcast(msg []byte) {
	h.broadcast <- msg
}

func (h *Hub) Register(ch chan []byte) {
	h.register <- ch
}

func (h *Hub) Unregister(ch chan []byte) {
	h.unregister <- ch
}
