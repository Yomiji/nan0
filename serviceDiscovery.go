package nan0

import (
	"context"
	"encoding/base64"
	"fmt"

	"github.com/golang/protobuf/proto"
	"github.com/yomiji/mdns"
	"github.com/yomiji/slog"
)

type ClientDNSFactory func(strictProtocols bool) (nan0 NanoServiceWrapper, err error)

func makeMdnsServer(nsb *baseBuilder, ws bool) (s *mdns.Server, err error) {
	defer recoverPanic(func(e error) {
		slog.Fail("while creating mdns server: %v", e)
		s = nil
		err = e
	})

	var protobufTypes = getIdentityMessageNames(nsb)

	mdnsDefinition := &MDefinition{
		ConnectionInfo: &ConnectionInfo{
			Websocket: ws,
			Port:      nsb.ns.Port,
			HostName:  nsb.ns.HostName,
			Uri:       nsb.ns.Uri,
		},
		SupportedMessageTypes: protobufTypes,
	}
	definitionBytes, err := proto.Marshal(mdnsDefinition)
	checkError(err)
	var bytes64 = make([]byte, base64.StdEncoding.EncodedLen(len(definitionBytes)))
	base64.StdEncoding.Encode(bytes64, definitionBytes)
	mdnsService, err := mdns.NewMDNSService(nsb.ns.HostName, nsb.ns.MdnsTag(),
		"", "", int(nsb.ns.Port), nil, []string{string(bytes64)})
	checkError(err)
	mdnsServer, err := mdns.NewServer(&mdns.Config{Zone: mdnsService})
	return mdnsServer, err
}

func startClientServiceDiscovery(ctx context.Context, ns DiscoverableService) <-chan *MDefinition {
	serviceTag := ns.MdnsTag()
	entriesCh := make(chan *mdns.ServiceEntry)
	nan0ServicesFound := make(chan *MDefinition)
	go func() {
		if ctx != nil {
			<-ctx.Done()
			slog.Debug("Shutting down a client discovery channel")
			close(entriesCh)
			close(nan0ServicesFound)
		}
	}()
	go func() {
		for entry := range entriesCh {
			slog.Debug("ServiceDiscovery Entry Found: %+v", entry)
			if len(entry.InfoFields) > 0 {
				var mDef = &MDefinition{}
				mdefBytes, err := base64.StdEncoding.DecodeString(entry.Info)
				if err != nil {
					slog.Warn("mdns message corrupted in transmission for: %s [%+v]", entry.Name, entry.Addr)
				}
				err = proto.Unmarshal(mdefBytes, mDef)
				if err != nil {
					slog.Warn("corrupted definition file for: %s [%+v]", entry.Name, entry.Addr)
				} else {
					nan0ServicesFound <- mDef
				}
			}
		}
	}()

	err := mdns.Lookup(serviceTag, entriesCh)
	if err != nil {
		slog.Fail("%v", err)
	}
	return nan0ServicesFound
}

func getIdentityMessageNames(bb *baseBuilder) []string {
	var protobufTypes []string
	for _, v := range bb.messageIdentMap {
		protobufTypes = append(protobufTypes, proto.MessageName(v))
	}
	return protobufTypes
}

func populateServiceFromMDef(service *Service, definition *MDefinition) {
	service.Port = definition.ConnectionInfo.Port
	service.HostName = definition.ConnectionInfo.HostName
	if definition.ConnectionInfo.Websocket {
		service.Uri = definition.ConnectionInfo.Uri
	}
}

func allProtocolsMatch(sec *baseBuilder, definition *MDefinition) bool {
	builderNames := getIdentityMessageNames(sec)
	mdefNames := definition.SupportedMessageTypes
	if len(builderNames) != len(mdefNames) {
		return false
	}
	var checkerMap = make(map[string]bool)
	for _, name := range builderNames {
		checkerMap[name] = true
	}
	for _, name := range mdefNames {
		if _, ok := checkerMap[name]; !ok {
			return false
		}
	}
	return true
}


func processMdef(sec *baseBuilder, mdef *MDefinition, strict bool) error {
	if strict && !allProtocolsMatch(sec, mdef) {
		return fmt.Errorf("builder fails to satisfy all protocols for service %s", sec.ns.ServiceName)
	}
	populateServiceFromMDef(sec.ns, mdef)
	return nil
}