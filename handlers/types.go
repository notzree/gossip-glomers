package handlers

type MaelstromMessage struct {
	Src  string `json:"src"`
	Dest string `json:"dest"`
}

type TopologyBody struct {
	Type     string              `json:"type"`
	Topology map[string][]string `json:"topology"`
}

type Readbody struct {
	Type string `json:"type"`
}

type BroadcastBody struct {
	Type      string  `json:"type"`
	Message   int     `json:"message"`
	MessageId *string `json:"message_id,omitempty"`
	Ttl       *int    `json:"ttl,omitempty"`
}
