package service_discovery

import (
	"fmt"
	"time"
	"github.com/golang/protobuf/proto"
	"errors"
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

// Implements Writer interface
func (ds DiscoveryService) Write(p []byte) (n int, err error) {
	defer func() {
		if e := recover(); e != nil {
			err = e.(error)
		}
	}()
	lenOfNano := proto.Size(&Nanoservice{})
	if len(p) < lenOfNano {
		return 0, errors.New("invalid length of byte array")
	}
	for i := 0; (i * lenOfNano) < len(p); i++ {
		start := i * lenOfNano
		end := ((i + 1) * lenOfNano) - 1
		if end < len(p) {
			nanoservice := &Nanoservice{}
			proto.Unmarshal(p[start:end], nanoservice)
			ds.Register(nanoservice.ServiceType, nanoservice)
			n += lenOfNano
		}
	}
	return n, nil
}

// Implements Reader interface
func (ds DiscoveryService) Read(p []byte) (n int, err error) {
	defer func() {
		if e := recover(); e != nil {
			err = e.(error)
		}
	}()
	j := 0
	for _,service := range ds.nanoservicesByName {
		bytes, err := proto.Marshal(service)
		if err != nil {
			panic(err)
		}
		lenOfNano := len(bytes)
		start := j * lenOfNano
		end := ((j + 1) * lenOfNano) - 1
		for i := range p[start : end] {
			p[start + i] = bytes[i]
			n++
		}
		j++
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