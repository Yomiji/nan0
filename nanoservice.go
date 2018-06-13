package service_discovery

type DiscoveryServerConfiguration struct {
	HostName string
	DefaultPort int
}

type Nanoservice interface {
	Register(config DiscoveryServerConfiguration) (bool, error)
	GetServerMessage()
}