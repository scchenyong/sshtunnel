package sshtunnel

type Tunnel struct {
	IsInput bool   `json:"isInput"`
	Remote  string `json:"remote"`
	Local   string `json:"local"`
}

type Config struct {
	Addr    string   `json:"addr"`
	User    string   `json:"user"`
	Pass    string   `json:"pass,omitempty"`
	Timeout int      `json:"timeout"`
	Tunnels []Tunnel `json:"tunnels,omitempty"`
}
