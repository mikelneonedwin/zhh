package protocol

import "encoding/json"

const (
	MsgIdentify     = "identify"
	MsgExec         = "exec"
	MsgExecStdout   = "exec_stdout"
	MsgExecStderr   = "exec_stderr"
	MsgExecDone     = "exec_done"
	MsgCD           = "cd"
	MsgShellSwitch  = "shell_switch"
	MsgShellList    = "shell_list"
	MsgFilePushReq  = "file_push_req"
	MsgFilePushData = "file_push_data"
	MsgFilePushEnd  = "file_push_end"
	MsgFilePushOK   = "file_push_ok"
	MsgFilePullReq  = "file_pull_req"
	MsgFilePullInfo = "file_pull_info"
	MsgFilePullData = "file_pull_data"
	MsgFilePullEnd  = "file_pull_end"
	MsgFilePullOK   = "file_pull_ok"
	MsgWhoami       = "whoami"
	MsgWhoamiResp   = "whoami_resp"
	MsgHeartbeat    = "heartbeat"
	MsgError        = "error"
)

type Message struct {
	Type    string          `json:"type"`
	Payload json.RawMessage `json:"payload,omitempty"`
}

type IdentifyPayload struct {
	Hostname string   `json:"hostname"`
	OS       string   `json:"os"`
	Version  string   `json:"version"`
	Shells   []string `json:"shells"`
	Octet    int      `json:"octet"`
	IP       string   `json:"ip"`
	Cwd      string   `json:"cwd"`
}

type ExecPayload struct {
	Cmd   string `json:"cmd"`
	Stdin string `json:"stdin,omitempty"`
}

type ExecOutputPayload struct {
	Data []byte `json:"data"`
}

type ExecDonePayload struct {
	Code int    `json:"code"`
	Cwd  string `json:"cwd"`
}

type CDPayload struct {
	Dir string `json:"dir"`
}

type ShellSwitchPayload struct {
	Shell string `json:"shell"`
}

type ShellListPayload struct {
	Shells []string `json:"shells"`
}

type FilePushReqPayload struct {
	Path string `json:"path"`
	Size int64  `json:"size"`
}

type FilePushDataPayload struct {
	Data []byte `json:"data"`
}

type FilePushOKPayload struct {
	Path string `json:"path"`
}

type FilePullReqPayload struct {
	Path string `json:"path"`
}

type FilePullInfoPayload struct {
	Size int64 `json:"size"`
}

type FilePullDataPayload struct {
	Data []byte `json:"data"`
}

type FilePullOKPayload struct {
	Path string `json:"path"`
}

type WhoamiRespPayload struct {
	User     string `json:"user"`
	Hostname string `json:"hostname"`
	IP       string `json:"ip"`
	OS       string `json:"os"`
	Version  string `json:"version"`
	Storage  string `json:"storage"`
	Battery  string `json:"battery"`
	Shell    string `json:"shell"`
	Cwd      string `json:"cwd"`
}

type ErrorPayload struct {
	Message string `json:"message"`
}

func NewMessage(msgType string, payload interface{}) *Message {
	if payload == nil {
		return &Message{Type: msgType}
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return &Message{Type: msgType}
	}
	return &Message{Type: msgType, Payload: json.RawMessage(data)}
}

func DecodePayload(raw json.RawMessage, v interface{}) error {
	return json.Unmarshal(raw, v)
}

