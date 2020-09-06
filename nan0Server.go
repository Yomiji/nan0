package nan0

import (
	"container/list"
	"context"
	"errors"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/yomiji/mdns"
	"github.com/yomiji/slog"
)

// The NanoServer structure is a wrapper around a service which allows
// for the acceptance of connections which are wrapped automatically
// in Nan0 objects. This allows for the communication of protocol buffers
// along channels for each connection.
type NanoServer struct {
	// The name of the service
	service              *Service
	connectionStreamInit bool
	// Each new connection received gets pushed to this list, wrapped in a Nan0
	connectionsList  *connList
	allConnectionsList *connList
	// The closed status
	closed              chan struct{}
	listener            net.Listener
	wsServer            *http.Server
	mdnsServer          *mdns.Server
}

func (server NanoServer) ActiveConnectionsCount() int {
	server.allConnectionsList.rwMux.RLock()
	defer server.allConnectionsList.rwMux.RUnlock()
	return server.allConnectionsList.innerList.Len()
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

type connList struct {
	innerList *list.List
	rwMux  *sync.RWMutex
}

func (cl *connList) init() {
	cl.innerList = new(list.List)
	cl.rwMux = new(sync.RWMutex)
}

func (cl *connList) purgeClosedConnections() {
	cl.rwMux.Lock()
	defer cl.rwMux.Unlock()
	if cl.innerList.Len() == 0 {
		return
	}
	l := cl.innerList
	k := new(list.List)
	for e := l.Front(); e != nil; e = e.Next() {
		if v,ok := e.Value.(NanoServiceWrapper); ok {
			if !v.IsClosed() {
				k.PushBack(v)
			}
		}
	}
	cl.innerList = k
}

func (cl connList) dumpConnListToSlice() []NanoServiceWrapper {
	cl.rwMux.RLock()
	defer cl.rwMux.RUnlock()
	if cl.innerList.Len() == 0 {
		return nil
	}
	l := cl.innerList
	slice := make([]NanoServiceWrapper, 0)
	for e := l.Front(); e != nil; e = e.Next() {
		if v,ok := e.Value.(NanoServiceWrapper); ok {
			slice = append(slice, v)
		}
	}
	return slice
}

func (cl connList) readConnListBack() *list.Element {
	cl.rwMux.RLock()
	defer cl.rwMux.RUnlock()
	return cl.innerList.Back()
}

func (cl *connList) clearConnList() {
	cl.rwMux.Lock()
	defer cl.rwMux.Unlock()
	cl.innerList.Init()
}

func (cl *connList) writeConnListElement(conn NanoServiceWrapper) {
	cl.rwMux.Lock()
	defer cl.rwMux.Unlock()
	cl.innerList.PushBack(conn)
}

func (cl *connList) removeConnListElement(element *list.Element) interface{} {
	cl.rwMux.Lock()
	defer cl.rwMux.Unlock()
	return cl.innerList.Remove(element)
}

// Get the channel which is fed new connections to the server
func (server *NanoServer) GetConnections() <-chan NanoServiceWrapper {
	if server.IsShutdown() {
		return nil
	}
	if server.connectionStreamInit {
		panic(errors.New("the method GetConnections can only be called once per server"))
	}
	server.connectionStreamInit = true
	c := make(chan NanoServiceWrapper)
	go func() {
		element := server.connectionsList.readConnListBack()
		ticker := time.NewTicker(1 * time.Millisecond)
		defer ticker.Stop()
		for ; !server.IsShutdown(); {
			<-ticker.C
			if element == nil {
				element = server.connectionsList.readConnListBack()
				continue
			}
			if element.Value != nil {
				value := server.connectionsList.removeConnListElement(element)
				if nsw, ok := value.(NanoServiceWrapper); ok {
					c <- nsw
				}
			}
			if element.Next() != nil {
				element = element.Next()
			} else {
				element = nil
			}
		}
		slog.Debug("Shutdown connection stream routine")
	}()
	return c
}

// Get all connections that this service has ever opened
func (server *NanoServer) GetAllConnections() []NanoServiceWrapper {
	if server.IsShutdown() {
		return nil
	}
	return server.allConnectionsList.dumpConnListToSlice()
}

// Puts a connection in the server
func (server *NanoServer) AddConnection(conn NanoServiceWrapper) {
	if server.IsShutdown() {
		return
	}
	server.connectionsList.writeConnListElement(conn)
	server.allConnectionsList.writeConnListElement(conn)
}

// Close all opened connections and clear connection cache
func (server *NanoServer) resetConnections() (total int) {
	for _, conn := range server.GetAllConnections() {
		if conn != nil && !conn.IsClosed() {
			conn.Close()
		}
	}
	server.connectionsList.clearConnList()
	server.allConnectionsList.clearConnList()
	return
}

func (server *NanoServer) IsShutdown() bool {
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
