package api

type NetworkCheckInput struct {
	Body struct {
		Host string `json:"host" minLength:"1" maxLength:"253"`
		Port int    `json:"port" minimum:"1" maximum:"65535"`
		TLS  bool   `json:"tls"`
	}
}

type NetworkCheck struct {
	Reachable bool   `json:"reachable"`
	Detail    string `json:"detail"`
}
