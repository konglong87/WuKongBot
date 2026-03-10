package toolcontext

// MessageBusAdapter wraps bus.MessageBus to satisfy toolcontext.MessageSender interface
type MessageBusAdapter struct {
	SendFunc func(channelID, senderID, content string) error
}

// SendOutbound implements toolcontext.MessageSender
func (a *MessageBusAdapter) SendOutbound(channelID, senderID, content string) error {
	return a.SendFunc(channelID, senderID, content)
}
