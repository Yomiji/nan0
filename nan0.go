package nan0

/**
Service Discovery API
Some features that are implemented here:
DiscoveryService implements Stringer, io.Reader, io.Writer
This API accepts and manages nanoservices
 */
import (
	"fmt"
	"time"
	"github.com/golang/protobuf/proto"
	"net"
	"os"
	"io"
	"log"
	"errors"
)

// An object that is used to control discovery of different nanoservices that have registered with the system
type DiscoveryService struct {
	defaultPort        int32
	nanoservices       map[string][]*Service
	nanoservicesByName map[string]*Service
	stale              bool
	shutdown 		   chan bool
	tcpShutdown		   chan bool
	livenessShutdown   chan bool
}

// A logger with a 'Service API' prefix used to log  s from this api. Call log.SetOutput to change output
// writer
var Logger = log.New(os.Stderr, "Service API: ", log.Ldate|log.Ltime|log.Lshortfile)

var TCPTimeout = 10 * time.Second

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

	// ensure that we send something to trigger the net.Accept for this discovery service so that we can continue
	//defer recoverPanic(nil)
	//conn, err := net.Dial("tcp", composeTcpAddress("localhost", ds.defaultPort))
	//checkError(err)
	//b, err := proto.Marshal(&Service{})
	//checkError(err)
	//conn.Write(b)

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
			Logger.Println("Safely shutting down nanoservice expiration check")
			// resend on shutdown for any other waiting services
			ds.shutdown <- true
			Logger.Println("Liveness shutdown signal successfully resent")
			// tell Shutdown that we are done with this method
			//TODO: maybe put these checks into a map
			ds.livenessShutdown <- true
			Logger.Println("Liveness final check sent")
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
			Logger.Println("Safely shutting down tcp service")
			err = listener.Close()
			checkError(err)
			// resend on shutdown for any other waiting services
			ds.shutdown <- true
			Logger.Println("TCP's shutdown signal successfully resent")
			// tell shutdown that this process is now complete
			// TODO: maybe put these checks into a map
			ds.tcpShutdown <- true
			Logger.Println("TCP Final Check Sent")
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
			nanoservices: nil,
			defaultPort: 0,
			stale: true,
			shutdown: nil,
			tcpShutdown: nil,
			livenessShutdown: nil,
		}
	}

	ds := &DiscoveryService{
		nanoservices:       make(map[string][]*Service),
		nanoservicesByName: make(map[string]*Service),
		defaultPort:        port,
		stale: 				false,
		shutdown:			make(chan bool, 1),
		tcpShutdown:		make(chan bool, 1),
		livenessShutdown:	make(chan bool, 1),
	}
	// start expiration process
	go ds.nanoserviceExpiryBackgroundProcess(serviceRefreshTimeInSec)
	// start tcp registration server
	go ds.tcpMessageReceiver()
	return ds
}

/*
 *	Service API
 */

// Checks if a particular nanoservice is expired based on its start time and time to live
func (ns Service) IsExpired() bool {
	nowInMS := time.Now().Unix()
	return (nowInMS - ns.StartTime) >= ns.TimeToLiveInMS
}

// Checks if this nanoservice is responding to tcp on its port
func (ns Service) IsAlive() bool {
	address := composeTcpAddress(ns.HostName, ns.Port)
	conn, err := net.Dial("tcp", address)
	defer conn.Close()
	if err != nil {
		return false
	}
	return true
}

// Refreshes the start time so that this service does not expire
func (ns *Service) Refresh() {
	ns.StartTime = time.Now().Unix()
}

/*
 *	Helper Functions
 */
// Create a connection to this nanoservice
func (ns *Service) Dial() (conn net.Conn, err error) {
	return net.Dial("tcp", composeTcpAddress(ns.HostName, ns.Port))
}

// Registers this nanoservice with the service discovery host at the given address
func (ns *Service) Register(host string, port int32) (err error) {
	address := composeTcpAddress(host, port)
	Logger.Printf("Registering '%v' service with discovery at '%v'", ns.ServiceName, address)
	defer recoverPanic(func(e error) { err = e.(error) })
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

func (ns Service) Equals(other Service) bool {
	return ns.Port == other.Port &&
		ns.HostName == other.HostName &&
		ns.TimeToLiveInMS == other.TimeToLiveInMS &&
		ns.StartTime == other.StartTime &&
		ns.ServiceName == other.ServiceName &&
		ns.ServiceType == other.ServiceType
}

// Checks the error passed and panics if issue found
func checkError(err error) {
	if err != nil {
		Logger.Printf("Error occurred: %s", err.Error())
		panic(err)
	}
}

// Combines a host name string with a port number to create a valid tcp address
func composeTcpAddress(hostName string, port int32) string {
	return fmt.Sprintf("%s:%d", hostName, port)
}

// Recovers from a panic using the recovery method and applies the given behavior when a recovery
// has occurred.
func recoverPanic(errfunc func(error)) func() {
	if errfunc != nil {
		return func() {
			if e := recover(); e != nil {
				// execute the abstract behavior
				errfunc(e.(error))
			}
		}
	} else {
		return func() {
			recover()
		}
	}
}
