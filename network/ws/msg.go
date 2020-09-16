package ws

/*
const (
	MsgOnline int = iota
	MsgConfig
	MsgGetConfig
	MsgReload
	MsgDisableInputs
	MsgEnableInputs
	MsgHeartbeat
)

type onlineMsg struct {
	Version         string    `json:"version"`
	OS              string    `json:"os"`
	Arch            string    `json:"arch"`
	Name            string    `json:"name"`
	EnabledInputs   []string  `json:"enabled_inputs"`
	AvailableInputs []string  `json:"available_inputs"`
	Docker          bool      `json:"docker"` // within docker?
	Token           string    `json:"token"`
	Uptime          time.Time `json:"uptime"`
	LastReload      time.Time `json:"reload_time"`
	ReloadCount     int       `json:"reload_count"`
}

type configMsg struct {
	Input  string `json:"input"`
	Config string `json:"config"`
}

type getConfigMsg struct {
	Inputs []string `json:"inputs"`
}

type disableInputMsg struct {
	Inputs []string `json:"inputs"`
}

type enableInputMsg struct {
	Inputs []string `json:"inputs"`
}

type datakitMsg struct {
	Type    int         `json:"type"`
	TraceID string      `json:"trace_id"`
	UUID    string      `json:"uuid"`
	Payload []byte      `json:"msgs"`
	Resp    interface{} `json:"resp"`
}

func handleDatakitMsg(c net.Conn, msg []byte, opcode ws.OpCode) error {

	l.Debugf("opcode %d", opcode)

	if !opcode.IsData() {
		return nil
	}

	totalMsgCnt++
	if totalMsgCnt%4096 == 0 {
		l.Debugf("total msg: %d, avg: %d", totalMsgCnt, totalMsgCnt/uint64(time.Since(up)/time.Second))
	}

	var m datakitMsg
	if err := json.Unmarshal(msg, &m); err != nil {
		l.Errorf("json.Unmarshal: %s", err.Error())
		return err
	}

	switch m.Type {
	case MsgOnline:
		return handleMsgOnline(c, &m)
	case MsgHeartbeat:
		return handleHeartbeat(&m)
	default:
		return handleDatakitResponse(&m)
	}

	return nil
	// This is commented out since in demo usage, stdout is showing messages sent from > 1M connections at very high rate
}

func handleMsgOnline(c net.Conn, m *datakitMsg) error {
	if m.Payload == nil {
		return fmt.Errorf("empty online msg")
	}

	l.Debugf("online msg(from %s): %s", m.UUID, string(m.Payload))

	wscliCh <- &wscli{
		uuid: m.UUID,
		conn: c,
		born: time.Now(),
	}

	// TODO: post to dataflux

	return nil
}

func handleHeartbeat(m *datakitMsg) error {
	hbCh <- m.UUID
	return nil
}

func handleDatakitResponse(m *datakitMsg) error {
	respCh <- m
	return nil
} */
