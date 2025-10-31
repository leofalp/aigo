package memory

import "aigo/providers/ai"

type Provider interface {
	AppendMessage(message *ai.Message)
	GetMessages() []ai.Message
	ClearMessages()
}
