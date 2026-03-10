package channels

import (
	"context"
	"sync"

	"github.com/charmbracelet/log"
	"github.com/google/uuid"
	"github.com/konglong87/wukongbot/internal/bus"
)

// Manager manages all communication channels
type Manager struct {
	mu       sync.RWMutex
	channels map[string]BaseChannel
	bus      bus.MessageBus
}

// NewManager creates a new channel manager
func NewManager(bus bus.MessageBus) *Manager {
	return &Manager{
		channels: make(map[string]BaseChannel),
		bus:      bus,
	}
}

// Register adds a channel to the manager
func (m *Manager) Register(channel BaseChannel) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.channels[channel.Name()] = channel
	log.Info("Registered channel", "name", channel.Name())
}

// Unregister removes a channel from the manager
func (m *Manager) Unregister(name string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.channels, name)
}

// Get retrieves a channel by name
func (m *Manager) Get(name string) (BaseChannel, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	channel, ok := m.channels[name]
	return channel, ok
}

// StartAll starts all registered channels
func (m *Manager) StartAll(ctx context.Context) error {
	m.mu.RLock()
	defer m.mu.RUnlock()

	for name, channel := range m.channels {
		if err := channel.Start(ctx); err != nil {
			log.Error("Failed to start channel", "name", name, "error", err)
			continue
		}
		log.Info("Started channel", "name", name)
	}
	return nil
}

// StopAll stops all registered channels
func (m *Manager) StopAll(ctx context.Context) error {
	m.mu.RLock()
	defer m.mu.RUnlock()

	for name, channel := range m.channels {
		if err := channel.Stop(ctx); err != nil {
			log.Error("Failed to stop channel", "name", name, "error", err)
			continue
		}
		log.Info("Stopped channel", "name", name)
	}
	return nil
}

// RouteMessages routes messages from all channels to the bus
func (m *Manager) RouteMessages(ctx context.Context) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	for _, channel := range m.channels {
		go func(ch BaseChannel) {
			for {
				select {
				case <-ctx.Done():
					return
				case msg, ok := <-ch.Receive():
					if !ok {
						return
					}
					log.Info("[RouteMessages] Received message from channel", "channel", ch.Name(), "message", msg)
					if !ch.IsAllowed(msg.SenderID) {
						log.Debug("Blocked message from unauthorized sender", "sender", msg.SenderID)
						continue
					}

					// Convert Media from channels package to bus package
					media := make([]bus.Media, len(msg.Media))
					for i, m := range msg.Media {
						media[i] = bus.Media{
							Type:     m.Type,
							URL:      m.URL,
							Data:     m.Data,
							MimeType: m.MimeType,
						}
					}

					inbound := bus.InboundMessage{
						ChannelID: ch.Type(),
						SenderID:  msg.SenderID,
						Content:   msg.Content,
						Timestamp: msg.Timestamp,
						Metadata:  msg.Metadata,
						Media:     media,
					}

					// Generate traceId if not present
					if inbound.TraceId == "" {
						inbound.TraceId = uuid.New().String()
					}

					if err := m.bus.PublishInbound(ctx, inbound); err != nil {
						log.Error("Failed to publish message", "error", err)
					}
				}
			}
		}(channel)
	}
}

// List returns all channel names
func (m *Manager) List() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	names := make([]string, 0, len(m.channels))
	for name := range m.channels {
		names = append(names, name)
	}
	return names
}
