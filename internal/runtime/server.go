package runtime

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"
)

// handler returns the HTTP multiplexer for the runtime contract:
//
//	GET  /health         liveness
//	GET  /status         worker / queue summary
//	GET  /tasks          list every task
//	POST /tasks          submit a new task
//	GET  /tasks/{id}     read one task
func (r *Runtime) handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/health", r.handleHealth)
	mux.HandleFunc("/status", r.handleStatus)
	mux.HandleFunc("/tasks", r.handleTasks)
	mux.HandleFunc("/tasks/", r.handleTaskByID)
	return mux
}

func (r *Runtime) handleHealth(w http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "use GET")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"status":  "ok",
		"agent":   r.config.Name,
		"started": r.startedAt.Format(time.RFC3339),
	})
}

func (r *Runtime) handleStatus(w http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "use GET")
		return
	}
	counts := r.store.Counts()
	writeJSON(w, http.StatusOK, map[string]any{
		"agent":    r.config.Name,
		"role":     r.config.Type,
		"loop":     map[string]any{"name": r.config.Loop.Name, "max_steps": r.config.Loop.MaxSteps},
		"provider": r.model.Provider(),
		"queued":   counts[StatusQueued],
		"running":  counts[StatusRunning],
		"done":     counts[StatusDone],
		"error":    counts[StatusError],
	})
}

func (r *Runtime) handleTasks(w http.ResponseWriter, request *http.Request) {
	switch request.Method {
	case http.MethodGet:
		writeJSON(w, http.StatusOK, map[string]any{"tasks": r.store.List()})
	case http.MethodPost:
		var body struct {
			Prompt string `json:"prompt"`
			System string `json:"system"`
		}
		if err := json.NewDecoder(request.Body).Decode(&body); err != nil {
			writeError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
			return
		}
		task, err := r.store.Submit(body.Prompt, body.System)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusAccepted, task)
	default:
		writeError(w, http.StatusMethodNotAllowed, "use GET or POST")
	}
}

func (r *Runtime) handleTaskByID(w http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "use GET")
		return
	}
	id := strings.TrimPrefix(request.URL.Path, "/tasks/")
	if id == "" || strings.Contains(id, "/") {
		writeError(w, http.StatusBadRequest, "task id required")
		return
	}
	task, err := r.store.Get(id)
	if errors.Is(err, ErrNotFound) {
		writeError(w, http.StatusNotFound, "task not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, task)
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	encoder := json.NewEncoder(w)
	_ = encoder.Encode(value)
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}
