package domain

import (
	"errors"
	"fmt"
	"strings"
	"time"
)

// RegistrationState 表示报名状态
type RegistrationState string

const (
	StateRegistered RegistrationState = "registered"
	StateConfirmed  RegistrationState = "confirmed"
	StateCheckedIn  RegistrationState = "checked_in"
	StateSettled    RegistrationState = "settled"
)

// AllowedTransitions 定义状态之间的合法转换
type AllowedTransitions struct {
	Next map[RegistrationState][]RegistrationState
}

// NewAllowedTransitions 返回默认状态机
type TransitionRules struct {
}

func (t TransitionRules) CanTransition(from, to RegistrationState) bool {
	switch from {
	case StateRegistered:
		return to == StateConfirmed
	case StateConfirmed:
		return to == StateCheckedIn || to == StateRegistered // 取消回退？不，这里不直接回退，取消是删除操作
	case StateCheckedIn:
		return to == StateSettled
	case StateSettled:
		return false
	}
	return false
}

// ValidateTransition 检查状态转换是否合法
func ValidateTransition(from, to RegistrationState) error {
	rules := TransitionRules{}
	if !rules.CanTransition(from, to) {
		return fmt.Errorf("invalid state transition from %s to %s", from, to)
	}
	return nil
}

// Volunteer 志愿者实体
type Volunteer struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Skills    []string  `json:"skills"`
	CreatedAt time.Time `json:"created_at"`
}

// HasSkill 检查志愿者是否拥有指定技能
func (v *Volunteer) HasSkill(skill string) bool {
	for _, s := range v.Skills {
		if strings.EqualFold(s, skill) {
			return true
		}
	}
	return false
}

// Shift 班次实体
type Shift struct {
	ID             string    `json:"id"`
	ActivityID     string    `json:"activity_id"`
	StartTime      time.Time `json:"start_time"`
	EndTime        time.Time `json:"end_time"`
	RequiredSkills []string  `json:"required_skills"`
	RequiredCount  int       `json:"required_count"`
}

// IsValid 校验班次基础合法性
func (s *Shift) IsValid() error {
	if s.StartTime.After(s.EndTime) {
		return errors.New("start time must be before end time")
	}
	if s.RequiredCount <= 0 {
		return errors.New("required count must be positive")
	}
	return nil
}

// Overlaps 判断两个班次时间是否重叠
func (s *Shift) Overlaps(other *Shift) bool {
	return s.StartTime.Before(other.EndTime) && other.StartTime.Before(s.EndTime)
}

// Activity 活动实体
type Activity struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Shifts    []*Shift  `json:"shifts"`
	CreatedAt time.Time `json:"created_at"`
}

// Registration 报名实体
type Registration struct {
	ID           string            `json:"id"`
	VolunteerID  string            `json:"volunteer_id"`
	ShiftID      string            `json:"shift_id"`
	State        RegistrationState `json:"state"`
	RegisteredAt time.Time         `json:"registered_at"`
	ConfirmedAt  *time.Time        `json:"confirmed_at,omitempty"`
	CheckedInAt  *time.Time        `json:"checked_in_at,omitempty"`
	SettledAt    *time.Time        `json:"settled_at,omitempty"`
}

// ActivityStatistics 活动统计结果
type ActivityStatistics struct {
	ActivityID     string  `json:"activity_id"`
	ShiftCount     int     `json:"shift_count"`
	TotalRequired  int     `json:"total_required"`
	ConfirmedCount int     `json:"confirmed_count"`
	CheckedInCount int     `json:"checked_in_count"`
	SettledCount   int     `json:"settled_count"`
	CoverageRate   float64 `json:"coverage_rate"`
	Shortage       int     `json:"shortage"`
}
