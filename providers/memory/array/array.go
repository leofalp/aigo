package array

import "aigo/providers/ai"

type ArrayMemory struct {
	messages []ai.Message
}

func NewArrayMemory() *ArrayMemory {
	return &ArrayMemory{
		messages: []ai.Message{},
	}
}

func (m *ArrayMemory) AppendMessage(message *ai.Message) {
	if message != nil {
		m.messages = append(m.messages, *message)
	}
}

func (m *ArrayMemory) GetMessages() []ai.Message {
	return m.messages
}

func (m *ArrayMemory) ClearMessages() {
	m.messages = []ai.Message{}
}
