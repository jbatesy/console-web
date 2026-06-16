package api

import (
	"encoding/json"
	"log"
	"net/http"

	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

type resizeMsg struct {
	Type string `json:"type"`
	Cols uint16 `json:"cols"`
	Rows uint16 `json:"rows"`
}

// RegisterWS adds the WebSocket route to an existing mux.
func (h *Handler) RegisterWS(mux *http.ServeMux) {
	mux.HandleFunc("GET /ws/pane/{id}", h.handleWS)
}

func (h *Handler) handleWS(w http.ResponseWriter, r *http.Request) {
	paneID := r.PathValue("id")

	pane, err := h.store.GetPane(paneID)
	if err != nil {
		http.Error(w, "pane not found", http.StatusNotFound)
		return
	}

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("ws upgrade: %v", err)
		return
	}
	defer conn.Close()
	conn.SetReadLimit(1 << 20) // 1 MB max incoming frame

	sendCh, doneCh, err := h.ptyMgr.AddClient(paneID, conn, pane.OutputPath)
	if err != nil {
		// PTY not running — send exited notification and close
		conn.WriteMessage(websocket.TextMessage, []byte(`{"type":"exited"}`))
		return
	}
	defer h.ptyMgr.RemoveClient(paneID, conn)

	// Goroutine: drain send channel → write to WebSocket
	writeErr := make(chan error, 1)
	go func() {
		for {
			select {
			case data, ok := <-sendCh:
				if !ok {
					writeErr <- nil
					return
				}
				if err := conn.WriteMessage(websocket.BinaryMessage, data); err != nil {
					writeErr <- err
					return
				}
			case <-doneCh:
				conn.WriteMessage(websocket.TextMessage, []byte(`{"type":"exited"}`))
				writeErr <- nil
				return
			}
		}
	}()

	// Main loop: read from WebSocket → PTY or resize
	for {
		msgType, data, err := conn.ReadMessage()
		if err != nil {
			break
		}
		switch msgType {
		case websocket.BinaryMessage:
			if err := h.ptyMgr.WriteInput(paneID, data); err != nil {
				log.Printf("pty write %s: %v", paneID, err)
			}
		case websocket.TextMessage:
			var msg resizeMsg
			if err := json.Unmarshal(data, &msg); err == nil && msg.Type == "resize" {
				if err := h.ptyMgr.Resize(paneID, msg.Cols, msg.Rows); err != nil {
					log.Printf("pty resize %s: %v", paneID, err)
				}
			}
		}
	}
	<-writeErr
}
