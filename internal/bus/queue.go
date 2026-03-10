package bus

import (
	"context"
	"fmt"
	"time"

	"github.com/charmbracelet/log"
)

// ChannelMessageBus implements the MessageBus interface using Go channels
type ChannelMessageBus struct {
	inboundChan  chan InboundMessage
	outboundChan chan OutboundMessage
	bufferSize   int
}

// NewChannelMessageBus creates a new channel-based message bus
func NewChannelMessageBus(bufferSize int) *ChannelMessageBus {
	return &ChannelMessageBus{
		inboundChan:  make(chan InboundMessage, bufferSize),
		outboundChan: make(chan OutboundMessage, bufferSize),
		bufferSize:   bufferSize,
	}
}

// PublishInbound publishes an inbound message to the bus
func (b *ChannelMessageBus) PublishInbound(ctx context.Context, msg InboundMessage) error {
	log.Debug("ChannelMessageBus PublishInbound", "msg", msg)
	select {
	case b.inboundChan <- msg:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(5 * time.Second):
		return fmt.Errorf("timeout publishing inbound message: channel buffer full")
	}
}

// ConsumeInbound retrieves an inbound message from the bus
func (b *ChannelMessageBus) ConsumeInbound(ctx context.Context) (InboundMessage, error) {
	select {
	case msg := <-b.inboundChan:
		log.Debug("ChannelMessageBus ConsumeInbound", "msg", msg)
		return msg, nil
	case <-ctx.Done():
		return InboundMessage{}, ctx.Err()
	}
}

// PublishOutbound publishes an outbound message to the bus
func (b *ChannelMessageBus) PublishOutbound(ctx context.Context, msg OutboundMessage) error {
	log.Debug("ChannelMessageBus PublishOutbound 2", "msg", msg)
	select {
	case b.outboundChan <- msg:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(5 * time.Second):
		return fmt.Errorf("timeout publishing outbound message: channel buffer full")
	}
}

// ConsumeOutbound retrieves an outbound message from the bus
func (b *ChannelMessageBus) ConsumeOutbound(ctx context.Context) (OutboundMessage, error) {
	select {
	case msg := <-b.outboundChan:
		log.Debug("ChannelMessageBus ConsumeOutbound", "msg", msg)
		return msg, nil
	case <-ctx.Done():
		return OutboundMessage{}, ctx.Err()
	}
}

// Close closes the message bus and releases resources
func (b *ChannelMessageBus) Close() error {
	close(b.inboundChan)
	close(b.outboundChan)
	return nil
}

// InboundChannel returns the inbound channel for direct access
func (b *ChannelMessageBus) InboundChannel() <-chan InboundMessage {
	return b.inboundChan
}

// OutboundChannel returns the outbound channel for direct access
func (b *ChannelMessageBus) OutboundChannel() <-chan OutboundMessage {
	return b.outboundChan
}
