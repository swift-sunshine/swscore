package graph

// MetadataKey is a mnemonic type name for string
type MetadataKey string

// Metadata is a map for storing node and edge metadata values reported by the vendors
type Metadata map[MetadataKey]interface{}

// NewMetadata returns an empty Metadata map
func NewMetadata() Metadata {
	return make(map[MetadataKey]interface{})
}

// Metadata keys to be used instead of literal strings
const (
	Aggregate             MetadataKey = "aggregate" // the prom attribute used for aggregation
	AggregateValue        MetadataKey = "aggregateValue"
	DestPrincipal         MetadataKey = "destPrincipal"
	DestServices          MetadataKey = "destServices"
	Gateways              MetadataKey = "gateways"
	HasCB                 MetadataKey = "hasCB"
	HasFaultInjection     MetadataKey = "hasFaultInjection"
	HasHealthConfig       MetadataKey = "hasHealthConfig"
	HasMissingSC          MetadataKey = "hasMissingSC"
	HasTCPTrafficShifting MetadataKey = "hasTCPTrafficShifting"
	HasTrafficShifting    MetadataKey = "hasTrafficShifting"
	HasRequestRouting     MetadataKey = "hasRequestRouting"
	HasRequestTimeout     MetadataKey = "hasRequestTimeout"
	HasVS                 MetadataKey = "hasVS"
	IsDead                MetadataKey = "isDead"
	IsEgressCluster       MetadataKey = "isEgressCluster"  // PassthroughCluster or BlackHoleCluster
	IsIngressGw           MetadataKey = "isIngressGateway" // Identifies a node that is an Istio ingress gateway
	IsIdle                MetadataKey = "isIdle"
	IsInaccessible        MetadataKey = "isInaccessible"
	IsMTLS                MetadataKey = "isMTLS"
	IsOutside             MetadataKey = "isOutside"
	IsRoot                MetadataKey = "isRoot"
	IsServiceEntry        MetadataKey = "isServiceEntry"
	ProtocolKey           MetadataKey = "protocol"
	ResponseTime          MetadataKey = "responseTime"
	SourcePrincipal       MetadataKey = "sourcePrincipal"
	Throughput            MetadataKey = "throughput"
	VirtualServices       MetadataKey = "virtualServices"
)

// DestServicesMetadata key=Service.Key()
type DestServicesMetadata map[string]ServiceName

// NewDestServicesMetadata returns an empty DestServicesMetadata map
func NewDestServicesMetadata() DestServicesMetadata {
	return make(map[string]ServiceName)
}

// Add adds or replaces a destService
func (dsm DestServicesMetadata) Add(key string, service ServiceName) DestServicesMetadata {
	dsm[key] = service
	return dsm
}

type GatewaysMetadata map[string][]string
type VirtualServicesMetadata map[string][]string
