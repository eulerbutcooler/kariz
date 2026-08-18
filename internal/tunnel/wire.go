package tunnel

import (
	"bufio"
	"bytes"
	"encoding/json"
	"io"
)

type Register struct {
	Secret  string   `json:"secret"`
	Tunnels []Tunnel `json:"tunnels"`
}

type Tunnel struct {
	ID     string `json:"id"`
	Scheme string `json:"scheme"`
}

type Ack struct {
	Results []TunnelResult `json:"results"`
	Error   string         `json:"error,omitempty"`
}

type TunnelResult struct {
	ID    string `json:"id"`
	Host  string `json:"host,omitempty"`
	Error string `json:"error,omitempty"`
}

type Dial struct {
	TunnelID string `json:"tunnel_id"`
}

func WriteFrame(w io.Writer, v any) error {
	b, err := json.Marshal(v)
	if err != nil {
		return err
	}
	b = append(b, '\n')
	_, err = w.Write(b)
	return err
}

func ReadFrame(r *bufio.Reader, v any) error {
	line, err := r.ReadBytes('\n')
	if err != nil {
		return err
	}
	return json.Unmarshal(bytes.TrimRight(line, "\n"), v)
}
