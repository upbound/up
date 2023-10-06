package controlplane

// Response is a normalized ControlPlane response.
// NOTE(tnthornton) this is expected to be different in the near future as
// cloud and spaces APIs converge.
type Response struct {
	ID      string
	Name    string
	Message string
	Status  string

	Cfg       string
	CfgStatus string

	ConnName      string
	ConnNamespace string
}
