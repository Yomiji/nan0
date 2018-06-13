package service_discovery

import (
	"time"
	"net"
	"fmt"
)

func (ns Nanoservice) IsExpired() bool {
	nowInMS := time.Now().Unix()
	return (nowInMS - ns.StartTime) >= ns.TimeToLiveInMS
}

func (ns Nanoservice) IsAlive() bool {
	address := fmt.Sprintf("%v:%v", ns.HostName, ns.Port)
	_,err := net.Dial("tcp", address)
	if err != nil {
		return false
	}
	return true
}

func (ns *Nanoservice) Refresh() {
	ns.StartTime = time.Now().Unix()
}
