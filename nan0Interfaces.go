package nan0

import (
	"google.golang.org/protobuf/proto"
)

// Any object which wraps a connection to send and receive different protocol buffers
type NanoServiceWrapper interface {
	softClose()
	startServiceSender(map[string]int, bool,*[32]byte,*[32]byte)
	startServiceReceiver(RouteMap, map[int]proto.Message,*[32]byte,*[32]byte)
	Closer
	TxRx
	Identity
	Equality
}

// Close the wrapper goroutines and the underlying connections
// Check if the wrapper is closed
type Closer interface {
	Close()
	IsClosed() bool
}

// Transmit and Receive
type TxRx interface {
	Sender
	Receiver
}

// Return the sender channel for this nanoservice wrapper, messages are sent along this route
type Sender interface {
	GetSender() chan<- interface{}
}

// Return the receiver channel for this nanoservice wrapper, messages are received from this
type Receiver interface {
	GetReceiver() <-chan interface{}
}

// Determine if two instances are equal
type Equality interface {
	Equals(NanoServiceWrapper) bool
}

// Get the name of a service or client
type Identity interface {
	GetServiceName() string
}

// Represents a server object with live-ness and registry to a discovery mechanism
type Server interface {
	Identity
	ServiceLifecycle
	ConnectionHandler
	DiscoverableService
}

// Handles shutdown procedure
type ServiceLifecycle interface {
	IsShutdown() bool
	Shutdown()
}

// Handles connections established
type ConnectionHandler interface {
	GetConnections() <-chan NanoServiceWrapper
	AddConnection(NanoServiceWrapper)
}

// Generates discovery tags
type DiscoverableService interface {
	MdnsTag() string
}

type RouteMap map[string]ExecutableRoute

type ExecutableRoute interface {
	Execute(msg proto.Message, sender chan<- interface{})
}