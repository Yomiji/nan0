package nan0

import (
	"github.com/golang/protobuf/proto"
)

// Any object which wraps a connection to send and receive different protocol buffers
type NanoServiceWrapper interface {
	startServiceSender(map[string]int, bool)
	startServiceReceiver(map[int]proto.Message)

	// Close the wrapper goroutines and the underlying connections
	Close()
	// Check if the wrapper is closed
	IsClosed() bool
	// Return the sender channel for this nanoservice wrapper, messages are sent along this route
	GetSender() chan<- interface{}
	// Return the receiver channel for this nanoservice wrapper, messages are received from this
	GetReceiver() <-chan interface{}
	// Get the service name identifier
	GetServiceName() string
	// Determine if two instances are equal
	Equals(NanoServiceWrapper) bool
}

// Represents a server object with live-ness and registry to a discovery mechanism
type Server interface {
	IsShutdown() bool
	Shutdown()
	GetServiceName() string
	GetConnections() <-chan NanoServiceWrapper
	AddConnection(NanoServiceWrapper)
}