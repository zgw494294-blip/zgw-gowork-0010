package httpapi

import (
	"encoding/json"
	"net/http"
	"time"

	"volunteer-scheduler/internal/domain"
	"volunteer-scheduler/internal/service"
)

// Handler HTTP 处理器
type Handler struct {
	svc *service.Service
}

// NewHandler 创建 HTTP 处理器
func NewHandler(svc *service.Service) *Handler {
	return &Handler{svc: svc}
}

// RegisterRoutes 注册路由
func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/api/volunteers", h.handleCreateVolunteer)
	mux.HandleFunc("/api/activities", h.handleCreateActivity)
	mux.HandleFunc("/api/registrations", h.handleCreateRegistration)
	mux.HandleFunc("/api/registrations/", h.handleRegistrationActions)
	mux.HandleFunc("/api/activities/", h.handleActivityStats)
}

// ---- 请求/响应结构 ----

type createVolunteerReq struct {
	Name   string   `json:"name"`
	Skills []string `json:"skills"`
}

type createActivityReq struct {
	Name   string       `json:"name"`
	Shifts []shiftInput `json:"shifts"`
}

type shiftInput struct {
	StartTime      string   `json:"start_time"`
	EndTime        string   `json:"end_time"`
	RequiredSkills []string `json:"required_skills"`
	RequiredCount  int      `json:"required_count"`
}

type createRegistrationReq struct {
	VolunteerID string `json:"volunteer_id"`
	ShiftID     string `json:"shift_id"`
}

type errorResponse struct {
	Error string `json:"error"`
}

// ---- HTTP 处理函数 ----

func (h *Handler) handleCreateVolunteer(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	var req createVolunteerReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	v, err := h.svc.CreateVolunteer(req.Name, req.Skills)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, v)
}

func (h *Handler) handleCreateActivity(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	var req createActivityReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	// 解析时间
	shifts := make([]service.ShiftInput, 0, len(req.Shifts))
	for _, s := range req.Shifts {
		start, err := time.Parse(time.RFC3339, s.StartTime)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid start_time format, use RFC3339")
			return
		}
		end, err := time.Parse(time.RFC3339, s.EndTime)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid end_time format, use RFC3339")
			return
		}
		shifts = append(shifts, service.ShiftInput{
			StartTime:      start,
			EndTime:        end,
			RequiredSkills: s.RequiredSkills,
			RequiredCount:  s.RequiredCount,
		})
	}
	a, err := h.svc.CreateActivity(req.Name, shifts)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, a)
}

func (h *Handler) handleCreateRegistration(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	var req createRegistrationReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	reg, err := h.svc.RegisterVolunteer(req.VolunteerID, req.ShiftID)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, reg)
}

func (h *Handler) handleRegistrationActions(w http.ResponseWriter, r *http.Request) {
	// 处理 /api/registrations/{id}/action 形式
	path := r.URL.Path // e.g. /api/registrations/r123/confirm
	// 提取 id 和 action
	parts := splitPath(path)
	if len(parts) < 3 {
		writeError(w, http.StatusNotFound, "not found")
		return
	}
	id := parts[2]
	action := ""
	if len(parts) >= 4 {
		action = parts[3]
	}
	// 根据方法处理
	switch {
	case r.Method == http.MethodDelete && len(parts) == 3:
		// 取消报名 DELETE /api/registrations/{id}
		reg, err := h.svc.CancelRegistration(id)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, reg)
	case r.Method == http.MethodPost && len(parts) == 4:
		var err error
		var reg *domain.Registration
		switch action {
		case "confirm":
			reg, err = h.svc.ConfirmRegistration(id)
		case "checkin":
			reg, err = h.svc.CheckInRegistration(id)
		case "settle":
			reg, err = h.svc.SettleRegistration(id)
		default:
			writeError(w, http.StatusNotFound, "action not found")
			return
		}
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, reg)
	default:
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

func (h *Handler) handleActivityStats(w http.ResponseWriter, r *http.Request) {
	// GET /api/activities/{id}/statistics
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	path := r.URL.Path
	parts := splitPath(path)
	if len(parts) < 4 || parts[3] != "statistics" {
		writeError(w, http.StatusNotFound, "not found")
		return
	}
	id := parts[2]
	stats, err := h.svc.GetActivityStatistics(id)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, stats)
}

// splitPath 拆分 URL 路径，忽略空字符串
func splitPath(path string) []string {
	var parts []string
	for _, p := range split(path) {
		if p != "" {
			parts = append(parts, p)
		}
	}
	return parts
}

func split(s string) []string {
	// 简单分割，无需处理转义
	var result []string
	start := 0
	for i, r := range s {
		if r == '/' {
			result = append(result, s[start:i])
			start = i + 1
		}
	}
	result = append(result, s[start:])
	return result
}

func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, errorResponse{Error: msg})
}
