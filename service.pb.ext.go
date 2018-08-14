package nan0

import (
	"net"
	"time"
	"github.com/golang/protobuf/proto"
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
	address := composeTcpAddress(ns.HostName, ns.Port)
	conn, err := net.Dial("tcp", address)
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

// Registers this nanoservice with the service discovery host at the given address
func (ns *Service) Register(host string, port int32) (err error) {
	address := composeTcpAddress(host, port)
	info("Registering '%v' service with discovery at '%v'", ns.ServiceName, address)
	defer recoverPanic(func(e error) { err = e.(error) })()
	conn, err := net.Dial("tcp", address)
	checkError(err)
	serviceList := &ServiceList{
		ServiceType:       ns.ServiceType,
		ServicesAvailable: []*Service{ns},
	}
	serviceListBytes, err := proto.Marshal(serviceList)
	checkError(err)
	_, err = conn.Write(serviceListBytes)
	checkError(err)
	return err
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
func (ns Service) DialNan0() *SecureNanoBuilder {
	return ns.NewNanoBuilder()
}

// Start the process of creating a secure nanoservice using a builder for the parameters
func (ns Service) DialNan0Secure(secretKey *[32]byte, authKey *[32]byte) *SecureNanoBuilder {
	return ns.NewNanoBuilder().EnableEncryption(secretKey, authKey)
}
