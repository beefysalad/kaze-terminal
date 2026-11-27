package ui

import (
	"strconv"
	"strings"
	"unicode/utf8"
)

type wsFilter int

const (
	wsAll wsFilter = iota
	wsOpenOnly
	wsClosedOnly
)

type wsListRow struct {
	ID, Name, Initial, Sub, State, Tone, RunLabel string
	Dots                                          []string
	Titles                                        string
	Tabs                                          []wsListTab
	Open                                          bool
}
type wsListTab struct{ Title, Folder, Cmd, More, Wait, Color string }
type wsListModel struct {
	Rows              []wsListRow
	Counts            [3]int
	ShowSearch, Empty bool
}

func buildWSList(w *window, filter wsFilter, query, open string) wsListModel {
	m := wsListModel{Empty: len(w.wsp.doc.Workspaces) == 0, ShowSearch: len(w.wsp.doc.Workspaces) > 5}
	query = strings.ToLower(strings.TrimSpace(query))
	for _, ws := range w.wsp.doc.Workspaces {
		state := w.wsBadge(ws.ID)
		closed := state == "Closed"
		m.Counts[0]++
		if closed {
			m.Counts[2]++
		} else {
			m.Counts[1]++
		}
		match := strings.Contains(strings.ToLower(ws.Name), query)
		for _, t := range ws.Tabs {
			match = match || strings.Contains(strings.ToLower(t.Title), query)
		}
		if !match || filter == wsOpenOnly && closed || filter == wsClosedOnly && !closed {
			continue
		}
		initial, _ := utf8.DecodeRuneInString(ws.Name)
		r := wsListRow{ID: ws.ID, Name: ws.Name, Initial: strings.ToUpper(string(initial)), Sub: plural(len(ws.Tabs), "tab"), State: state, Tone: "off", RunLabel: "Run", Open: open == ws.ID}
		if !closed {
			r.RunLabel = "Go to tabs"
			r.Tone = "run"
		}
		if ws.ID == w.opt.Config.StartupWorkspace {
			r.Sub += " · runs at startup"
		}
		var titles []string
		for _, t := range ws.Tabs {
			mem := w.listMember(t)
			tab := wsListTab{Title: mem.title, Folder: mem.folder, Cmd: "no commands", Wait: mem.tag, Color: mem.color}
			var cmds []string
			for _, c := range t.Commands {
				if strings.TrimSpace(c) != "" {
					cmds = append(cmds, c)
				}
			}
			if len(cmds) > 0 {
				tab.Cmd = cmds[0]
			}
			if len(cmds) > 1 {
				tab.More = "+" + strconv.Itoa(len(cmds)-1) + " more"
			}
			r.Tabs = append(r.Tabs, tab)
			r.Dots = append(r.Dots, t.Color)
			titles = append(titles, mem.title)
		}
		r.Titles = strings.Join(titles, ", ")
		m.Rows = append(m.Rows, r)
	}
	return m
}
