package graph

import (
	"fmt"
	"strings"

	"github.com/kiali/kiali/log"
)

// constants usable by any protocol
const (
	requestsPerSecond = "requests per second"
	rps               = "rps"
)

// Rate describes one rate provided by a protocol
type Rate struct {
	Name         MetadataKey
	IsErr        bool
	IsIn         bool
	IsOut        bool
	IsPercentErr bool
	IsPercentReq bool
	IsTotal      bool
	Precision    int
}

// Protocol describes a supported protocol and the rates it provides
type Protocol struct {
	Name          string
	EdgeRates     []Rate
	EdgeResponses MetadataKey
	NodeRates     []Rate
	Unit          string
	UnitShort     string
}

// Each supported protocol is defined below.  Each rate provided as node or edge metadata must be defined.
// Each method below should have a section handling each supported protocol.

//
// GRPC Protocol
//
const (
	grpc             = "grpc"
	grpcNoResponse   = "grpcNoResponse" //typically a client termination (envoy flag=DC)
	grpcErr          = "grpcErr"
	grpcPercentErr   = "grpcPercentErr"
	grpcPercentReq   = "grpcPercentReq"
	grpcResponses    = "grpcResponses"
	grpcIn           = "grpcIn"
	grpcInNoResponse = "grpcInNoResponse"
	grpcInErr        = "grpcInErr"
	grpcOut          = "grpcOut"
)

// GRPC Protocol
var GRPC = Protocol{
	Name: grpc,
	EdgeRates: []Rate{
		{Name: grpc, IsTotal: true, Precision: 2},
		{Name: grpcNoResponse, IsErr: true, Precision: 2},
		{Name: grpcErr, IsErr: true, Precision: 2},
		{Name: grpcPercentErr, IsPercentErr: true, Precision: 1},
		{Name: grpcPercentReq, IsPercentReq: true, Precision: 1},
	},
	EdgeResponses: grpcResponses,
	NodeRates: []Rate{
		{Name: grpcIn, IsIn: true, Precision: 2},
		{Name: grpcInNoResponse, IsErr: true, Precision: 2},
		{Name: grpcInErr, IsErr: true, Precision: 2},
		{Name: grpcOut, IsOut: true, Precision: 2},
	},
	Unit:      requestsPerSecond,
	UnitShort: rps,
}

//
// HTTP Protocol
//
const (
	http             = "http"
	httpNoResponse   = "httpNoResponse" // typically a client termination (envoy flag=DC)
	http3xx          = "http3xx"
	http4xx          = "http4xx"
	http5xx          = "http5xx"
	httpPercentErr   = "httpPercentErr"
	httpPercentReq   = "httpPercentReq"
	httpResponses    = "httpResponses"
	httpIn           = "httpIn"
	httpInNoResponse = "httpInNoResponse"
	httpIn3xx        = "httpIn3xx"
	httpIn4xx        = "httpIn4xx"
	httpIn5xx        = "httpIn5xx"
	httpOut          = "httpOut"
)

// HTTP Protocol
var HTTP = Protocol{
	Name: http,
	EdgeRates: []Rate{
		{Name: http, IsTotal: true, Precision: 2},
		{Name: httpNoResponse, IsErr: true, Precision: 2},
		{Name: http3xx, Precision: 2},
		{Name: http4xx, IsErr: true, Precision: 2},
		{Name: http5xx, IsErr: true, Precision: 2},
		{Name: httpPercentErr, IsPercentErr: true, Precision: 1},
		{Name: httpPercentReq, IsPercentReq: true, Precision: 1},
	},
	EdgeResponses: httpResponses,
	NodeRates: []Rate{
		{Name: httpIn, IsIn: true, Precision: 2},
		{Name: httpInNoResponse, IsErr: true, Precision: 2},
		{Name: httpIn3xx, Precision: 2},
		{Name: httpIn4xx, IsErr: true, Precision: 2},
		{Name: httpIn5xx, IsErr: true, Precision: 2},
		{Name: httpOut, IsOut: true, Precision: 2},
	},
	Unit:      requestsPerSecond,
	UnitShort: rps,
}

//
// TCP Protocol
//
const (
	tcp            = "tcp"
	tcpResponses   = "tcpResponses"
	tcpIn          = "tcpIn"
	tcpOut         = "tcpOut"
	bytesPerSecond = "bytes per second"
	bps            = "bps"
)

// TCP Protocol
var TCP = Protocol{
	Name: tcp,
	EdgeRates: []Rate{
		{Name: tcp, IsTotal: true, Precision: 2},
	},
	EdgeResponses: tcpResponses,
	NodeRates: []Rate{
		{Name: tcpIn, IsIn: true, Precision: 2},
		{Name: tcpOut, IsOut: true, Precision: 2},
	},
	Unit:      bytesPerSecond,
	UnitShort: bps,
}

// Protocols defines the supported protocols to be handled by the vendor code.
var Protocols = []Protocol{GRPC, HTTP, TCP}

// AddToMetadata takes a single traffic value and adds it appropriately as source, dest and edge traffic
func AddToMetadata(protocol string, val float64, code, flags, host string, sourceMetadata, destMetadata, edgeMetadata Metadata) {
	if val <= 0.0 {
		return
	}

	switch protocol {
	case grpc:
		addToMetadataGrpc(val, code, flags, host, sourceMetadata, destMetadata, edgeMetadata)
	case http:
		addToMetadataHTTP(val, code, flags, host, sourceMetadata, destMetadata, edgeMetadata)
	case tcp:
		addToMetadataTCP(val, flags, host, sourceMetadata, destMetadata, edgeMetadata)
	default:
		log.Tracef("Ignore unhandled metadata protocol [%s]", protocol)
	}
}

func addToMetadataGrpc(val float64, code, flags, host string, sourceMetadata, destMetadata, edgeMetadata Metadata) {
	addToMetadataValue(sourceMetadata, grpcOut, val)
	addToMetadataValue(destMetadata, grpcIn, val)
	addToMetadataValue(edgeMetadata, grpc, val)
	addToMetadataResponses(edgeMetadata, grpcResponses, code, flags, host, val)

	switch {
	case code == "-":
		addToMetadataValue(destMetadata, grpcInNoResponse, val)
		addToMetadataValue(edgeMetadata, grpcNoResponse, val)
	default:
		// Older Istio telemetry may use HTTP codes for gRPC, so if it quacks like a duck...
		isHTTPCode := len(code) == 3
		isErr := false
		if isHTTPCode {
			isErr = IsHTTPErr(code)
		} else {
			isErr = IsGRPCErr(code)
		}
		if isErr {
			addToMetadataValue(destMetadata, grpcInErr, val)
			addToMetadataValue(edgeMetadata, grpcErr, val)
		}
	}
}

func addToMetadataHTTP(val float64, code, flags, host string, sourceMetadata, destMetadata, edgeMetadata Metadata) {
	addToMetadataValue(sourceMetadata, httpOut, val)
	addToMetadataValue(destMetadata, httpIn, val)
	addToMetadataValue(edgeMetadata, http, val)
	addToMetadataResponses(edgeMetadata, httpResponses, code, flags, host, val)

	// note, we don't track 2xx because it's not used downstream and can be easily
	// calculated: 2xx = (rate - NoResponse - 3xx - 4xx - 5xx)
	switch {
	case code == "-":
		addToMetadataValue(destMetadata, httpInNoResponse, val)
		addToMetadataValue(edgeMetadata, httpNoResponse, val)
	case strings.HasPrefix(code, "3"):
		addToMetadataValue(destMetadata, httpIn3xx, val)
		addToMetadataValue(edgeMetadata, http3xx, val)
	case strings.HasPrefix(code, "4"):
		addToMetadataValue(destMetadata, httpIn4xx, val)
		addToMetadataValue(edgeMetadata, http4xx, val)
	case strings.HasPrefix(code, "5"):
		addToMetadataValue(destMetadata, httpIn5xx, val)
		addToMetadataValue(edgeMetadata, http5xx, val)
	}
}

func addToMetadataTCP(val float64, flags, host string, sourceMetadata, destMetadata, edgeMetadata Metadata) {
	addToMetadataValue(sourceMetadata, tcpOut, val)
	addToMetadataValue(destMetadata, tcpIn, val)
	addToMetadataValue(edgeMetadata, tcp, val)
	addToMetadataResponses(edgeMetadata, tcpResponses, "-", flags, host, val)
}

// IsHTTPErr return true if code is 4xx or 5xx
func IsHTTPErr(code string) bool {
	return strings.HasPrefix(code, "4") || strings.HasPrefix(code, "5")
}

// IsGRPCErr return true if code != 0
func IsGRPCErr(code string) bool {
	return code != "0" && code != ""
}

// AddOutgoingEdgeToMetadata updates the source node's outgoing traffic with the outgoing edge traffic value
func AddOutgoingEdgeToMetadata(sourceMetadata, edgeMetadata Metadata) {
	if val, valOk := edgeMetadata[grpc]; valOk {
		addToMetadataValue(sourceMetadata, grpcOut, val.(float64))
	}
	if val, valOk := edgeMetadata[http]; valOk {
		addToMetadataValue(sourceMetadata, httpOut, val.(float64))
	}
	if val, valOk := edgeMetadata[tcp]; valOk {
		addToMetadataValue(sourceMetadata, tcpOut, val.(float64))
	}
}

// ResetOutgoingMetadata sets outgoing traffic to zero. This is useful for some graph type manipulations.
func ResetOutgoingMetadata(sourceMetadata Metadata) {
	delete(sourceMetadata, grpcOut)
	delete(sourceMetadata, httpOut)
	delete(sourceMetadata, tcpOut)
}

// AggregateNodeTraffic adds all <nodeMetadata> values (for all protocols) into aggregateNodeMetadata.
func AggregateNodeTraffic(node, aggregateNode *Node) {
	for _, protocol := range Protocols {
		for _, rate := range protocol.NodeRates {
			if val, ok := node.Metadata[rate.Name]; ok {
				addToMetadataValue(aggregateNode.Metadata, rate.Name, val.(float64))
			}
		}
	}
}

// AggregateEdgeTraffic is for aggregating edge traffic when reducing multiple edges into one edge (e.g.
// when generating service graph from workload graph, or aggregating serviceEntry nodes).
func AggregateEdgeTraffic(edge, aggregateEdge *Edge) {
	protocol, ok := edge.Metadata[ProtocolKey]
	if !ok {
		return
	}

	switch protocol {
	case grpc:
		if val, ok := edge.Metadata[grpc]; ok {
			addToMetadataValue(aggregateEdge.Metadata, grpc, val.(float64))
		}
		if val, ok := edge.Metadata[grpcErr]; ok {
			addToMetadataValue(aggregateEdge.Metadata, grpcErr, val.(float64))
		}
		if responses, ok := edge.Metadata[grpcResponses]; ok {
			addToResponses(aggregateEdge.Metadata, grpcResponses, responses.(Responses))
		}
	case http:
		if val, ok := edge.Metadata[http]; ok {
			addToMetadataValue(aggregateEdge.Metadata, http, val.(float64))
		}
		if val, ok := edge.Metadata[http3xx]; ok {
			addToMetadataValue(aggregateEdge.Metadata, http3xx, val.(float64))
		}
		if val, ok := edge.Metadata[http4xx]; ok {
			addToMetadataValue(aggregateEdge.Metadata, http4xx, val.(float64))
		}
		if val, ok := edge.Metadata[http5xx]; ok {
			addToMetadataValue(aggregateEdge.Metadata, http5xx, val.(float64))
		}
		if responses, ok := edge.Metadata[httpResponses]; ok {
			addToResponses(aggregateEdge.Metadata, httpResponses, responses.(Responses))
		}
	case tcp:
		if val, ok := edge.Metadata[tcp]; ok {
			addToMetadataValue(aggregateEdge.Metadata, tcp, val.(float64))
		}
		if responses, ok := edge.Metadata[tcpResponses]; ok {
			addToResponses(aggregateEdge.Metadata, tcpResponses, responses.(Responses))
		}

	default:
		Error(fmt.Sprintf("Unexpected edge protocol [%v] for edge [%+v]", protocol, aggregateEdge))
	}

	// handle any appender-based edge data (nothing currently)
	// note: We used to average response times of the aggregated edges but realized that
	// we can't average quantiles (kiali-2297).
}

func addToMetadataValue(md Metadata, k MetadataKey, v float64) {
	if v <= 0 || md == nil {
		return
	}
	if curr, ok := md[k]; ok {
		md[k] = curr.(float64) + v
	} else {
		md[k] = v
	}
}

// The metadata for response codes is two map of maps. Each response code is broken down by responseFlags:percentageOfTraffic and
// hosts:percentagOfTraffic like this:
// "200" : {
// 	  Flags: {
//      "-"  : 90.00,
//      "DC" : 10.00,
//    },
//    Hosts: {
//      "www.google.com" : 100.00
//    },
//  } ...

// ResponseFlags maps flags to request percentage
type ResponseFlags map[string]float64

// ResponseHosts maps hosts to request percentage
type ResponseHosts map[string]float64

// ResponseDetail consolidates response detail for a response code
type ResponseDetail struct {
	Flags ResponseFlags
	Hosts ResponseHosts
}

// Responses maps codes to ResponseDetail
type Responses map[string]*ResponseDetail

func addToResponses(md Metadata, k MetadataKey, responses Responses) {
	for code, detailsValMap := range responses {
		for flags, val := range detailsValMap.Flags {
			addToMetadataResponses(md, k, code, flags, "", val)
		}
		for host, val := range detailsValMap.Hosts {
			addToMetadataResponses(md, k, code, "", host, val)
		}
	}
}

func addToMetadataResponses(md Metadata, k MetadataKey, code, flags, host string, v float64) {
	if md == nil {
		return
	}

	var responses Responses
	var responseDetail *ResponseDetail
	var ok bool

	if code == "" {
		code = "-"
	}

	responses, ok = md[k].(Responses)
	if !ok {
		responses = Responses{}
		md[k] = responses
	}
	responseDetail, ok = responses[code]
	if !ok {
		responseDetail = &ResponseDetail{
			Flags: ResponseFlags{},
			Hosts: ResponseHosts{},
		}
		responses[code] = responseDetail
	}
	if flags != "" {
		responseDetail.Flags[flags] += v
	}
	if host != "" {
		responseDetail.Hosts[host] += v
	}
}

// averageMetadataValue is currently unused but shows how to perform averaging using metadata values.
//func averageMetadataValue(md Metadata, k string, v float64) {
//	total := v
//	count := 1.0
//	kTotal := k + "_total"
//	kCount := k + "_count"
//	if prevTotal, ok := md[kTotal]; ok {
//		total += prevTotal.(float64)
//	}
//	if prevCount, ok := md[kCount]; ok {
//		count += prevCount.(float64)
//	}
//	md[kTotal] = total
//	md[kCount] = count
//	md[k] = total / count
//}
