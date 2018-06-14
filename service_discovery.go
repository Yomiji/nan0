package service_discovery
/**
Nanoservice Discovery API
Some features that are implemented here:
DiscoveryService implements Stringer, io.Reader, io.Writer
This API accepts and manages nanoservices
 */
import (
	"fmt"
	"time"
	"github.com/golang/protobuf/proto"
	"net"
)

type DiscoveryService struct {
	hostName           string
	defaultPort        int
	nanoservices       map[string][]*Nanoservice
	nanoservicesByName map[string]*Nanoservice
	pinger             func()
	shutdown           bool
}

// Implements Stringer
func (ds DiscoveryService) String() string {
	return "(Discovery Service Instance) {HostName='" + ds.hostName +
		"' DefaultPort='"+ fmt.Sprint(ds.defaultPort) +
		"' Nanoservices=[" + fmt.Sprint(ds.nanoservices) + "]"
}

// Register a nanoservice to the specified service type
func (ds *DiscoveryService) Register(serviceType string, nanoservice *Nanoservice) {
	registeredServices := ds.nanoservices[serviceType]
	registeredServices = append(registeredServices, nanoservice)
	ds.nanoservicesByName[nanoservice.ServiceName] = nanoservice
}

// Perform a check of all services to see if they are expired. If so, remove them from all maps.
func (ds DiscoveryService) ExpireAllNS() {
	for key,services := range ds.nanoservices {
		k := 0
		for _,service := range services {
			// effectively remove expired services by not saving them
			if !service.IsExpired() {
				services[k] = service
				k++
			} else {
				// explicitly delete all services not saved from the named map
				delete(ds.nanoservicesByName, service.ServiceName)
			}
		}
		// retain all non-expired services
		ds.nanoservices[key] = services
	}
}

// Retrieve a group of nanoservices by their service type
func (ds DiscoveryService) GetNanoservicesByType(serviceType string) []*Nanoservice {
	return ds.nanoservices[serviceType]
}

// Retrieve a nanoservice by its service name
func (ds DiscoveryService) GetNanoserviceByName(serviceName string) *Nanoservice {
	return ds.nanoservicesByName[serviceName]
}

func (ds DiscoveryService) GetNanoservicesByTypeBytes(serviceType string) (bytes []byte, err error) {
	message := &NanoserviceList{
		ServiceType:serviceType,
		ServicesAvailable:ds.GetNanoservicesByType(serviceType),
	}
	return proto.Marshal(message)
}

func (ds DiscoveryService) GetNanoserviceByNameBytes(serviceName string) (bytes []byte, err error) {
	message := ds.GetNanoserviceByName(serviceName)
	return proto.Marshal(message)
}

// Implements Writer interface. Assumes that p represents a NanoserviceList
func (ds *DiscoveryService) Write(p []byte) (n int, err error) {
	defer func() {
		if e := recover(); e != nil {
			err = e.(error)
		}
	}()
	// make a NanoserviceList object
	serviceListMessage := &NanoserviceList{}
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
		for _,v := range servicesAvailable {
			ds.nanoservicesByName[v.ServiceName] = v
		}
	}
	return n, err
}

// Implements Reader interface
func (ds DiscoveryService) Read(p []byte) (n int, err error) {
	defer func() {
		if e := recover(); e != nil {
			err = e.(error)
		}
	}()

	for _,service := range ds.nanoservicesByName {
		var serviceBytes = make([]byte, 0)
		serviceBytes, err = proto.Marshal(service)
		if err != nil {
			panic(err)
		}
		for i,v := range serviceBytes {
			p[i] = v
		}
		n += len(serviceBytes)
	}
	return n, err
}

// Generates a new DiscoveryService instance and starts its management protocol
func NewDiscoveryService(hostName string, port int, serviceRefreshTimeInSec time.Duration) *DiscoveryService {
	ds := &DiscoveryService{
		nanoservices:       make(map[string][]*Nanoservice),
		nanoservicesByName: make(map[string]*Nanoservice),
		shutdown:           false,
		hostName:           hostName,
		defaultPort:        port,
	}
	// define a method that will run in the background to expire/refresh nanoservices
	ds.pinger = func() {
		for ds.shutdown != true {
			// this check occurs every interval
			time.Sleep(serviceRefreshTimeInSec * time.Second)
			if len(ds.nanoservices) > 0 {
				for _,services := range ds.nanoservices {
					for _,service := range services {
						if service.IsAlive() {
							service.Refresh()
						}
					}
				}
			}
			// perform expiry if they are expired
			ds.ExpireAllNS()
		}
	}
	if serviceRefreshTimeInSec != 0 {
		go ds.pinger()
	}
	return ds
}

// Checks if a particular nanoservice is expired based on its start time and time to live
func (ns Nanoservice) IsExpired() bool {
	nowInMS := time.Now().Unix()
	return (nowInMS - ns.StartTime) >= ns.TimeToLiveInMS
}

// Checks if this nanoservice is responding to tcp on its port
func (ns Nanoservice) IsAlive() bool {
	address := fmt.Sprintf("%v:%v", ns.HostName, ns.Port)
	_,err := net.Dial("tcp", address)
	if err != nil {
		return false
	}
	return true
}

// Refreshes the start time so that this service does not expire
func (ns *Nanoservice) Refresh() {
	ns.StartTime = time.Now().Unix()
}
