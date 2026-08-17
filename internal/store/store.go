package store

import (
	"sync"
	"volunteer-scheduler/internal/domain"
)

// Store 线程安全的内存存储
type Store struct {
	mu            sync.RWMutex
	volunteers    map[string]*domain.Volunteer
	activities    map[string]*domain.Activity
	shifts        map[string]*domain.Shift
	registrations map[string]*domain.Registration
}

// NewStore 创建存储实例
func NewStore() *Store {
	return &Store{
		volunteers:    make(map[string]*domain.Volunteer),
		activities:    make(map[string]*domain.Activity),
		shifts:        make(map[string]*domain.Shift),
		registrations: make(map[string]*domain.Registration),
	}
}

// ==================== 志愿者 ====================

func (s *Store) SaveVolunteer(v *domain.Volunteer) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.volunteers[v.ID] = v
}

func (s *Store) GetVolunteer(id string) (*domain.Volunteer, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	v, ok := s.volunteers[id]
	return v, ok
}

// ==================== 活动与班次 ====================

func (s *Store) SaveActivity(a *domain.Activity) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.activities[a.ID] = a
	for _, sh := range a.Shifts {
		s.shifts[sh.ID] = sh
	}
}

func (s *Store) GetActivity(id string) (*domain.Activity, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	a, ok := s.activities[id]
	return a, ok
}

func (s *Store) GetShift(id string) (*domain.Shift, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	sh, ok := s.shifts[id]
	return sh, ok
}

func (s *Store) GetActivityShifts(activityID string) []*domain.Shift {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var result []*domain.Shift
	for _, sh := range s.shifts {
		if sh.ActivityID == activityID {
			result = append(result, sh)
		}
	}
	return result
}

// ==================== 报名 ====================

func (s *Store) SaveRegistration(r *domain.Registration) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.registrations[r.ID] = r
}

func (s *Store) GetRegistration(id string) (*domain.Registration, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	r, ok := s.registrations[id]
	return r, ok
}

func (s *Store) DeleteRegistration(id string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.registrations, id)
}

// FindRegistrationsByVolunteer 获取志愿者所有报名
type RegistrationFilter struct {
	VolunteerID string
	ShiftID     string
	State       *domain.RegistrationState
}

func (s *Store) FindRegistrations(filter RegistrationFilter) []*domain.Registration {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var result []*domain.Registration
	for _, r := range s.registrations {
		if filter.VolunteerID != "" && r.VolunteerID != filter.VolunteerID {
			continue
		}
		if filter.ShiftID != "" && r.ShiftID != filter.ShiftID {
			continue
		}
		if filter.State != nil && r.State != *filter.State {
			continue
		}
		result = append(result, r)
	}
	return result
}

// ListRegistrationsByShift 获取某班次的所有报名，按报名时间排序
func (s *Store) ListRegistrationsByShift(shiftID string) []*domain.Registration {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var result []*domain.Registration
	for _, r := range s.registrations {
		if r.ShiftID == shiftID {
			result = append(result, r)
		}
	}
	// 按 RegisteredAt 升序排序
	// 这里简单写排序逻辑，实际用 sort.Slice，但为减少 import 可以在 service 中排序，这里不排序，由 service 处理
	return result
}

// ListAllRegistrations 返回所有报名（用于测试或统计）
func (s *Store) ListAllRegistrations() []*domain.Registration {
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make([]*domain.Registration, 0, len(s.registrations))
	for _, r := range s.registrations {
		result = append(result, r)
	}
	return result
}
