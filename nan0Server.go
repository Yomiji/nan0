package nan0


// The Nan0Server structure is a wrapper around a service which allows
// for the acceptance of connections which are wrapped automatically
// in Nan0 objects. This allows for the communication of protocol buffers
// along channels for each connection.
type Nan0Server struct {
	// The name of the service
	service *Service
	// Each new connection received gets pushed to this channel, wrapped in a Nan0
	newConnections chan NanoServiceWrapper
	// Connections array, which keeps connected clients
	connections []NanoServiceWrapper
	// The closed status
	closed bool
	// Listener shutdown
	listenerShutdown chan bool
	// Shutdown confirmation
	confirmShutdown chan bool
}

// Exposes the service delegate's serviceName property
func (server Nan0Server) GetServiceName() string {
	return server.service.ServiceName
}

// Exposes the service delegate's serviceType property
func (server Nan0Server) GetServiceType() string {
	return server.service.ServiceType
}

// Exposes the service delegate's startTime property
func (server Nan0Server) GetStartTime() int64 {
	return server.service.StartTime
}

// Exposes the service delegate's hostName property
func (server Nan0Server) GetHost() string {
	return server.service.HostName
}

// Exposes the service delegate's port property
func (server Nan0Server) GetPort() int32 {
	return server.service.Port
}

// Exposes the IsExpired method of the service delegate
func (server Nan0Server) IsExpired() bool {
	return server.service.IsExpired()
}

// Exposes the IsAlive method of the service delegate
func (server Nan0Server) IsAlive() bool {
	return server.service.IsAlive()
}

// Get the channel which is fed new connections to the server
func (server *Nan0Server) GetConnections() <-chan NanoServiceWrapper {
	return server.newConnections
}

// Get all connections that this service has ever opened
func (server *Nan0Server) GetAllConnections() []NanoServiceWrapper {
	return server.connections
}

// Puts a connection in the server
func (server *Nan0Server) AddConnection(conn NanoServiceWrapper) {
	server.connections = append(server.connections, conn)
	server.newConnections <- conn
}

// Close all opened connections and clear connection cache
func (server *Nan0Server) ResetConnections() (total int) {
	total = len(server.connections)
	for _,conn := range server.connections {
		if conn != nil {
			conn.Close()
		}
	}
	server.connections = make([]NanoServiceWrapper, MaxNanoCache)
	return
}

func (server Nan0Server) IsShutdown() bool {
	return server.closed
}

// Shut down the server
func (server *Nan0Server) Shutdown() {
	if server.IsShutdown() {
		debug("Service already closed when calling Shutdown() on %v", server.service.ServiceName)
		return
	}
	server.closed = true

	debug("Shutting down server %v", server.service.ServiceName)

	// stop the listener and goroutine
	server.listenerShutdown <- true
	<-server.confirmShutdown
}
