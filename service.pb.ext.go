package nan0

import (
	"net"
	"strings"
	"time"
)

/*******************
 	Service API
 *******************/
const defaultTxRxIdleDuration = 5 * time.Minute

func (ns *Service) NewNanoBuilder() *NanoBuilder {
	nsb := new(NanoBuilder)
	nsb.baseBuilder = new(baseBuilder)
	nsb.txRxIdleDuration = defaultTxRxIdleDuration
	nsb.initialize(ns)
	return nsb
}

func (ns *Service) NewWebsocketBuilder() *WebsocketBuilder {
	wsb := new(WebsocketBuilder)
	wsb.initialize(ns)
	wsb.websocketFlag = true
	return wsb
}

func (ns Service) MdnsTag() string {
	return strings.Join([]string{ns.ServiceName, ns.ServiceType}, ".")
}

// Compare two service instances for equality
func (ns Service) Equals(other Service) bool {
	return ns.Port == other.Port &&
		ns.HostName == other.HostName &&
		ns.Expired == other.Expired &&
		ns.StartTime == other.StartTime &&
		ns.ServiceName == other.ServiceName &&
		ns.ServiceType == other.ServiceType
}

// Starts a tcp listener for this service
func (ns Service) start() (net.Listener, error) {
	return net.Listen("tcp", composeTcpAddress("", ns.Port))
}
