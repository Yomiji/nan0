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
)

// An object that is used to control discovery of different nanoservices that have registered with the system
type DiscoveryService struct {
	defaultPort        int32
	nanoservices       map[string][]*Service
	nanoservicesByName map[string]*Service
	shutdown           chan bool
	stale              bool
}

// A logger with a 'Service API' prefix used to log  s from this api. Call log.SetOutput to change output
// writer
var Logger = log.New(os.Stderr, "Service API: ", log.Ldate|log.Ltime|log.Lshortfile)

// use new discovery service to prevent function unused issues
var _ = NewDiscoveryService("", 0, 0)

// Implements Stringer
func (ds DiscoveryService) String() string {
	return "(Discovery Service Instance) {" +
		"defaultPort='" + fmt.Sprint(ds.defaultPort) +
		"' nanoservices=[" + fmt.Sprint(ds.nanoservices) +
		"] " + " shutdown='" + fmt.Sprint(ds.shutdown) + "'}"
}

// Passes the shutdown condition to this object. The result of this call is the object will no longer be able to process
// new information via tcp connections. This cannot be reversed and a new object will need to be created to re-establish
// a tcp server
func (ds *DiscoveryService) Shutdown() {
	ds.stale = true
	ds.shutdown <- true
}

// Whether a shutdown has been triggered on this object
func (ds DiscoveryService) IsShutdown() bool {
	return ds.stale
}

// Retrieve a group of nanoservices by their service type
func (ds DiscoveryService) GetServicesByType(serviceType string) []*Service {
	return ds.nanoservices[serviceType]
}

// Retrieve a nanoservice by its service name
func (ds DiscoveryService) GetServicesByName(serviceName string) *Service {
	return ds.nanoservicesByName[serviceName]
}

// Get nanoservices registered to this object by the type. The result is the byte-slice representation of the protocol
// buffer object 'Service'
func (ds DiscoveryService) GetServicesByTypeBytes(serviceType string) (bytes []byte, err error) {
	message := &ServiceList{
		ServiceType:       serviceType,
		ServicesAvailable: ds.GetServicesByType(serviceType),
	}
	return proto.Marshal(message)
}

// Get nanoservices registered to this object by name. The result is the byte-slice representation of the protocol
// buffer object 'Service'
func (ds DiscoveryService) GetServicesByNameBytes(serviceName string) (bytes []byte, err error) {
	message := ds.GetServicesByName(serviceName)
	return proto.Marshal(message)
}

// Implements Writer interface. Assumes that p represents a ServiceList
func (ds *DiscoveryService) Write(p []byte) (n int, err error) {
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
		}
		// perform expiry if they are expired
		ds.expireAllNS()

		// check termination of method
		select {
		case <-ds.shutdown:
			Logger.Println("Safely shutting down nanoservice expiration check")
			return
		default:
		}
	}
}

// Runs in background to receive registration requests from nanoservices
func (ds *DiscoveryService) tcpMessageReceiver() {
	defer recoverPanic(nil)
	address := composeTcpAddress("", ds.defaultPort)
	listener, err := net.Listen("tcp", address)
	checkError(err)
	for ; ; {
		// accept incomming information
		if conn, err := listener.Accept(); err == nil {
			// handle the nanoserviceList message
			go ds.handleTcpClient(conn)
		} else {
			continue
		}

		// check termination of method
		select {
		case <-ds.shutdown:
			Logger.Println("Safely shutting down tcp service")
			err = listener.Close()
			checkError(err)
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
func NewDiscoveryService(hostName string, port int32, serviceRefreshTimeInSec time.Duration) *DiscoveryService {
	if hostName == "" || port == 0 {
		return &DiscoveryService{}
	}

	ds := &DiscoveryService{
		nanoservices:       make(map[string][]*Service),
		nanoservicesByName: make(map[string]*Service),
		shutdown:           make(chan bool),
		defaultPort:        port,
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
	defer recoverPanic(func(e error) { err = e.(error) })
	conn, err := net.Dial("tcp", composeTcpAddress(host, port))
	checkError(err)
	serviceList := &ServiceList{
		ServiceType:ns.ServiceType,
		ServicesAvailable: []*Service{ns},
	}
	serviceListBytes, err := proto.Marshal(serviceList)
	checkError(err)
	_, err = conn.Write(serviceListBytes)
	checkError(err)
	return err
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
