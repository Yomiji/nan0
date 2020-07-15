package nan0

import (
	"context"
	"encoding/base64"

	"github.com/golang/protobuf/proto"
	"github.com/yomiji/mdns"
	"github.com/yomiji/slog"
)

func makeMdnsServer(nsb *NanoBuilder) (s *mdns.Server, err error) {
	defer recoverPanic(func(e error) {
		slog.Fail("while creating mdns server: %v", e)
		s = nil
		err = e
	})

	var protobufTypes = getIdentityMessageNames(nsb)

	mdnsDefinition := &MDefinition{
		ConnectionInfo: &ConnectionInfo{
			Websocket: nsb.websocketFlag,
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
		"", "", int(nsb.ns.MdnsPort), nil, []string{string(bytes64)})
	checkError(err)
	mdnsServer, err := mdns.NewServer(&mdns.Config{Zone: mdnsService})
	return mdnsServer, err
}

func startClientServiceDiscovery(ctx context.Context, ns *Service) <-chan *MDefinition {
	serviceTag := ns.MdnsTag()
	entriesCh := make(chan *mdns.ServiceEntry)
	nan0ServicesFound := make(chan *MDefinition)
	go func() {
		if ctx != nil {
			<-ctx.Done()
			close(entriesCh)
			close(nan0ServicesFound)
		}
	}()
	go func() {
		for entry := range entriesCh {
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

func getIdentityMessageNames(nsb *NanoBuilder) []string {
	var protobufTypes []string
	for _, v := range nsb.messageIdentMap {
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