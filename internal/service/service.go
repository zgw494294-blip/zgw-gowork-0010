package service

import (
	"errors"
	"fmt"
	"sort"
	"sync"
	"time"

	"volunteer-scheduler/internal/domain"
	"volunteer-scheduler/internal/store"
)

// Service 业务服务层
type Service struct {
	store *store.Store
	mu    sync.Mutex // 用于保证复合操作的原子性，避免并发报名冲突
}

// NewService 创建服务实例
func NewService(st *store.Store) *Service {
	if st == nil {
		st = store.NewStore()
	}
	return &Service{store: st}
}

// 以下是领域服务的公开方法

// CreateVolunteer 创建志愿者
func (s *Service) CreateVolunteer(name string, skills []string) (*domain.Volunteer, error) {
	if name == "" {
		return nil, errors.New("name is required")
	}
	v := &domain.Volunteer{
		ID:        generateID("v"),
		Name:      name,
		Skills:    skills,
		CreatedAt: time.Now(),
	}
	s.store.SaveVolunteer(v)
	return v, nil
}

// CreateActivity 创建活动及其班次
func (s *Service) CreateActivity(name string, shiftInputs []ShiftInput) (*domain.Activity, error) {
	if name == "" {
		return nil, errors.New("activity name is required")
	}
	activityID := generateID("a")
	activity := &domain.Activity{
		ID:        activityID,
		Name:      name,
		CreatedAt: time.Now(),
	}
	for i, input := range shiftInputs {
		shift := &domain.Shift{
			ID:             fmt.Sprintf("s%s-%d", activityID, i),
			ActivityID:     activityID,
			StartTime:      input.StartTime,
			EndTime:        input.EndTime,
			RequiredSkills: input.RequiredSkills,
			RequiredCount:  input.RequiredCount,
		}
		if err := shift.IsValid(); err != nil {
			return nil, err
		}
		activity.Shifts = append(activity.Shifts, shift)
	}
	s.store.SaveActivity(activity)
	return activity, nil
}

// ShiftInput 创建班次时的输入
type ShiftInput struct {
	StartTime      time.Time
	EndTime        time.Time
	RequiredSkills []string
	RequiredCount  int
}

// RegisterVolunteer 志愿者报名班次
func (s *Service) RegisterVolunteer(volunteerID, shiftID string) (*domain.Registration, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	// 检查志愿者是否存在
	volunteer, ok := s.store.GetVolunteer(volunteerID)
	if !ok {
		return nil, errors.New("volunteer not found")
	}
	shift, ok := s.store.GetShift(shiftID)
	if !ok {
		return nil, errors.New("shift not found")
	}
	// 技能校验
	if !hasSkillMatch(volunteer.Skills, shift.RequiredSkills) {
		return nil, errors.New("volunteer does not match required skills")
	}
	// 时间冲突检查：查找该志愿者所有非取消状态的报名，检查时间重叠
	registrations := s.store.FindRegistrations(store.RegistrationFilter{VolunteerID: volunteerID})
	for _, reg := range registrations {
		if reg.State == domain.StateSettled {
			continue
		}
		existingShift, _ := s.store.GetShift(reg.ShiftID)
		if existingShift != nil && existingShift.Overlaps(shift) {
			return nil, fmt.Errorf("time conflict with registration %s", reg.ID)
		}
	}
	// 创建报名，状态为 registered
	reg := &domain.Registration{
		ID:           generateID("r"),
		VolunteerID:  volunteerID,
		ShiftID:      shiftID,
		State:        domain.StateRegistered,
		RegisteredAt: time.Now(),
	}
	s.store.SaveRegistration(reg)
	return reg, nil
}

// ConfirmRegistration 确认报名：只能从 registered -> confirmed
func (s *Service) ConfirmRegistration(regID string) (*domain.Registration, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	reg, ok := s.store.GetRegistration(regID)
	if !ok {
		return nil, errors.New("registration not found")
	}
	if err := domain.ValidateTransition(reg.State, domain.StateConfirmed); err != nil {
		return nil, err
	}
	now := time.Now()
	reg.State = domain.StateConfirmed
	reg.ConfirmedAt = &now
	s.store.SaveRegistration(reg)
	return reg, nil
}

// CheckInRegistration 签到：只能从 confirmed -> checked_in
func (s *Service) CheckInRegistration(regID string) (*domain.Registration, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	reg, ok := s.store.GetRegistration(regID)
	if !ok {
		return nil, errors.New("registration not found")
	}
	if err := domain.ValidateTransition(reg.State, domain.StateCheckedIn); err != nil {
		return nil, err
	}
	now := time.Now()
	reg.State = domain.StateCheckedIn
	reg.CheckedInAt = &now
	s.store.SaveRegistration(reg)
	return reg, nil
}

// SettleRegistration 结算：只能从 checked_in -> settled
func (s *Service) SettleRegistration(regID string) (*domain.Registration, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	reg, ok := s.store.GetRegistration(regID)
	if !ok {
		return nil, errors.New("registration not found")
	}
	if err := domain.ValidateTransition(reg.State, domain.StateSettled); err != nil {
		return nil, err
	}
	now := time.Now()
	reg.State = domain.StateSettled
	reg.SettledAt = &now
	s.store.SaveRegistration(reg)
	return reg, nil
}

// CancelRegistration 取消报名：删除记录，并尝试递补
func (s *Service) CancelRegistration(regID string) (*domain.Registration, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	reg, ok := s.store.GetRegistration(regID)
	if !ok {
		return nil, errors.New("registration not found")
	}
	// 记录被取消的报名信息，用于返回
	cancelled := &domain.Registration{
		ID:           reg.ID,
		VolunteerID:  reg.VolunteerID,
		ShiftID:      reg.ShiftID,
		State:        "cancelled", // 实际不持久化，仅用于返回
		RegisteredAt: reg.RegisteredAt,
	}
	// 从存储中删除该报名
	s.store.DeleteRegistration(regID)
	// 如果被取消的是 confirmed 状态，则需要触发递补
	if reg.State == domain.StateConfirmed || reg.State == domain.StateCheckedIn {
		s.autoFill(reg.ShiftID)
	}
	return cancelled, nil
}

// autoFill 递补：从该班次所有 registered 状态的报名中选择最早者，转为 confirmed
func (s *Service) autoFill(shiftID string) {
	// 获取该班次所有报名
	registrations := s.store.ListRegistrationsByShift(shiftID)
	// 筛选 registered 状态并按报名时间排序
	var candidates []*domain.Registration
	for _, r := range registrations {
		if r.State == domain.StateRegistered {
			candidates = append(candidates, r)
		}
	}
	if len(candidates) == 0 {
		return
	}
	// 按 RegisteredAt 升序排序
	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].RegisteredAt.Before(candidates[j].RegisteredAt)
	})
	// 尝试递补第一个候选人
	candidate := candidates[0]
	volunteer, _ := s.store.GetVolunteer(candidate.VolunteerID)
	shift, _ := s.store.GetShift(candidate.ShiftID)
	if volunteer == nil || shift == nil {
		return
	}
	// 检查技能匹配（应当已满足，但保险起见再次校验）
	if !hasSkillMatch(volunteer.Skills, shift.RequiredSkills) {
		return
	}
	// 检查时间冲突（候选人可能已经报名了其他班次，需要检查）
	others := s.store.FindRegistrations(store.RegistrationFilter{VolunteerID: volunteer.ID})
	for _, other := range others {
		otherShift, _ := s.store.GetShift(other.ShiftID)
		if otherShift != nil && shift.Overlaps(otherShift) && other.ID != candidate.ID {
			return // 不能递补，因为冲突
		}
	}
	// 更新状态为 confirmed
	now := time.Now()
	candidate.State = domain.StateConfirmed
	candidate.ConfirmedAt = &now
	s.store.SaveRegistration(candidate)
}

// GetActivityStatistics 按活动统计覆盖率与缺口
func (s *Service) GetActivityStatistics(activityID string) (*domain.ActivityStatistics, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, ok := s.store.GetActivity(activityID)
	if !ok {
		return nil, errors.New("activity not found")
	}
	shifts := s.store.GetActivityShifts(activityID)
	if len(shifts) == 0 {
		return &domain.ActivityStatistics{
			ActivityID: activityID,
		}, nil
	}

	totalRequired := 0
	confirmedCount := 0
	checkedInCount := 0
	settledCount := 0

	// 遍历每个班次，统计每个班次的确认/签到/结算人数
	// 注意：这里的“已确认”包括 confirmed, checked_in, settled 三种状态？原描述“已确认+已签到”以及“确认人数”含义需明确。
	// 按照常规，覆盖率 = (已确认 + 已签到 + 已结算) / 所需人数，因为后两者也属于已确认。
	// 但我们统计三个独立数字，然后计算覆盖率为 (confirmed+checkedIn+settled) / totalRequired。
	// 但 confirmedCount 只计算处于 confirmed 状态的，checkedInCount 只计算 checked_in，settledCount 只计算 settled。
	// 这样三个数加起来才是有效人数。
	for _, shift := range shifts {
		totalRequired += shift.RequiredCount
		regs := s.store.FindRegistrations(store.RegistrationFilter{ShiftID: shift.ID})
		for _, r := range regs {
			switch r.State {
			case domain.StateConfirmed:
				confirmedCount++
			case domain.StateCheckedIn:
				checkedInCount++
			case domain.StateSettled:
				settledCount++
			}
		}
	}

	effective := confirmedCount + checkedInCount + settledCount
	shortage := totalRequired - effective
	if shortage < 0 {
		shortage = 0
	}
	coverageRate := 0.0
	if totalRequired > 0 {
		coverageRate = float64(effective) / float64(totalRequired)
	}

	return &domain.ActivityStatistics{
		ActivityID:     activityID,
		ShiftCount:     len(shifts),
		TotalRequired:  totalRequired,
		ConfirmedCount: confirmedCount,
		CheckedInCount: checkedInCount,
		SettledCount:   settledCount,
		CoverageRate:   coverageRate,
		Shortage:       shortage,
	}, nil
}

// 辅助函数

func generateID(prefix string) string {
	return fmt.Sprintf("%s-%d", prefix, time.Now().UnixNano())
}

// hasSkillMatch 检查志愿者技能是否满足班次要求（至少一个匹配）
func hasSkillMatch(volunteerSkills, requiredSkills []string) bool {
	for _, req := range requiredSkills {
		for _, vs := range volunteerSkills {
			if vs == req {
				return true
			}
		}
	}
	return false
}
