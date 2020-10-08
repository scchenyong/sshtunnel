package sshtunnel

type Tunnel struct {
	Remote string `json:"remote"`
	Local  string `json:"local"`
}

type Config struct {
	Addr    string   `json:"addr"`
	User    string   `json:"user"`
	Pass    string   `json:"pass,omitempty"`
	Tunnels []Tunnel `json:"tunnels,omitempty"`
}
