package nan0

import (
	"context"
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
	closed      chan struct{}
	shutdownMux *sync.Mutex
	listener    net.Listener
	wsServer    *http.Server
	mdnsServer  *mdns.Server
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

func (server NanoServer) MdnsTag() string {
	return server.service.MdnsTag()
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
	server.shutdownMux.Lock()
	defer server.shutdownMux.Unlock()
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
	close(server.newConnections)
	server.connections = make([]NanoServiceWrapper, MaxNanoCache)
	return
}

func (server *NanoServer) IsShutdown() bool {
	server.shutdownMux.Lock()
	defer server.shutdownMux.Unlock()
	select {
	case <-server.closed:
		return true
	default:
		return false
	}
}

func (server *NanoServer) Shutdown() {
	defer recoverPanic(func(e error) {
		slog.Fail("In shutdown of server, %s: %v", server.GetServiceName(), e)
	})
	server.shutdownMux.Lock()
	defer server.shutdownMux.Unlock()
	server.resetConnections()

	if server.listener != nil {
		err := server.listener.Close()
		checkError(err)
	}
	if server.wsServer != nil {
		err := server.wsServer.Close()
		checkError(err)
		err = server.wsServer.Shutdown(context.Background())
		checkError(err)
	}
	if server.mdnsServer != nil {
		err := server.mdnsServer.Shutdown()
		checkError(err)
	}
	close(server.closed)
}
