package tui

import (
	"sort"
	"strings"

	"glplan/internal/model"
)

func (m Model) filteredTasks(colID string) []model.Task {
	out := []model.Task{}
	for _, t := range m.tasks {
		if colID != "" && t.ColumnID != colID {
			continue
		}
		if m.search != "" {
			q := strings.ToLower(m.search)
			if !strings.Contains(strings.ToLower(t.Title), q) &&
				!strings.Contains(strings.ToLower(t.Description), q) &&
				!strings.Contains(strings.ToLower(t.Ref), q) {
				continue
			}
		}
		out = append(out, t)
	}
	columnOrder := map[string]int{}
	if m.board != nil {
		for i, col := range m.board.Columns {
			columnOrder[col.ID] = i
		}
	}
	sort.SliceStable(out, func(i, j int) bool {
		if colID == "" && out[i].ColumnID != out[j].ColumnID {
			return columnOrder[out[i].ColumnID] < columnOrder[out[j].ColumnID]
		}
		if out[i].Position == out[j].Position {
			return out[i].CreatedAt.Before(out[j].CreatedAt)
		}
		return out[i].Position < out[j].Position
	})
	return out
}

func (m Model) currentColumn() *model.Column {
	if m.board == nil || len(m.board.Columns) == 0 || m.colIdx < 0 || m.colIdx >= len(m.board.Columns) {
		return nil
	}
	return &m.board.Columns[m.colIdx]
}

func (m *Model) clampCursors() {
	if m.board != nil {
		if len(m.board.Columns) == 0 {
			m.colIdx = 0
		} else {
			m.colIdx = clamp(m.colIdx, 0, len(m.board.Columns)-1)
		}
		valid := map[string]bool{}
		for _, col := range m.board.Columns {
			valid[col.ID] = true
			n := len(m.filteredTasks(col.ID))
			m.cursors[col.ID] = clamp(m.cursors[col.ID], 0, max(0, n-1))
			m.colScroll[col.ID] = clamp(m.colScroll[col.ID], 0, max(0, n-1))
		}
		for id := range m.cursors {
			if !valid[id] {
				delete(m.cursors, id)
				delete(m.colScroll, id)
			}
		}
		m.ensureColumnVisible()
		m.ensureCursorVisible()
	}
	if n := len(m.filteredTasks("")); n == 0 {
		m.listIdx = 0
		m.listStart = 0
	} else {
		m.listIdx = clamp(m.listIdx, 0, n-1)
		m.listStart = clamp(m.listStart, 0, n-1)
	}
}

func (m Model) cursor() int {
	if m.view == viewList {
		return m.listIdx
	}
	col := m.currentColumn()
	if col == nil {
		return 0
	}
	return m.cursors[col.ID]
}

func (m *Model) setCursor(v int) {
	if m.view == viewList {
		n := len(m.filteredTasks(""))
		m.listIdx = clamp(v, 0, max(0, n-1))
		m.ensureListVisible()
		return
	}
	col := m.currentColumn()
	if col == nil {
		return
	}
	n := len(m.filteredTasks(col.ID))
	m.cursors[col.ID] = clamp(v, 0, max(0, n-1))
	m.ensureCursorVisible()
}

func (m *Model) moveCursor(delta int) {
	if m.view == viewActivity {
		limit := max(0, len(m.activity)-m.activityPageSize())
		m.activityStart = clamp(m.activityStart+delta, 0, limit)
		return
	}
	m.setCursor(m.cursor() + delta)
}

func (m Model) selectedTask() *model.Task {
	var list []model.Task
	if m.view == viewList {
		list = m.filteredTasks("")
	} else if col := m.currentColumn(); col != nil {
		list = m.filteredTasks(col.ID)
	}
	i := m.cursor()
	if i < 0 || i >= len(list) {
		return nil
	}
	t := list[i]
	return &t
}

func (m *Model) ensureColumnVisible() {
	metrics := m.boardMetrics()
	if metrics.visibleCols <= 0 {
		return
	}
	if m.colIdx < m.colStart {
		m.colStart = m.colIdx
	}
	if m.colIdx >= m.colStart+metrics.visibleCols {
		m.colStart = m.colIdx - metrics.visibleCols + 1
	}
	if m.board != nil {
		m.colStart = clamp(m.colStart, 0, max(0, len(m.board.Columns)-metrics.visibleCols))
	}
}

func (m *Model) ensureCursorVisible() {
	col := m.currentColumn()
	if col == nil {
		return
	}
	metrics := m.boardMetrics()
	cursor := m.cursors[col.ID]
	scroll := m.colScroll[col.ID]
	if cursor < scroll {
		scroll = cursor
	}
	if cursor >= scroll+metrics.visibleCards {
		scroll = cursor - metrics.visibleCards + 1
	}
	n := len(m.filteredTasks(col.ID))
	scroll = clamp(scroll, 0, max(0, n-metrics.visibleCards))
	m.colScroll[col.ID] = scroll
}

func (m *Model) ensureListVisible() {
	page := m.listPageSize()
	if m.listIdx < m.listStart {
		m.listStart = m.listIdx
	}
	if m.listIdx >= m.listStart+page {
		m.listStart = m.listIdx - page + 1
	}
	m.listStart = clamp(m.listStart, 0, max(0, len(m.filteredTasks(""))-page))
}

func reorderPosition(tasks []model.Task, idx, delta int) (float64, bool) {
	if idx < 0 || idx >= len(tasks) {
		return 0, false
	}
	if delta < 0 {
		if idx == 0 {
			return 0, false
		}
		prev := tasks[idx-1]
		if idx-2 >= 0 {
			return (tasks[idx-2].Position + prev.Position) / 2, true
		}
		return prev.Position - 100, true
	}
	if delta > 0 {
		if idx+1 >= len(tasks) {
			return 0, false
		}
		next := tasks[idx+1]
		if idx+2 < len(tasks) {
			return (next.Position + tasks[idx+2].Position) / 2, true
		}
		return next.Position + 100, true
	}
	return 0, false
}
