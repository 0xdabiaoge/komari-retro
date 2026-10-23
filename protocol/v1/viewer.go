package v1

import "time"

// ViewerReport deliberately excludes raw agent message, method and future fields.
type ViewerReport struct {
	UUID        string            `json:"uuid,omitempty"`
	CPU         CPUReport         `json:"cpu"`
	Ram         RamReport         `json:"ram"`
	Swap        RamReport         `json:"swap"`
	Load        LoadReport        `json:"load"`
	Disk        DiskReport        `json:"disk"`
	Network     NetworkReport     `json:"network"`
	Connections ConnectionsReport `json:"connections"`
	GPU         *GPUDetailReport  `json:"gpu,omitempty"`
	Uptime      int64             `json:"uptime"`
	Process     int               `json:"process"`
	UpdatedAt   time.Time         `json:"updated_at"`
}

func PublicReport(r Report) ViewerReport {
	return ViewerReport{r.UUID, r.CPU, r.Ram, r.Swap, r.Load, r.Disk, r.Network, r.Connections, r.GPU, r.Uptime, r.Process, r.UpdatedAt}
}
