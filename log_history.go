package main

import (
	"fmt"
	"sync"
)

// maximum message length (message is truncated if too long)
const logHistoryMaxMessageLen = 256

type logHistorySlot struct {
	payload *Message
	older   *logHistorySlot
	newer   *logHistorySlot
}

// LogHistory stores messages in a limited size double chain list
type LogHistory struct {
	maxSize     int
	currentSize int
	oldest      *logHistorySlot
	newest      *logHistorySlot
	mux         sync.Mutex
}

// NewLogHistory will create and initialize a new log message history
func NewLogHistory(elems int) *LogHistory {
	return &LogHistory{
		maxSize: elems,
	}
}

// Push a new message in the list
func (lh *LogHistory) Push(message *Message) {
	lh.mux.Lock()
	defer lh.mux.Unlock()

	localMsg := message

	// truncate message if needed
	if len(message.Message) > logHistoryMaxMessageLen {
		dup := *message
		dup.Message = dup.Message[:logHistoryMaxMessageLen] + "…"
		localMsg = &dup
	}

	curr := &logHistorySlot{
		payload: localMsg,
	}

	if lh.currentSize == 0 {
		lh.newest = curr
		lh.oldest = curr
		lh.currentSize++
		return
	}

	// place "curr" in front
	lh.newest.newer = curr
	curr.older = lh.newest
	lh.newest = curr
	lh.currentSize++

	// remove the oldest slot
	if lh.currentSize > lh.maxSize {
		lh.oldest = lh.oldest.newer
		lh.oldest.older = nil
		lh.currentSize--
	}

}

// Dump all logs in the buffer (temporary test)
func (lh *LogHistory) Dump() {
	lh.mux.Lock()
	defer lh.mux.Unlock()

	fmt.Println(lh.currentSize)
	curr := lh.newest
	for curr != nil {
		fmt.Println(curr.payload)
		curr = curr.older
	}
}

// Search return an array of messages (latest messages, up to maxMessages, for a specific topic)
func (lh *LogHistory) Search(maxMessages int, topic string) []*Message {
	lh.mux.Lock()
	defer lh.mux.Unlock()

	reversedMessages := make([]*Message, maxMessages)

	curr := lh.newest
	count := 0
	for curr != nil && count < maxMessages {
		if curr.payload.MatchTarget(topic) == false {
			curr = curr.older
			continue
		}
		reversedMessages[count] = curr.payload
		count++
		curr = curr.older
	}

	// reverse the array
	messages := make([]*Message, count)
	for i := 0; i < count; i++ {
		messages[i] = reversedMessages[count-i-1]
	}
	return messages
}
