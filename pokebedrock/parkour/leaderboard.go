package parkour

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/df-mc/dragonfly/server/entity"
	"github.com/df-mc/dragonfly/server/world"
	"github.com/go-gl/mathgl/mgl64"
	"github.com/sandertv/gophertunnel/minecraft/text"
)

// leaderboard ...
type leaderboard struct {
	Courses map[string]courseData `json:"courses"`

	path string
	mu   sync.Mutex
}

// courseData ...
type courseData struct {
	Best  map[string]int64  `json:"best"`
	Names map[string]string `json:"names"`
	Ranks map[string]string `json:"ranks"`
	Top   []Entry           `json:"top"`
}

// Entry ...
type Entry struct {
	XUID    string `json:"xuid"`
	Name    string `json:"name"`
	Rank    string `json:"rank"`
	TimeMS  int64  `json:"time_ms"`
	Updated int64  `json:"updated"`
}

// newLeaderboard ...
func newLeaderboard(path string) *leaderboard {
	lb := &leaderboard{
		path:    path,
		Courses: make(map[string]courseData),
	}
	_ = os.MkdirAll(filepath.Dir(path), os.ModePerm)
	_ = lb.load()
	return lb
}

// load ...
func (l *leaderboard) load() error {
	l.mu.Lock()
	defer l.mu.Unlock()

	data, err := os.ReadFile(l.path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	return json.Unmarshal(data, l)
}

// save ...
func (l *leaderboard) save() {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.saveLocked()
}

// saveLocked ...
func (l *leaderboard) saveLocked() {
	_ = os.MkdirAll(filepath.Dir(l.path), os.ModePerm)
	data, err := json.MarshalIndent(l, "", "  ")
	if err != nil {
		return
	}
	_ = os.WriteFile(l.path, data, os.ModePerm)
}

// update ...
func (l *leaderboard) update(courseID, xuid, name, rankName string, dur time.Duration) (placement int, prevBest time.Duration, err error) {
	l.mu.Lock()
	defer l.mu.Unlock()

	course := l.Courses[courseID]
	if course.Best == nil {
		course.Best = make(map[string]int64)
	}
	if course.Names == nil {
		course.Names = make(map[string]string)
	}
	if course.Ranks == nil {
		course.Ranks = make(map[string]string)
	}

	if old, ok := course.Best[xuid]; ok {
		prevBest = time.Duration(old) * time.Millisecond
		if old <= dur.Milliseconds() {
			course.Names[xuid] = name
			course.Ranks[xuid] = rankName
			course.Top = rebuildTop(course)
			l.Courses[courseID] = course
			l.saveLocked()
			return 0, prevBest, nil
		}
	}

	course.Best[xuid] = dur.Milliseconds()
	course.Names[xuid] = name
	course.Ranks[xuid] = rankName
	course.Top = rebuildTop(course)
	l.Courses[courseID] = course
	l.saveLocked()

	for i, entry := range course.Top {
		if entry.XUID == xuid {
			return i + 1, prevBest, nil
		}
	}
	return 0, prevBest, nil
}

// best ...
func (l *leaderboard) best(courseID, xuid string) time.Duration {
	l.mu.Lock()
	defer l.mu.Unlock()
	course, exists := l.Courses[courseID]
	if !exists {
		return 0
	}
	best, exists := course.Best[xuid]
	if !exists {
		return 0
	}
	return time.Duration(best) * time.Millisecond
}

// top ...
func (l *leaderboard) top(courseID string) []Entry {
	l.mu.Lock()
	defer l.mu.Unlock()
	course, exists := l.Courses[courseID]
	if !exists {
		return nil
	}
	return slices.Clone(course.Top)
}

// rebuildTop ...
func rebuildTop(course courseData) []Entry {
	type kv struct {
		xuid string
		time int64
	}
	list := make([]kv, 0, len(course.Best))
	for xuid, t := range course.Best {
		list = append(list, kv{xuid: xuid, time: t})
	}

	sort.Slice(list, func(i, j int) bool {
		return list[i].time < list[j].time
	})

	top := make([]Entry, 0, 10)
	for i, kv := range list {
		if i >= 10 {
			break
		}
		top = append(top, Entry{
			XUID:    kv.xuid,
			Name:    course.Names[kv.xuid],
			Rank:    course.Ranks[kv.xuid],
			TimeMS:  kv.time,
			Updated: time.Now().UnixMilli(),
		})
	}
	return top
}

// reset ...
func (l *leaderboard) reset(courseID string, xuid string) {
	l.mu.Lock()
	defer l.mu.Unlock()

	course, exists := l.Courses[courseID]
	if !exists {
		return
	}

	if xuid == "" {
		delete(l.Courses, courseID)
	} else {
		delete(course.Best, xuid)
		delete(course.Names, xuid)
		delete(course.Ranks, xuid)
		course.Top = rebuildTop(course)
		l.Courses[courseID] = course
	}

	l.saveLocked()
}

// Reset ...
func (m *Manager) Reset(courseID, xuid string) {
	m.lb.reset(courseID, xuid)
	m.updateLeaderboardText(courseID)
}

// updateLeaderboardText ...
func (m *Manager) updateLeaderboardText(courseID string) {
	course, ok := m.courses[courseID]
	if !ok {
		return
	}

	textContent := m.leaderboardTextContent(course)
	pos := course.Leaderboard.vec3().Add(mgl64.Vec3{0, 2.5})

	m.leaderboardMu.RLock()
	handle, exists := m.leaderboardTexts[courseID]
	m.leaderboardMu.RUnlock()

	m.w.Exec(func(tx *world.Tx) {
		if exists && handle != nil {
			if ent, ok := handle.Entity(tx); ok {
				if textEnt, ok := ent.(*entity.Ent); ok {
					textEnt.SetNameTag(textContent)
					return
				}
			}
			handle = nil
		}

		h := entity.NewText(textContent, pos)
		tx.AddEntity(h)
		m.leaderboardMu.Lock()
		m.leaderboardTexts[courseID] = h
		m.leaderboardMu.Unlock()
	})
}

// LeaderboardEntities ...
func (m *Manager) LeaderboardEntities() []*world.EntityHandle {
	m.leaderboardMu.RLock()
	defer m.leaderboardMu.RUnlock()

	handles := make([]*world.EntityHandle, 0, len(m.leaderboardTexts))
	for _, h := range m.leaderboardTexts {
		handles = append(handles, h)
	}
	return handles
}

// NPCHandles ...
func (m *Manager) NPCHandles() []*world.EntityHandle {
	m.npcMu.RLock()
	defer m.npcMu.RUnlock()

	handles := make([]*world.EntityHandle, 0, len(m.npcHandles))
	for _, h := range m.npcHandles {
		handles = append(handles, h)
	}
	return handles
}

// leaderboardTextContent ...
func (m *Manager) leaderboardTextContent(course CourseConfig) string {
	top := m.lb.top(course.Identifier)

	lines := make([]string, 0, 11)
	lines = append(lines, text.Colourf("<yellow>Top 10 for %s</yellow>", course.Name))

	for i := 0; i < 9; i++ {
		if i < len(top) {
			entry := top[i]

			label := entry.Name
			if entry.Rank != "" {
				label = fmt.Sprintf("[%s] %s", entry.Rank, entry.Name)
			}

			duration := formatDuration(time.Duration(entry.TimeMS) * time.Millisecond)
			lines = append(lines, text.Colourf("<grey>%d.</grey> %s - %s", i+1, label, duration))
			continue
		}

		lines = append(lines, text.Colourf("<grey>%d.</grey> - N/A", i+1))
	}

	return strings.Join(lines, "\n")
}
