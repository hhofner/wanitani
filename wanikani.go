package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

const wkBaseURL = "https://api.wanikani.com/v2"

// ── API response types ──────────────────────────────────────

type wkSubjectResponse struct {
	Object string `json:"object"` // "collection"
	Data   []wkSubject `json:"data"`
}

type wkSubject struct {
	ID     int    `json:"id"`
	Object string `json:"object"` // "radical", "kanji", "vocabulary", "kana_vocabulary"
	Data   wkSubjectData `json:"data"`
}

type wkSubjectData struct {
	Characters          *string              `json:"characters"` // nil for image radicals
	Meanings            []wkMeaning          `json:"meanings"`
	Readings            []wkReading          `json:"readings"`
	ComponentSubjectIDs []int                `json:"component_subject_ids"`
	MeaningMnemonic     string               `json:"meaning_mnemonic"`
	ReadingMnemonic     string               `json:"reading_mnemonic"`
	ContextSentences    []wkContextSentence  `json:"context_sentences"`
	PartsOfSpeech       []string             `json:"parts_of_speech"`
}

type wkMeaning struct {
	Meaning string `json:"meaning"`
	Primary bool   `json:"primary"`
}

type wkReading struct {
	Reading string `json:"reading"`
	Primary bool   `json:"primary"`
	Type    string `json:"type"` // "onyomi", "kunyomi", "nanori" for kanji
}

type wkContextSentence struct {
	EN string `json:"en"`
	JA string `json:"ja"`
}

// ── API helpers ─────────────────────────────────────────────

func wkGet(token string, path string) ([]byte, error) {
	req, err := http.NewRequest("GET", wkBaseURL+path, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Wanikani-Revision", "20170710")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("API error %d: %s", resp.StatusCode, string(body))
	}

	return body, nil
}

// fetchLessonSubjectIDs gets the subject IDs available for lessons from the summary.
func fetchLessonSubjectIDs(token string) ([]int, error) {
	body, err := wkGet(token, "/summary")
	if err != nil {
		return nil, err
	}

	var result wkSummaryResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, err
	}

	var ids []int
	for _, l := range result.Data.Lessons {
		ids = append(ids, l.SubjectIDs...)
	}
	return ids, nil
}

// fetchSubjects fetches subject details for the given IDs (max 1000 per request).
func fetchSubjects(token string, ids []int) ([]wkSubject, error) {
	if len(ids) == 0 {
		return nil, nil
	}

	// Build comma-separated ID list
	idStrs := make([]string, len(ids))
	for i, id := range ids {
		idStrs[i] = fmt.Sprintf("%d", id)
	}

	path := fmt.Sprintf("/subjects?ids=%s", strings.Join(idStrs, ","))
	body, err := wkGet(token, path)
	if err != nil {
		return nil, err
	}

	var result wkSubjectResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, err
	}

	return result.Data, nil
}

// fetchComponentNames fetches subject names for component IDs (to build composition text).
func fetchComponentNames(token string, ids []int) (map[int]string, error) {
	subjects, err := fetchSubjects(token, ids)
	if err != nil {
		return nil, err
	}

	names := make(map[int]string)
	for _, s := range subjects {
		char := "?"
		if s.Data.Characters != nil {
			char = *s.Data.Characters
		}
		// Find primary meaning
		for _, m := range s.Data.Meanings {
			if m.Primary {
				names[s.ID] = fmt.Sprintf("%s (%s)", char, m.Meaning)
				break
			}
		}
		if _, ok := names[s.ID]; !ok {
			names[s.ID] = char
		}
	}
	return names, nil
}

// ── Convert API subjects to lesson items ────────────────────

func subjectToLessonItem(s wkSubject, componentNames map[int]string) lessonItem {
	item := lessonItem{}
	item.subjectID = s.ID

	// Characters
	if s.Data.Characters != nil {
		item.characters = *s.Data.Characters
	} else {
		item.characters = "〇" // placeholder for image-only radicals
	}

	// Kind
	switch s.Object {
	case "radical":
		item.kind = itemRadical
	case "kanji":
		item.kind = itemKanji
	case "vocabulary", "kana_vocabulary":
		item.kind = itemVocabulary
	}

	// Meanings (accepted answers)
	for _, m := range s.Data.Meanings {
		item.meanings = append(item.meanings, m.Meaning)
	}

	// Readings
	for _, r := range s.Data.Readings {
		item.readings = append(item.readings, r.Reading)
	}

	// Composition
	if len(s.Data.ComponentSubjectIDs) > 0 && componentNames != nil {
		var parts []string
		for _, id := range s.Data.ComponentSubjectIDs {
			if name, ok := componentNames[id]; ok {
				parts = append(parts, name)
			}
		}
		if len(parts) > 0 {
			item.composition = strings.Join(parts, " + ")
		}
	}
	if s.Data.MeaningMnemonic != "" {
		if item.composition != "" {
			item.composition += "\n\n"
		}
		item.composition += s.Data.MeaningMnemonic
	}

	// Context
	if len(s.Data.ContextSentences) > 0 {
		var sentences []string
		for _, cs := range s.Data.ContextSentences {
			sentences = append(sentences, fmt.Sprintf("%s\n%s", cs.JA, cs.EN))
		}
		item.context = strings.Join(sentences, "\n\n")
	}
	if s.Data.ReadingMnemonic != "" {
		if item.context != "" {
			item.context = s.Data.ReadingMnemonic + "\n\n" + item.context
		} else {
			item.context = s.Data.ReadingMnemonic
		}
	}

	// Tabs (radicals don't have readings)
	if item.kind == itemRadical {
		item.tabs = []lessonTab{tabMeaning}
		if item.composition != "" {
			item.tabs = append([]lessonTab{tabComposition}, item.tabs...)
		}
		if item.context != "" {
			item.tabs = append(item.tabs, tabContext)
		}
	} else {
		item.tabs = []lessonTab{tabComposition, tabMeaning, tabReading, tabContext}
	}

	return item
}

// ── Tea commands for async fetching ─────────────────────────

type lessonsFetchedMsg struct {
	items []lessonItem
	err   error
}

func fetchLessonsCmd(token string, maxItems int) tea.Cmd {
	return func() tea.Msg {
		// 1. Get lesson subject IDs
		ids, err := fetchLessonSubjectIDs(token)
		if err != nil {
			return lessonsFetchedMsg{err: err}
		}

		if len(ids) == 0 {
			return lessonsFetchedMsg{err: fmt.Errorf("no lessons available")}
		}

		// Limit batch size (WaniKani does batches of 5)
		if maxItems > 0 && len(ids) > maxItems {
			ids = ids[:maxItems]
		}

		// 2. Fetch the subjects
		subjects, err := fetchSubjects(token, ids)
		if err != nil {
			return lessonsFetchedMsg{err: err}
		}

		// 3. Collect all component IDs to resolve names
		componentIDSet := make(map[int]bool)
		for _, s := range subjects {
			for _, cid := range s.Data.ComponentSubjectIDs {
				componentIDSet[cid] = true
			}
		}
		var componentIDs []int
		for id := range componentIDSet {
			componentIDs = append(componentIDs, id)
		}

		// 4. Fetch component names
		var componentNames map[int]string
		if len(componentIDs) > 0 {
			componentNames, err = fetchComponentNames(token, componentIDs)
			if err != nil {
				// Non-fatal: just skip composition info
				componentNames = nil
			}
		}

		// 5. Convert to lesson items
		var items []lessonItem
		for _, s := range subjects {
			items = append(items, subjectToLessonItem(s, componentNames))
		}

		return lessonsFetchedMsg{items: items}
	}
}

// ── Start assignments (mark lessons as completed) ───────────

type wkAssignmentsResponse struct {
	Data []struct {
		ID   int `json:"id"`
		Data struct {
			SubjectID int `json:"subject_id"`
		} `json:"data"`
	} `json:"data"`
}

// startAssignment tells WaniKani that a lesson has been completed.
func startAssignment(token string, assignmentID int) error {
	now := time.Now().UTC().Format("2006-01-02T15:04:05.000000Z")
	body := []byte(`{"started_at":"` + now + `"}`)

	req, err := http.NewRequest("PUT",
		fmt.Sprintf("%s/assignments/%d/start", wkBaseURL, assignmentID),
		bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Wanikani-Revision", "20170710")
	req.Header.Set("Content-Type", "application/json; charset=utf-8")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("start assignment %d: HTTP %d: %s", assignmentID, resp.StatusCode, string(respBody))
	}
	return nil
}

// fetchAssignmentIDsForSubjects gets assignment IDs for the given subject IDs.
func fetchAssignmentIDsForSubjects(token string, subjectIDs []int) (map[int]int, error) {
	idStrs := make([]string, len(subjectIDs))
	for i, id := range subjectIDs {
		idStrs[i] = fmt.Sprintf("%d", id)
	}

	path := fmt.Sprintf("/assignments?subject_ids=%s", strings.Join(idStrs, ","))
	body, err := wkGet(token, path)
	if err != nil {
		return nil, err
	}

	var result wkAssignmentsResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, err
	}

	// Map subject_id -> assignment_id
	m := make(map[int]int)
	for _, a := range result.Data {
		m[a.Data.SubjectID] = a.ID
	}
	return m, nil
}

type assignmentsStartedMsg struct {
	count int
	err   error
}

func startAssignmentsCmd(token string, subjectIDs []int) tea.Cmd {
	return func() tea.Msg {
		// 1. Get assignment IDs for these subjects
		assignmentMap, err := fetchAssignmentIDsForSubjects(token, subjectIDs)
		if err != nil {
			return assignmentsStartedMsg{err: err}
		}

		// 2. Start each assignment
		started := 0
		for _, assignmentID := range assignmentMap {
			if err := startAssignment(token, assignmentID); err != nil {
				// Some may already be started, continue
				continue
			}
			started++
		}

		return assignmentsStartedMsg{count: started}
	}
}
