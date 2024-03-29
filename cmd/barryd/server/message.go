package server

import (
	"errors"
	"fmt"
	"time"

	"github.com/fatih/color"
)

// Messages are the glue between the client and the server

// SUCCESS & FAILURE will end a client connection (no?)
const (
	MessageSuccess = "SUCCESS"
	MessageFailure = "FAILURE"
)

// Message types
const (
	MessageError   = "ERROR"
	MessageWarning = "WARNING"
	MessageInfo    = "INFO"
	MessageTrace   = "TRACE"
	MessageNoop    = "NOOP" // MessageNoop is used for keep-alive messages
)

// Messages generic topics
const (
	MsgGlob = ".GLOBAL"
)

// Message.Print options
const (
	MessagePrintTime    = true
	MessagePrintNoTime  = false
	MessagePrintTopic   = true
	MessagePrintNoTopic = false
)

// Message describe a message between client and server
type Message struct {
	Time    time.Time `json:"time"`
	Type    string    `json:"type"`
	Topic   string    `json:"topic"`
	Message string    `json:"message"`
}

// NewMessage creates a new Message instance
func NewMessage(mtype string, topic string, message string) *Message {
	return &Message{
		Time:    time.Now(),
		Type:    mtype,
		Topic:   topic,
		Message: message,
	}
}

// MatchTarget returns true if the message matches the topic (or is global)
func (message *Message) MatchTarget(topic string) bool {
	if topic == message.Topic {
		return true
	}

	if topic == MsgGlob {
		return true
	}

	return false
}

// Print the formatted message
func (message *Message) Print(showTime bool, showTopic bool) error {
	var retError error

	// the longest types are 7 chars wide
	mtype := fmt.Sprintf("% -7s", message.Type)
	content := message.Message

	switch message.Type {
	case MessageTrace:
		c := color.New(color.FgWhite).SprintFunc()
		content = c(content)
		mtype = c(mtype)
	case MessageInfo:
	case MessageWarning:
		c := color.New(color.FgYellow).SprintFunc()
		content = c(content)
		mtype = c(mtype)
	case MessageError:
		c := color.New(color.FgRed).SprintFunc()
		content = c(content)
		mtype = c(mtype)
	case MessageFailure:
		retError = errors.New("exiting with failure status due to previous errors")
		c := color.New(color.FgHiRed).SprintFunc()
		content = c(content)
		mtype = c(mtype)
	case MessageSuccess:
		c := color.New(color.FgHiGreen).SprintFunc()
		content = c(content)
		mtype = c(mtype)
	}

	time := ""
	if showTime {
		time = message.Time.Format("15:04:05") + " "
	}

	topic := ""
	if showTopic {
		topic = "[" + message.Topic + "] "
	}

	fmt.Printf("%s%s%s: %s\n", time, topic, mtype, content)

	return retError
}
