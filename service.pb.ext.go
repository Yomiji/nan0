package nan0

import (
	"net"
	"time"
)

/*******************
 	Service API
 *******************/


// Checks if a particular nanoservice is expired based on its start time and time to live
func (ns Service) IsExpired() bool {
	return ns.Expired
}

// Checks if this nanoservice is responding to tcp on its port
func (ns Service) IsAlive() bool {
	conn, err := net.Dial("tcp", composeTcpAddress(ns.HostName, ns.Port))
	defer func() {
		if conn != nil {
			conn.Close()
		}
	}()
	if err != nil {
		ns.Expired = true
		return false
	}
	return true
}

// Refreshes the start time so that this service does not expire
func (ns *Service) Refresh() {
	ns.StartTime = time.Now().Unix()
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

// Starts a listener for this service
func (ns *Service) Start() (net.Listener, error) {
	return net.Listen("tcp", composeTcpAddress(ns.HostName, ns.Port))
}

// Create a connection to this nanoservice using a traditional TCP connection
func (ns Service) DialTCP() (nan0 net.Conn, err error) {
	nan0, err = net.Dial("tcp", composeTcpAddress(ns.HostName, ns.Port))

	return nan0, err
}

// Create a connection to this nanoservice using the Nan0 wrapper around a protocol buffer service layer
func (ns Service) DialNan0() *NanoBuilder {
	return ns.NewNanoBuilder()
}
