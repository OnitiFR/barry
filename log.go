package main

import (
	"fmt"
)

// Log host logs for the application
type Log struct {
	history *LogHistory
	trace   bool
}

// NewLog creates a new Log
func NewLog(trace bool, history *LogHistory) *Log {
	return &Log{
		history: history,
		trace:   trace,
	}
}

// Log is a low-level function for sending a Message
func (log *Log) Log(message *Message) {

	if !(message.Type == MessageTrace && log.trace == false) {
		fmt.Printf("%s: %s\n", message.Type, message.Message)
	}

	// we don't historize NOOP and TRACE messages
	if message.Type != MessageNoop && message.Type != MessageTrace {
		log.history.Push(message)
	}
}

// Error sends a MessageError Message
func (log *Log) Error(topic, message string) {
	log.Log(NewMessage(MessageError, topic, message))
}

// Errorf sends a formated string MessageError Message
func (log *Log) Errorf(topic, format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)
	log.Error(msg, topic)
}

// Warning sends a MessageWarning Message
func (log *Log) Warning(topic, message string) {
	log.Log(NewMessage(MessageWarning, topic, message))
}

// Warningf sends a formated string MessageWarning Message
func (log *Log) Warningf(topic, format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)
	log.Warning(msg, topic)
}

// Info sends an MessageInfo Message
func (log *Log) Info(topic, message string) {
	log.Log(NewMessage(MessageInfo, topic, message))
}

// Infof sends a formated string MessageInfo Message
func (log *Log) Infof(topic, format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)
	log.Info(topic, msg)
}

// Trace sends an MessageTrace Message
func (log *Log) Trace(topic, message string) {
	log.Log(NewMessage(MessageTrace, topic, message))
}

// Tracef sends a formated string MessageTrace Message
func (log *Log) Tracef(topic, format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)
	log.Trace(topic, msg)
}

// Success sends an MessageSuccess Message
func (log *Log) Success(topic, message string) {
	log.Log(NewMessage(MessageSuccess, topic, message))
}

// Successf sends a formated string MessageSuccess Message
func (log *Log) Successf(topic, format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)
	log.Success(topic, msg)
}

// Failure sends an MessageFailure Message
func (log *Log) Failure(topic, message string) {
	log.Log(NewMessage(MessageFailure, topic, message))
}

// Failuref sends a formated string MessageFailure Message
func (log *Log) Failuref(topic, format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)
	log.Failure(topic, msg)
}
