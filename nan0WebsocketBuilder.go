package nan0

import (
	"context"
	"crypto/tls"
	"net/http"
	"net/url"
	"sync"
	"time"

	"github.com/yomiji/mdns"
	"github.com/yomiji/slog"
	"github.com/yomiji/websocket"
)

type wsBuilderOption func(*WebsocketBuilder)

type TLSConfig struct {
	CertFile string
	KeyFile  string
	tls.Config
}

type WebsocketBuilder struct {
	websocketFlag bool
	origin        string
	origins       []*url.URL
	tls           *TLSConfig
	baseBuilder
}

// Enables secure websocket handling for this connection
func SecureWs(config TLSConfig) wsBuilderOption {
	return func(bb *WebsocketBuilder) {
		bb.secure = true
		bb.tls = &config
	}
}

// Adds origin checks to websocket handler (no use for clients)
func AddOrigins(origin ...string) wsBuilderOption {
	return func(wsb *WebsocketBuilder) {
		for _, v := range origin {
			u, err := url.Parse(v)
			if err == nil {
				wsb.origins = append(wsb.origins, u)
			}
		}
	}
}

// Adds origin checks to websocket handler (no use for clients)
func AppendOrigins(origin ...*url.URL) wsBuilderOption {
	return func(wsb *WebsocketBuilder) {
		wsb.origins = append(wsb.origins, origin...)
	}
}

func (wsb *WebsocketBuilder) BuildWebsocketClient(opts ...interface{}) (nan0 NanoServiceWrapper, err error) {
	wsb.buildOpts(opts)
	return wrapWsClient(wsb)(&wsb.baseBuilder)
}

func (wsb *WebsocketBuilder) BuildWebsocketDNS(ctx context.Context, strategy clientDNSStrategy, opts ...interface{}) ClientDNSFactory {
	wsb.buildOpts(opts)
	return buildDNS(ctx, &wsb.baseBuilder, wrapWsClient(wsb), strategy)
}

func (wsb *WebsocketBuilder) BuildWebsocketServer(opts ...interface{}) (*NanoServer, error) {
	wsb.buildOpts(opts)
	return buildWebsocketServer(wsb)
}

func (wsb *WebsocketBuilder) buildOpts(opts []interface{}) {
	for _, opt := range opts {
		switch ot := opt.(type) {
		case baseBuilderOption:
			ot(&wsb.baseBuilder)
		case wsBuilderOption:
			ot(wsb)
		case func(builder *baseBuilder):
			ot(&wsb.baseBuilder)
		}
	}
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
		closed:         make(chan struct{}),
		writerShutdown: make(chan struct{}, 1),
		readerShutdown: make(chan struct{}, 1),
		lastTxRx:       time.Now(),
		closeMux:       new(sync.Mutex),
	}

	go func() {
		nan0.startServiceReceiver(bb.routes, bb.messageIdentMap, nil, nil)
		nan0.Close()
	}()
	go func() {
		nan0.startServiceSender(bb.inverseIdentMap, bb.writeDeadlineActive, nil, nil)
		nan0.Close()
	}()
	go CheckAndDoWithDuration(func() bool {
		if bb.txRxIdleDuration <= 0 {
			return nan0.LastComm().Add(defaultTxRxIdleDuration).After(time.Now())
		}
		return nan0.LastComm().Add(bb.txRxIdleDuration).After(time.Now())
	}, func() {
		slog.Debug("[%s] Auto Close Triggered (idle: %v) (lastcom: %v)",
			nan0.GetServiceName(), bb.txRxIdleDuration, nan0.LastComm())
		nan0.Close()
	}, bb.txRxIdleDuration)
	return nan0, err
}

func wrapWsClient(wsb *WebsocketBuilder) nanoClientFactory {
	return func(bb *baseBuilder) (nan0 NanoServiceWrapper, err error) {
		// setup a url to dial the websocket, hostname shouldn't include protocol
		var u url.URL
		var conn *websocket.Conn
		if bb.secure {
			u = url.URL{Scheme: "wss", Host: composeTcpAddress(bb.ns.HostName, bb.ns.Port), Path: bb.ns.Uri}

			d := new(websocket.Dialer)
			d.TLSClientConfig = &wsb.tls.Config
			conn, _, err = d.Dial(u.String(), nil)
		} else {
			u = url.URL{Scheme: "ws", Host: composeTcpAddress(bb.ns.HostName, bb.ns.Port), Path: bb.ns.Uri}
			conn, _, err = websocket.DefaultDialer.Dial(u.String(), nil)
		}
		// call the websocket server

		checkError(err)

		return wrapConnectionWs(conn, bb)
	}
}

func makeMdnsServerWs(wsb *WebsocketBuilder) (s *mdns.Server, err error) {
	return makeMdnsServer(&wsb.baseBuilder, wsb.websocketFlag)
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
		service:            wsb.ns,
		connectionsList:    new(connList),
		allConnectionsList: new(connList),
		closed:             make(chan struct{}),
		mdnsServer:         mdnsServer,
	}

	server.connectionsList.init()
	server.allConnectionsList.init()

	if wsb.purgeConnections <= 0 {
		go func() {
			purgeLogic(defaultPurgeDuration, server, &wsb.baseBuilder)
		}()
	} else {
		go func() {
			purgeLogic(wsb.purgeConnections, server, &wsb.baseBuilder)
		}()
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
		newNano, err := wrapConnectionWs(conn, &wsb.baseBuilder)
		server.AddConnection(newNano)
	}

	serveMux := http.NewServeMux()
	serveMux.Handle(wsb.ns.GetUri(), handler)
	srv := &http.Server{Handler: serveMux, Addr: composeTcpAddress("", wsb.ns.Port)}

	server.wsServer = srv
	go func(serviceName string, wsb WebsocketBuilder) {
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
	}(server.GetServiceName(), *wsb)

	return
}
