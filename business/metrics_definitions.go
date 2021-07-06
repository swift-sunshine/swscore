package business

type istioMetric struct {
	kialiName      string
	istioName      string
	isHisto        bool
	useErrorLabels bool
}

var istioMetrics = []istioMetric{
	{
		kialiName: "grpc_received",
		istioName: "istio_response_messages_total",
		isHisto:   false,
	},
	{
		kialiName: "grpc_sent",
		istioName: "istio_request_messages_total",
		isHisto:   false,
	},
	{
		kialiName: "request_count",
		istioName: "istio_requests_total",
		isHisto:   false,
	},
	{
		kialiName:      "request_error_count",
		istioName:      "istio_requests_total",
		isHisto:        false,
		useErrorLabels: true,
	},
	{
		kialiName: "request_duration_millis",
		istioName: "istio_request_duration_milliseconds",
		isHisto:   true,
	},
	{
		kialiName: "request_throughput",
		istioName: "istio_request_bytes_sum",
		isHisto:   false,
	},
	{
		kialiName: "response_throughput",
		istioName: "istio_response_bytes_sum",
		isHisto:   false,
	},
	{
		kialiName: "request_size",
		istioName: "istio_request_bytes",
		isHisto:   true,
	},
	{
		kialiName: "response_size",
		istioName: "istio_response_bytes",
		isHisto:   true,
	},
	{
		kialiName: "tcp_received",
		istioName: "istio_tcp_received_bytes_total",
		isHisto:   false,
	},
	{
		kialiName: "tcp_sent",
		istioName: "istio_tcp_sent_bytes_total",
		isHisto:   false,
	},
}

func (in *istioMetric) labelsToUse(labels string, labelsError []string) []string {
	if in.useErrorLabels {
		return labelsError
	}
	return []string{labels}
}
