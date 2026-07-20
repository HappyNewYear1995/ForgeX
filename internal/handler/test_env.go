package handler

import (
	"encoding/json"
	"log"
	"net/http"
	"strconv"
	"sync"

	"jenkinsAgent/internal/service"
)

// recordingResult holds the outcome of an async recording session.
type recordingResult struct {
	done    bool
	err     error
	content string
}

type TestEnvHandler struct {
	configService     *service.ConfigService
	playwrightService *service.PlaywrightService
	recordingMap      sync.Map // envID -> *recordingResult
}

func NewTestEnvHandler(cs *service.ConfigService, ps *service.PlaywrightService) *TestEnvHandler {
	return &TestEnvHandler{configService: cs, playwrightService: ps}
}

// RecordScript starts recording asynchronously and returns immediately.
func (h *TestEnvHandler) RecordScript(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.Atoi(r.PathValue("id"))
	env, err := h.configService.GetTestEnv(uint(id))
	if err != nil {
		http.Error(w, "测试环境不存在", http.StatusNotFound)
		return
	}

	// Check if already recording
	if v, ok := h.recordingMap.Load(uint(id)); ok {
		if rec, ok := v.(*recordingResult); ok && !rec.done {
			http.Error(w, "录制正在进行中", http.StatusConflict)
			return
		}
	}

	rec := &recordingResult{}
	h.recordingMap.Store(uint(id), rec)

	log.Printf("[testenv] starting async recording for env #%d (%s)", env.ID, env.Name)

	go func() {
		content, err := h.playwrightService.StartRecording(env)
		if err != nil {
			log.Printf("[testenv] recording error for env #%d: %v", env.ID, err)
			rec.err = err
			rec.done = true
			return
		}
		if saveErr := h.playwrightService.SaveScript(env.ID, content); saveErr != nil {
			rec.err = saveErr
			rec.done = true
			return
		}
		log.Printf("[testenv] recording saved for env #%d (%d bytes)", env.ID, len(content))
		rec.content = content
		rec.done = true
	}()

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{"status": "recording"})
}

// RecordStatus polls the recording status for a test environment.
func (h *TestEnvHandler) RecordStatus(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.Atoi(r.PathValue("id"))

	v, ok := h.recordingMap.Load(uint(id))
	if !ok {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"done": true, "error": "未找到录制任务"})
		return
	}
	rec := v.(*recordingResult)
	resp := map[string]interface{}{"done": rec.done}
	if rec.done {
		defer h.recordingMap.Delete(uint(id))
		if rec.err != nil {
			resp["error"] = rec.err.Error()
		} else {
			resp["content"] = rec.content
		}
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}

// SaveScript saves the script content for a test environment.
func (h *TestEnvHandler) SaveScript(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "invalid form", http.StatusBadRequest)
		return
	}

	id, _ := strconv.Atoi(r.PathValue("id"))
	content := r.FormValue("script_content")

	if err := h.playwrightService.SaveScript(uint(id), content); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "保存失败: " + err.Error()})
		return
	}
	log.Printf("[testenv] script saved for env #%d", id)

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

// RunScript executes the test environment's script asynchronously.
func (h *TestEnvHandler) RunScript(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.Atoi(r.PathValue("id"))
	env, err := h.playwrightService.GetTestEnv(uint(id))
	if err != nil {
		http.Error(w, "测试环境不存在", http.StatusNotFound)
		return
	}

	log.Printf("[testenv] running script for env #%d (%s)", env.ID, env.Name)
	h.playwrightService.ExecuteScriptAsync(env)

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{"status": "running"})
}

// ScriptOutput returns the current execution status and output.
func (h *TestEnvHandler) ScriptOutput(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.Atoi(r.PathValue("id"))
	env, err := h.playwrightService.GetTestEnv(uint(id))
	if err != nil {
		http.Error(w, "测试环境不存在", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"status":   env.LastRunStatus,
		"output":   env.LastRunOutput,
		"last_run": env.LastRunAt,
	})
}
