package nan0

import (
	"net"
	"net/http"
	"sync"
)

// The NanoServer structure is a wrapper around a service which allows
// for the acceptance of connections which are wrapped automatically
// in Nan0 objects. This allows for the communication of protocol buffers
// along channels for each connection.
type NanoServer struct {
	// The name of the service
	service *Service
	// Each new connection received gets pushed to this channel, wrapped in a Nan0
	newConnections chan NanoServiceWrapper
	// Connections array, which keeps connected clients
	connections []NanoServiceWrapper
	// The closed status
	closed bool
  listener net.Listener
	wsServer *http.Server
	mutex sync.Mutex
}

// Exposes the service delegate's serviceName property
func (server NanoServer) GetServiceName() string {
	return server.service.ServiceName
}

// Exposes the service delegate's serviceType property
func (server NanoServer) GetServiceType() string {
	return server.service.ServiceType
}

// Exposes the service delegate's startTime property
func (server NanoServer) GetStartTime() int64 {
	return server.service.StartTime
}

// Exposes the service delegate's hostName property
func (server NanoServer) GetHost() string {
	return server.service.HostName
}

// Exposes the service delegate's port property
func (server NanoServer) GetPort() int32 {
	return server.service.Port
}

// Exposes the IsExpired method of the service delegate
func (server NanoServer) IsExpired() bool {
	if server.IsShutdown() {
		return true
	}
	return server.service.IsExpired()
}

// Exposes the IsAlive method of the service delegate
func (server NanoServer) IsAlive() bool {
	if server.IsShutdown() {
		return false
	}
	return server.service.IsAlive()
}

// Get the channel which is fed new connections to the server
func (server *NanoServer) GetConnections() <-chan NanoServiceWrapper {
	if server.IsShutdown() {
		return nil
	}
	return server.newConnections
}

// Get all connections that this service has ever opened
func (server *NanoServer) GetAllConnections() []NanoServiceWrapper {
	if server.IsShutdown() {
		return nil
	}
	return server.connections
}

// Puts a connection in the server
func (server *NanoServer) AddConnection(conn NanoServiceWrapper) {
	if server.IsShutdown() {
		return
	}
	server.connections = append(server.connections, conn)
	server.newConnections <- conn
}

// Close all opened connections and clear connection cache
func (server *NanoServer) ResetConnections() (total int) {
	if server.IsShutdown() {
		return 0
	}
	total = len(server.connections)
	for _,conn := range server.connections {
		if conn != nil  && !conn.IsClosed() {
			conn.Close()
		}
	}
	server.connections = make([]NanoServiceWrapper, MaxNanoCache)
	return
}

func (server *NanoServer) IsShutdown() bool {
	server.mutex.Lock()
	defer	server.mutex.Unlock()
	c := server.closed
	return c
}
func (server *NanoServer) Shutdown() {
	recoverPanic(func(e error) {
		fail("In shutdown of server, %s: %v",server.GetServiceName(), e)
	})
	server.ResetConnections()

	if server.listener != nil {
		err := server.listener.Close()
		checkError(err)
	}
	if server.wsServer != nil {
		err := server.wsServer.Close()
		checkError(err)
	}
	server.mutex.Lock()
	server.closed = true
	server.mutex.Unlock()
}