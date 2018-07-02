package nan0

import (
	"net"
	"io"
	"time"
	"fmt"
	"errors"
	"github.com/golang/protobuf/proto"
)

// An object that is used to control discovery of different nanoservices that have registered with the system
type DiscoveryService struct {
	defaultPort        int32
	nanoservices       map[string][]*Service
	nanoservicesByName map[string]*Service
	stale              bool
	shutdown           chan bool
	tcpShutdown        chan bool
	livenessShutdown   chan bool
}

// use new discovery service to prevent function unused issues
var _ = NewDiscoveryService(0, 0)

// Implements Stringer
func (ds DiscoveryService) String() string {
	return fmt.Sprintf("(Discovery Service Instance) defaultPort=%v, nanoservices=%v, stale=%v",
		ds.defaultPort, ds.nanoservicesByName, ds.stale)
}

// Passes the shutdown condition to this object. The result of this call is the object will no longer be able to process
// new information via tcp connections. This cannot be reversed and a new object will need to be created to re-establish
// a tcp server
func (ds *DiscoveryService) Shutdown() {
	if ds.stale == true {
		return
	}
	ds.stale = true

	// send the shutdown signal chain, both background services should handle this
	ds.shutdown <- true

	// await termination
	<-ds.tcpShutdown
	Logger.Printf("TCP at Port '%v' is shutdown", ds.defaultPort)
	<-ds.livenessShutdown
	Logger.Printf("Alive check at Port '%v' is shutdown\n", ds.defaultPort)
}

// Whether a shutdown has been triggered on this object
func (ds DiscoveryService) IsShutdown() bool {
	return ds.stale
}

// Retrieve a group of nanoservices by their service type
func (ds DiscoveryService) GetServicesByType(serviceType string) []*Service {
	if ds.stale == true {
		return nil
	}
	return ds.nanoservices[serviceType]
}

// Retrieve a nanoservice by its service name
func (ds DiscoveryService) GetServiceByName(serviceName string) *Service {
	if ds.stale == true {
		return nil
	}
	return ds.nanoservicesByName[serviceName]
}

// Get nanoservices registered to this object by the type. The result is the byte-slice representation of the protocol
// buffer object 'Service'
func (ds DiscoveryService) GetServicesByTypeBytes(serviceType string) (bytes []byte, err error) {
	if ds.stale == true {
		return nil, errors.New("discovery service object is stale")
	}
	message := &ServiceList{
		ServiceType:       serviceType,
		ServicesAvailable: ds.GetServicesByType(serviceType),
	}
	return proto.Marshal(message)
}

// Get nanoservices registered to this object by name. The result is the byte-slice representation of the protocol
// buffer object 'Service'
func (ds DiscoveryService) GetServiceByNameBytes(serviceName string) (bytes []byte, err error) {
	if ds.stale == true {
		return nil, errors.New("discovery service object is stale")
	}
	message := ds.GetServiceByName(serviceName)
	return proto.Marshal(message)
}

// Implements Writer interface. Assumes that p represents a ServiceList
func (ds *DiscoveryService) Write(p []byte) (n int, err error) {
	if ds.stale == true {
		return len(p), errors.New("discovery service object is stale")
	}
	defer recoverPanic(func(e error) { err = e.(error) })
	// make a NanoserviceList object
	serviceListMessage := &ServiceList{}
	// convert from byte array to NanoserviceList
	err = proto.Unmarshal(p, serviceListMessage)
	// ensure we set n to the size of the message received
	n = proto.Size(serviceListMessage)
	//
	if err != nil {
		panic(err)
	}
	if serviceListMessage.ServiceType != "" {
		serviceType := serviceListMessage.ServiceType
		servicesAvailable := serviceListMessage.ServicesAvailable
		ds.nanoservices[serviceType] = servicesAvailable
		for _, v := range servicesAvailable {
			ds.nanoservicesByName[v.ServiceName] = v
		}
	}
	return n, err
}

// Implements Reader interface, p represents a ServiceList
func (ds DiscoveryService) Read(p []byte) (n int, err error) {
	if ds.stale == true {
		return len(p), errors.New("discovery service object is stale")
	}
	defer recoverPanic(func(e error) { err = e.(error) })

	for _, service := range ds.nanoservicesByName {
		var serviceBytes = make([]byte, 0)
		serviceBytes, err = proto.Marshal(service)
		if err != nil {
			panic(err)
		}
		for i, v := range serviceBytes {
			p[i] = v
		}
		n += len(serviceBytes)
	}
	return n, err
}

// Register a nanoservice to the specified service type
func (ds *DiscoveryService) register(nanoservice *Service) {
	registeredServices := ds.nanoservices[nanoservice.ServiceType]
	registeredServices = append(registeredServices, nanoservice)
	ds.nanoservicesByName[nanoservice.ServiceName] = nanoservice
	Logger.Printf("Registered new service: %v", nanoservice)
}

// Perform a check of all services to see if they are expired. If so, remove them from all maps.
func (ds DiscoveryService) expireAllNS() {
	for key, services := range ds.nanoservices {
		k := 0
		for _, service := range services {
			// effectively remove expired services by not saving them
			if !service.IsExpired() {
				services[k] = service
				k++
			} else {
				Logger.Printf("Service expired: %v", service)
				// explicitly delete all services not saved from the named map
				delete(ds.nanoservicesByName, service.ServiceName)

			}
		}
		// retain all non-expired services
		ds.nanoservices[key] = services
	}
}

// Runs in the background to expire/refresh nanoservices
func (ds DiscoveryService) nanoserviceExpiryBackgroundProcess(serviceRefreshTimeInSec time.Duration) {
	Logger.Printf("Starting Liveness Check for Discovery Service on Port %v", ds.defaultPort)
	for ; ; {
		// this check occurs every interval
		time.Sleep(serviceRefreshTimeInSec * time.Second)
		if len(ds.nanoservices) > 0 {
			for _, services := range ds.nanoservices {
				for _, service := range services {
					if service.IsAlive() {
						service.Refresh()
					}
				}
			}
			// perform expiry if they are expired
			ds.expireAllNS()
		}

		// check termination of method
		select {
		case <-ds.shutdown:
			// resend on shutdown for any other waiting services
			ds.shutdown <- true
			// tell Shutdown that we are done with this method
			//TODO: maybe put these checks into a map
			ds.livenessShutdown <- true
			Logger.Println("Safely shutting down nanoservice expiration check")
			return
		default:
		}
	}
}

// Runs in background to receive registration requests from nanoservices
func (ds *DiscoveryService) tcpMessageReceiver() {
	Logger.Printf("Starting Nanoservice Receiver for Discovery Service on Port %v", ds.defaultPort)
	defer recoverPanic(nil)
	address := composeTcpAddress("", ds.defaultPort)
	listener, err := net.Listen("tcp", address)
	checkError(err)

	for ; ; {
		//set a deadline for listening
		if listener, ok := listener.(*net.TCPListener); ok {
			listener.SetDeadline(time.Now().Add(TCPTimeout))
		}
		// accept incomming information
		if conn, err := listener.Accept(); err == nil {
			// handle the nanoserviceList message
			go ds.handleTcpClient(conn)
		}

		// check termination of method, if shutdown channel has received a value
		select {
		case <-ds.shutdown:
			err = listener.Close()
			checkError(err)
			// resend on shutdown for any other waiting services
			ds.shutdown <- true
			// tell shutdown that this process is now complete
			// TODO: maybe put these checks into a map
			ds.tcpShutdown <- true
			Logger.Println("Safely shutting down tcp service")
			return
		default:
		}
	}
}

// Copy the information from the TCP connection to the discovery service
func (ds *DiscoveryService) handleTcpClient(conn net.Conn) {
	Logger.Println("Received connection from client")
	var err error = nil
	defer conn.Close()
	defer recoverPanic(func(e error) { err = e.(error) })
	//Read the data waiting on the connection and put it in the data buffer
	_, err = io.Copy(ds, conn)
	checkError(err)
}

// Generates a new DiscoveryService instance and starts its management protocol
func NewDiscoveryService(port int32, serviceRefreshTimeInSec time.Duration) *DiscoveryService {
	// skip initialization if port is invalid, return a non-working discovery service and do NOT
	// start any of the goroutines
	if port <= 0 {
		return &DiscoveryService{
			nanoservicesByName: nil,
			nanoservices:       nil,
			defaultPort:        0,
			stale:              true,
			shutdown:           nil,
			tcpShutdown:        nil,
			livenessShutdown:   nil,
		}
	}

	ds := &DiscoveryService{
		nanoservices:       make(map[string][]*Service),
		nanoservicesByName: make(map[string]*Service),
		defaultPort:        port,
		stale:              false,
		shutdown:           make(chan bool, 1),
		tcpShutdown:        make(chan bool, 1),
		livenessShutdown:   make(chan bool, 1),
	}
	// start expiration process
	go ds.nanoserviceExpiryBackgroundProcess(serviceRefreshTimeInSec)
	// start tcp registration server
	go ds.tcpMessageReceiver()
	return ds
}