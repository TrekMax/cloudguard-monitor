package output

// StatusDisplay is the structured status data for display.
type StatusDisplay struct {
	CPU     CPUStatus     `json:"cpu" yaml:"cpu"`
	Memory  MemoryStatus  `json:"memory" yaml:"memory"`
	Network NetworkStatus `json:"network" yaml:"network"`
}

type CPUStatus struct {
	Usage  float64 `json:"usage" yaml:"usage"`
	User   float64 `json:"user" yaml:"user"`
	System float64 `json:"system" yaml:"system"`
	Idle   float64 `json:"idle" yaml:"idle"`
	IOWait float64 `json:"iowait" yaml:"iowait"`
	Load1  float64 `json:"load1" yaml:"load1"`
	Load5  float64 `json:"load5" yaml:"load5"`
	Load15 float64 `json:"load15" yaml:"load15"`
	Cores  float64 `json:"cores" yaml:"cores"`
}

type MemoryStatus struct {
	Total        float64 `json:"total" yaml:"total"`
	Used         float64 `json:"used" yaml:"used"`
	Free         float64 `json:"free" yaml:"free"`
	Available    float64 `json:"available" yaml:"available"`
	UsagePercent float64 `json:"usage_percent" yaml:"usage_percent"`
	SwapTotal    float64 `json:"swap_total" yaml:"swap_total"`
	SwapUsed     float64 `json:"swap_used" yaml:"swap_used"`
	SwapPercent  float64 `json:"swap_percent" yaml:"swap_percent"`
}

type NetworkStatus struct {
	RxRate      float64 `json:"rx_bytes_rate" yaml:"rx_bytes_rate"`
	TxRate      float64 `json:"tx_bytes_rate" yaml:"tx_bytes_rate"`
	RxBytes     float64 `json:"rx_bytes" yaml:"rx_bytes"`
	TxBytes     float64 `json:"tx_bytes" yaml:"tx_bytes"`
	Connections float64 `json:"connections" yaml:"connections"`
}

// FromStatusData converts raw API status data to a display struct.
func FromStatusData(cpu, memory, network map[string]float64) *StatusDisplay {
	return &StatusDisplay{
		CPU: CPUStatus{
			Usage:  cpu["usage"],
			User:   cpu["user"],
			System: cpu["system"],
			Idle:   cpu["idle"],
			IOWait: cpu["iowait"],
			Load1:  cpu["load1"],
			Load5:  cpu["load5"],
			Load15: cpu["load15"],
			Cores:  cpu["cores"],
		},
		Memory: MemoryStatus{
			Total:        memory["total"],
			Used:         memory["used"],
			Free:         memory["free"],
			Available:    memory["available"],
			UsagePercent: memory["usage_percent"],
			SwapTotal:    memory["swap_total"],
			SwapUsed:     memory["swap_used"],
			SwapPercent:  memory["swap_percent"],
		},
		Network: NetworkStatus{
			RxRate:      network["rx_bytes_rate"],
			TxRate:      network["tx_bytes_rate"],
			RxBytes:     network["rx_bytes"],
			TxBytes:     network["tx_bytes"],
			Connections: network["connections"],
		},
	}
}
