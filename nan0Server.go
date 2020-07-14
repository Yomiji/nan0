package nan0

import (
	"net"
	"net/http"
	"sync"

	"github.com/yomiji/mdns"
	"github.com/yomiji/slog"
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
	closed        bool
	listener      net.Listener
	wsServer      *http.Server
	rxTxWaitGroup *sync.WaitGroup
	mdnsServer    *mdns.Server
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
	server.rxTxWaitGroup.Wait()
	if server.IsShutdown() {
		return true
	}
	return server.service.IsExpired()
}

// Exposes the IsAlive method of the service delegate
func (server NanoServer) IsAlive() bool {
	server.rxTxWaitGroup.Wait()
	if server.IsShutdown() {
		return false
	}
	return server.service.IsAlive()
}

// Get the channel which is fed new connections to the server
func (server *NanoServer) GetConnections() <-chan NanoServiceWrapper {
	server.rxTxWaitGroup.Wait()
	if server.IsShutdown() {
		return nil
	}
	return server.newConnections
}

// Get all connections that this service has ever opened
func (server *NanoServer) GetAllConnections() []NanoServiceWrapper {
	server.rxTxWaitGroup.Wait()
	if server.IsShutdown() {
		return nil
	}
	return server.connections
}

// Puts a connection in the server
func (server *NanoServer) AddConnection(conn NanoServiceWrapper) {
	server.rxTxWaitGroup.Wait()
	if server.IsShutdown() {
		return
	}
	server.connections = append(server.connections, conn)
	server.newConnections <- conn
}

// Close all opened connections and clear connection cache
func (server *NanoServer) resetConnections() (total int) {
	total = len(server.connections)
	for _, conn := range server.connections {
		if conn != nil && !conn.IsClosed() {
			conn.Close()
		}
	}
	server.connections = make([]NanoServiceWrapper, MaxNanoCache)
	return
}

func (server *NanoServer) IsShutdown() bool {
	return server.closed
}

func (server *NanoServer) Shutdown() {
	server.rxTxWaitGroup.Add(1)
	defer server.rxTxWaitGroup.Done()
	recoverPanic(func(e error) {
		slog.Fail("In shutdown of server, %s: %v", server.GetServiceName(), e)
	})
	server.resetConnections()

	if server.listener != nil {
		err := server.listener.Close()
		checkError(err)
	}
	if server.wsServer != nil {
		err := server.wsServer.Close()
		checkError(err)
	}
	if server.mdnsServer != nil {
		err := server.mdnsServer.Shutdown()
		checkError(err)
	}
	server.closed = true
}
