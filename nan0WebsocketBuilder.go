package nan0

import (
	"context"
	"net/http"
	"net/url"
	"sync"

	"github.com/yomiji/mdns"
	"github.com/yomiji/slog"
	"github.com/yomiji/websocket"
)

type TLSConfig struct {
	CertFile string
	KeyFile  string
}

type WebsocketBuilder struct {
	websocketFlag bool
	origin        string
	origins       []*url.URL
	tls           *TLSConfig
	*baseBuilder
}

// Enables secure websocket handling for this connection
func (wsb *WebsocketBuilder) Secure(config *TLSConfig) baseBuilderOption {
	return func(_ *baseBuilder) {
		wsb.tls = config
	}
}

// Adds origin checks to websocket handler (no use for clients)
func (wsb *WebsocketBuilder) AddOrigins(origin ...string) baseBuilderOption {
	return func(_ *baseBuilder) {
		for _, v := range origin {
			u, err := url.Parse(v)
			if err == nil {
				wsb.origins = append(wsb.origins, u)
			}
		}
	}
}

// Adds origin checks to websocket handler (no use for clients)
func (wsb *WebsocketBuilder) AppendOrigins(origin ...*url.URL) baseBuilderOption {
	return func(_ *baseBuilder) {
		wsb.origins = append(wsb.origins, origin...)
	}
}

func (wsb *WebsocketBuilder) BuildWebsocketClient(opts ...baseBuilderOption) (nan0 NanoServiceWrapper, err error) {
	wsb.build(opts...)
	return buildWebsocketClient(wsb.baseBuilder)
}

func (wsb *WebsocketBuilder) BuildWebsocketDNS(ctx context.Context, strategy clientDNSStrategy, opts ...baseBuilderOption) ClientDNSFactory {
	wsb.build(opts...)
	return BuildDNS(ctx, wsb.baseBuilder, buildWebsocketClient, strategy)
}

func (wsb *WebsocketBuilder) BuildWebsocketServer(opts ...baseBuilderOption) (*NanoServer, error) {
	wsb.build(opts...)
	return buildWebsocketServer(wsb)
}

func wrapConnectionWs(connection *websocket.Conn, bb *baseBuilder) (nan0 NanoServiceWrapper, err error) {
	defer recoverPanic(func(e error) {
		nan0 = nil
		err = e.(error)
	})()

	nan0 = &WsNan0{
		ServiceName:    bb.ns.ServiceName,
		receiver:       makeReceiveChannelFromBuilder(bb),
		sender:         makeSendChannelFromBuilder(bb),
		conn:           connection,
		closed:         false,
		writerShutdown: make(chan bool, 1),
		readerShutdown: make(chan bool, 1),
		closeComplete:  make(chan bool, 2),
		rxTxWaitGroup:  new(sync.WaitGroup),
	}

	go nan0.startServiceReceiver(bb.messageIdentMap, nil, nil)
	go nan0.startServiceSender(bb.inverseIdentMap, bb.writeDeadlineActive, nil, nil)

	return nan0, err
}

func buildWebsocketClient(bb *baseBuilder) (nan0 NanoServiceWrapper, err error) {
	// setup a url to dial the websocket, hostname shouldn't include protocol
	u := url.URL{Scheme: "ws", Host: composeTcpAddress(bb.ns.HostName, bb.ns.Port), Path: bb.ns.Uri}
	// call the websocket server
	conn, _, err := websocket.DefaultDialer.Dial(u.String(), nil)
	checkError(err)

	return wrapConnectionWs(conn, bb)
}

func makeMdnsServerWs(wsb *WebsocketBuilder) (s *mdns.Server, err error) {
	return makeMdnsServer(wsb.baseBuilder, wsb.websocketFlag)
}

func buildWebsocketServer(wsb *WebsocketBuilder) (server *NanoServer, err error) {
	defer recoverPanic(func(e error) {
		slog.Fail("Error occurred while serving %v: %v", server.service.ServiceName, e)
		err = e
		server = nil
	})()
	var mdnsServer *mdns.Server = nil
	if wsb.serviceDiscovery {
		mdnsServer, err = makeMdnsServerWs(wsb)
		checkError(err)
	}
	server = &NanoServer{
		newConnections: make(chan NanoServiceWrapper),
		connections:    make([]NanoServiceWrapper, MaxNanoCache),
		closed:         false,
		service:        wsb.ns,
		rxTxWaitGroup:  new(sync.WaitGroup),
		mdnsServer:     mdnsServer,
	}

	var upgrader = websocket.Upgrader{
		ReadBufferSize:  1024,
		WriteBufferSize: 1024,
	}
	// create a CheckOrigin function suitable for the upgrader
	upgrader.CheckOrigin = func(r *http.Request) bool {
		for _, v := range wsb.origins {
			if v.Host == r.Header.Get("Host") {
				return true
			}
		}
		slog.Debug("Failed origin check for host %s", r.Header.Get("Host"))
		return false
	}

	// construct and sanitize origins
	if wsb.origin != "" {
		// dissect and interpret a manually set origin (for backward compat)
		rawOrigin, err := url.Parse(wsb.origin)
		if err != nil {
			wsb.origins = append(wsb.origins, rawOrigin)
		}
	}

	var handler http.HandlerFunc = func(w http.ResponseWriter, r *http.Request) {
		//check origin
		if !upgrader.CheckOrigin(r) {
			slog.Fail("Connection failed due to origin not accepted: %s", r.Host)
			return
		}
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			slog.Warn("Connection dropped due to %v", err)
			return
		} else {
			slog.Info("%s has connected to the server.", conn.RemoteAddr())
		}
		newNano, err := wrapConnectionWs(conn, wsb.baseBuilder)
		server.AddConnection(newNano)
	}
	serveMux := http.NewServeMux()
	serveMux.Handle(wsb.ns.GetUri(), handler)
	srv := &http.Server{Handler: serveMux, Addr: composeTcpAddress("", wsb.ns.Port)}
	srv.RegisterOnShutdown(func() {
		for _, conn := range server.connections {
			if conn != nil && !conn.IsClosed() {
				conn.Close()
			}
		}
	})
	server.wsServer = srv
	go func(serviceName string) {
		if wsb.tls == nil {
			if err := srv.ListenAndServe(); err != http.ErrServerClosed {
				slog.Fail("Websocket server: %s", err)
			} else {
				slog.Info("Websocket server %s closed.", serviceName)
			}
		} else {
			if err := srv.ListenAndServeTLS(wsb.tls.CertFile, wsb.tls.KeyFile); err != http.ErrServerClosed {
				slog.Fail("Websocket server: %s", err)
			} else {
				slog.Info("Websocket server %s closed.", serviceName)
			}
		}

	}(server.GetServiceName())

	return
}
