package nan0

import (
	"net"
	"strings"
	"time"
)

/*******************
 	Service API
 *******************/

func (ns Service) MdnsTag() string {
	return strings.Join([]string{ns.ServiceName, ns.ServiceType}, ".")
}

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

// Starts a tcp listener for this service
func (ns *Service) Start() (net.Listener, error) {
	return net.Listen("tcp", composeTcpAddress(ns.HostName, ns.Port))
}
