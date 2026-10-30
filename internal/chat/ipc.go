package chat

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/FacileStudio/bulle/internal/settings"
)

// IPCRequest is an inbound IPC command sent to the chat daemon.
type IPCRequest struct {
	Action string `json:"action"`
	Room   string `json:"room,omitempty"`
	Text   string `json:"text,omitempty"`
	Prompt string `json:"prompt,omitempty"`

	Notify *bool    `json:"notify,omitempty"`
	Record bool     `json:"record,omitempty"`
	Items  []string `json:"items,omitempty"`

	RecordText string `json:"record_text,omitempty"`
}

// IPCSend delivers one outbound message. Notify nil means the message should
// notify; false asks for a quiet notice. Record appends the text to the room's
// conversation, and Items names the ledger keys the message is about so a
// reaction can be routed back to them.
type IPCSend func(ctx context.Context, req IPCRequest) error

// IPCTrigger runs one prompt through a room's session and returns the reply.
type IPCTrigger func(ctx context.Context, room, prompt string) (string, error)

// IPCResponse is the reply returned by the chat daemon.
type IPCResponse struct {
	OK    bool   `json:"ok"`
	Reply string `json:"reply,omitempty"`
	Error string `json:"error,omitempty"`
}

// SocketPath returns the default Unix domain socket path for bulle chat IPC.
func SocketPath() (string, error) {
	if sock := os.Getenv("BULLE_CHAT_SOCK"); sock != "" {
		return sock, nil
	}
	dir, err := settings.HomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "chat.sock"), nil
}

// ServeIPC runs the IPC Unix domain socket server until the context is canceled.
func ServeIPC(ctx context.Context, path string, send IPCSend, trigger IPCTrigger) error {
	if path == "" {
		var err error
		if path, err = SocketPath(); err != nil {
			return err
		}
	}
	ln, err := listenIPC(path)
	if err != nil {
		return err
	}
	defer func() {
		ln.Close()
		os.Remove(path)
	}()
	return acceptIPC(ctx, ln, send, trigger)
}

func listenIPC(path string) (*net.UnixListener, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return nil, err
	}
	if _, err := os.Stat(path); err == nil {
		conn, dialErr := net.DialTimeout("unix", path, 100*time.Millisecond)
		if dialErr == nil {
			conn.Close()
			return nil, fmt.Errorf("socket %s is already in use", path)
		}
		if err := os.Remove(path); err != nil {
			return nil, err
		}
	}
	ln, err := net.ListenUnix("unix", &net.UnixAddr{Name: path, Net: "unix"})
	if err != nil {
		return nil, err
	}
	if err := os.Chmod(path, 0o600); err != nil {
		ln.Close()
		os.Remove(path)
		return nil, err
	}
	return ln, nil
}

func acceptIPC(ctx context.Context, ln *net.UnixListener, send IPCSend, trigger IPCTrigger) error {
	var wg sync.WaitGroup
	defer wg.Wait()
	stop := context.AfterFunc(ctx, func() {
		ln.Close()
	})
	defer stop()
	for {
		conn, err := ln.Accept()
		if err != nil {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			return err
		}
		wg.Add(1)
		go func(c net.Conn) {
			defer wg.Done()
			handleIPCConn(ctx, c, send, trigger)
		}(conn)
	}
}

func handleIPCConn(ctx context.Context, conn net.Conn, send IPCSend, trigger IPCTrigger) {
	defer conn.Close()
	stop := context.AfterFunc(ctx, func() {
		conn.Close()
	})
	defer stop()
	dec := json.NewDecoder(conn)
	enc := json.NewEncoder(conn)
	for {
		var req IPCRequest
		if err := dec.Decode(&req); err != nil {
			return
		}
		resp := handleIPCRequest(ctx, req, send, trigger)
		if err := enc.Encode(resp); err != nil {
			return
		}
	}
}

func handleIPCRequest(ctx context.Context, req IPCRequest, send IPCSend, trigger IPCTrigger) IPCResponse {
	switch req.Action {
	case "send":
		if send == nil {
			return IPCResponse{OK: false, Error: "send handler not configured"}
		}
		if err := send(ctx, req); err != nil {
			return IPCResponse{OK: false, Error: err.Error()}
		}
		return IPCResponse{OK: true}
	case "trigger":
		if trigger == nil {
			return IPCResponse{OK: false, Error: "trigger handler not configured"}
		}
		reply, err := trigger(ctx, req.Room, req.Prompt)
		if err != nil {
			return IPCResponse{OK: false, Error: err.Error()}
		}
		return IPCResponse{OK: true, Reply: reply}
	default:
		return IPCResponse{OK: false, Error: fmt.Sprintf("unknown action: %s", req.Action)}
	}
}

// SendIPCTo dials a specific Unix domain socket path and exchanges one request and response.
func SendIPCTo(ctx context.Context, path string, req IPCRequest) (IPCResponse, error) {
	dialCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	var d net.Dialer
	conn, err := d.DialContext(dialCtx, "unix", path)
	if err != nil {
		return IPCResponse{}, err
	}
	defer conn.Close()
	stop := context.AfterFunc(ctx, func() {
		conn.Close()
	})
	defer stop()
	if err := json.NewEncoder(conn).Encode(req); err != nil {
		return IPCResponse{}, err
	}
	var resp IPCResponse
	if err := json.NewDecoder(conn).Decode(&resp); err != nil {
		return IPCResponse{}, err
	}
	return resp, nil
}

// SendIPC dials the default chat Unix domain socket and exchanges one request and response.
func SendIPC(ctx context.Context, req IPCRequest) (IPCResponse, error) {
	path, err := SocketPath()
	if err != nil {
		return IPCResponse{}, err
	}
	return SendIPCTo(ctx, path, req)
}
