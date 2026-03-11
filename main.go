package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

var version = "dev"

type autoRefreshMsg struct{}

func autoRefreshCmd() tea.Cmd {
	return tea.Tick(10*time.Minute, func(t time.Time) tea.Msg {
		return autoRefreshMsg{}
	})
}

// ── Item types ──────────────────────────────────────────────

type itemType int

const (
	itemRadical itemType = iota
	itemKanji
	itemVocabulary
)

func (t itemType) String() string {
	switch t {
	case itemRadical:
		return "Radical"
	case itemKanji:
		return "Kanji"
	case itemVocabulary:
		return "Vocabulary"
	}
	return ""
}

type questionType int

const (
	questionMeaning questionType = iota
	questionReading
)

func (q questionType) String() string {
	switch q {
	case questionMeaning:
		return "Meaning"
	case questionReading:
		return "Reading"
	}
	return ""
}

// ── Lesson content tabs ─────────────────────────────────────

type lessonTab int

const (
	tabComposition lessonTab = iota
	tabMeaning
	tabReading
	tabContext
)

func (t lessonTab) String() string {
	switch t {
	case tabComposition:
		return "Composition"
	case tabMeaning:
		return "Meaning"
	case tabReading:
		return "Reading"
	case tabContext:
		return "Context"
	}
	return ""
}

// ── Lesson item ─────────────────────────────────────────────

type lessonItem struct {
	subjectID   int
	characters  string
	kind        itemType
	meanings    []string
	readings    []string
	composition string // radicals that make up kanji, kanji that make up vocab
	context     string // example sentence or usage
	tabs        []lessonTab
}

func (l lessonItem) tabContent(tab lessonTab) string {
	switch tab {
	case tabComposition:
		if l.composition != "" {
			return l.composition
		}
		return "No composition data."
	case tabMeaning:
		if len(l.meanings) > 0 {
			return strings.Join(l.meanings, ", ")
		}
		return "No meanings."
	case tabReading:
		if len(l.readings) > 0 {
			return strings.Join(l.readings, ", ")
		}
		return "No readings."
	case tabContext:
		if l.context != "" {
			return l.context
		}
		return "No context."
	}
	return ""
}

// ── Quiz question ───────────────────────────────────────────

type quizQuestion struct {
	subjectID  int
	characters string
	kind       itemType
	question   questionType
	answers    []string // accepted answers
}

type reviewScore struct {
	incorrectMeaning int
	incorrectReading int
}

func (q quizQuestion) promptLabel() string {
	return fmt.Sprintf("%s %s", q.kind, q.question)
}

// ── App screens ─────────────────────────────────────────────

type screen int

const (
	screenDashboard screen = iota
	screenLesson
	screenQuiz
	screenReview
)

// ── Input modes ─────────────────────────────────────────────

type inputMode int

const (
	modeDefault inputMode = iota
	modeAwaitingToken
)

// ── Commands ────────────────────────────────────────────────

type command struct {
	name string
	desc string
}

var commands = []command{
	{name: "/help", desc: "Show available commands"},
	{name: "/add-token", desc: "Set your WaniKani API token"},
	{name: "/learn", desc: "Start a lesson session"},
	{name: "/review", desc: "Start a review session"},
	{name: "/refresh", desc: "Refresh lesson/review counts"},
	{name: "/quit", desc: "Exit WaniTani"},
}

// ── Mascot ──────────────────────────────────────────────────


// ── WaniKani API ────────────────────────────────────────────

type wkSummary struct {
	LessonCount int
	ReviewCount int
}

type wkSummaryResponse struct {
	Data struct {
		Lessons []struct {
			SubjectIDs []int `json:"subject_ids"`
		} `json:"lessons"`
		Reviews []struct {
			SubjectIDs []int `json:"subject_ids"`
		} `json:"reviews"`
	} `json:"data"`
}

func configDir() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".wanitani")
}

func tokenPath() string {
	return filepath.Join(configDir(), "token")
}

func saveToken(token string) error {
	dir := configDir()
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}
	return os.WriteFile(tokenPath(), []byte(token), 0600)
}

func loadToken() string {
	data, err := os.ReadFile(tokenPath())
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(data))
}

func fetchSummary(token string) (wkSummary, error) {
	req, err := http.NewRequest("GET", "https://api.wanikani.com/v2/summary", nil)
	if err != nil {
		return wkSummary{}, err
	}
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return wkSummary{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		return wkSummary{}, fmt.Errorf("API error %d: %s", resp.StatusCode, string(body))
	}

	var result wkSummaryResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return wkSummary{}, err
	}

	summary := wkSummary{}
	for _, l := range result.Data.Lessons {
		summary.LessonCount += len(l.SubjectIDs)
	}
	for _, r := range result.Data.Reviews {
		summary.ReviewCount += len(r.SubjectIDs)
	}
	return summary, nil
}

type summaryFetchedMsg struct {
	summary wkSummary
	err     error
}

func fetchSummaryCmd(token string) tea.Cmd {
	return func() tea.Msg {
		s, err := fetchSummary(token)
		return summaryFetchedMsg{summary: s, err: err}
	}
}

// ── Styles ──────────────────────────────────────────────────

var (
	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#FF86C8"))

	notAuthStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FFD54F")).
			Bold(true).
			Align(lipgloss.Center)

	notAuthSubStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#888888")).
			Align(lipgloss.Center)

	inputPrefixStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#FF86C8")).
				Bold(true)

	promptLabelStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#FFD54F")).
				Bold(true)

	statusStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#555555"))

	cmdNameStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FF86C8")).
			Bold(true)

	cmdDescStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#666666"))

	cmdSelectedStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#FFFFFF")).
				Background(lipgloss.Color("#FF86C8")).
				Bold(true)

	cmdSelectedDescStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#FFD4E8")).
				Background(lipgloss.Color("#FF86C8"))

	inputBoxFocusStyle = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(lipgloss.Color("#FF86C8")).
				Padding(0, 1)

	lessonCountStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#4A9BF5")).
				Bold(true)

	reviewCountStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#F54A8C")).
				Bold(true)

	countLabelStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#888888"))

	// Item type colors
	radicalColor = lipgloss.Color("#4A9BF5")
	kanjiColor   = lipgloss.Color("#F54A8C")
	vocabColor   = lipgloss.Color("#9B59F5")

	// Lesson-specific styles
	charDisplayStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color("#FFFFFF"))

	tabActiveStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FFFFFF")).
			Bold(true).
			Underline(true)

	tabInactiveStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#555555"))

	contentStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#CCCCCC")).
			Align(lipgloss.Center)

	dotFilledStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FFFFFF"))

	dotEmptyStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#444444"))

	quizPromptStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FFD54F")).
			Bold(true).
			Align(lipgloss.Center)

	correctStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#66BB6A")).
			Bold(true)

	incorrectStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FF4444")).
			Bold(true)

	hintStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#555555")).
			Italic(true)
)

func itemColor(kind itemType) lipgloss.Color {
	switch kind {
	case itemRadical:
		return radicalColor
	case itemKanji:
		return kanjiColor
	case itemVocabulary:
		return vocabColor
	}
	return lipgloss.Color("#FFFFFF")
}

// ── Model ───────────────────────────────────────────────────

type model struct {
	textInput      textinput.Model
	width          int
	height         int
	ready          bool
	authenticated  bool
	token          string

	statusMsg      string
	mode           inputMode
	summary        *wkSummary
	loadingSummary bool
	selectedCmd    int

	// Screen state
	screen screen

	// Lesson state
	lessonItems    []lessonItem
	lessonIdx      int // which item in the lesson batch
	lessonTabIdx   int // which content tab for current item
	loadingLessons bool

	// Quiz state (shared by lesson quiz and review)
	quizQuestions  []quizQuestion
	quizIdx        int
	quizResult     string // "correct", "incorrect", or ""
	romajiBuffer   string // full raw romaji input for reading questions

	// Review state
	loadingReviews    bool
	reviewIncorrect   map[int]*reviewScore // subjectID -> incorrect counts
}

func initialModel() model {
	ti := textinput.New()
	ti.Placeholder = "Type answer or /help"
	ti.Focus()
	ti.CharLimit = 256
	ti.Prompt = inputPrefixStyle.Render("❯ ")

	token := loadToken()
	authenticated := token != ""

	return model{
		textInput:     ti,
		authenticated: authenticated,
		token:         token,

		statusMsg:     "",
		mode:          modeDefault,
		selectedCmd:   -1,
		screen:        screenDashboard,
	}
}

func (m model) Init() tea.Cmd {
	cmds := []tea.Cmd{textinput.Blink}
	if m.authenticated {
		cmds = append(cmds, fetchSummaryCmd(m.token), autoRefreshCmd())
	}
	return tea.Batch(cmds...)
}

// ── Update ──────────────────────────────────────────────────

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case autoRefreshMsg:
		if m.authenticated {
			return m, tea.Batch(fetchSummaryCmd(m.token), autoRefreshCmd())
		}

	case summaryFetchedMsg:
		m.loadingSummary = false
		if msg.err != nil {
			m.statusMsg = fmt.Sprintf("API error: %s", msg.err)
		} else {
			m.summary = &msg.summary
			m.statusMsg = "Summary loaded."
		}

	case assignmentsStartedMsg:
		if msg.err != nil {
			m.statusMsg = fmt.Sprintf("Lesson complete! (Warning: failed to submit: %s)", msg.err)
		} else {
			m.statusMsg = fmt.Sprintf("Lesson complete! Started %d items on WaniKani.", msg.count)
		}
		return m, fetchSummaryCmd(m.token)

	case lessonsFetchedMsg:
		m.loadingLessons = false
		if msg.err != nil {
			m.statusMsg = fmt.Sprintf("Failed to load lessons: %s", msg.err)
		} else {
			m.lessonItems = msg.items
			m.lessonIdx = 0
			m.lessonTabIdx = 0
			m.screen = screenLesson
			m.statusMsg = ""
		}

	case reviewsFetchedMsg:
		m.loadingReviews = false
		if msg.err != nil {
			m.statusMsg = fmt.Sprintf("Failed to load reviews: %s", msg.err)
		} else if len(msg.questions) == 0 {
			m.statusMsg = "No reviews available right now."
		} else {
			m.quizQuestions = msg.questions
			m.quizIdx = 0
			m.quizResult = ""
			m.romajiBuffer = ""
			m.reviewIncorrect = make(map[int]*reviewScore)
			m.textInput.SetValue("")
			m.textInput.Placeholder = "Type your answer..."
			m.screen = screenReview
			m.statusMsg = ""
		}

	case reviewsSubmittedMsg:
		if msg.err != nil {
			m.statusMsg = fmt.Sprintf("Review complete! (Warning: failed to submit: %s)", msg.err)
		} else {
			m.statusMsg = fmt.Sprintf("Review complete! Submitted %d reviews.", msg.count)
		}
		return m, fetchSummaryCmd(m.token)

	case tea.KeyMsg:
		// Screen-specific key handling
		switch m.screen {
		case screenLesson:
			return m.updateLesson(msg)
		case screenQuiz:
			return m.updateQuiz(msg)
		case screenReview:
			return m.updateReview(msg)
		}

		// Dashboard key handling
		switch msg.Type {
		case tea.KeyCtrlC:
			return m, tea.Quit
		case tea.KeyEsc:
			if m.mode != modeDefault {
				m.mode = modeDefault
				m.textInput.Placeholder = "Type answer or /help"
				m.textInput.Prompt = inputPrefixStyle.Render("❯ ")
				m.textInput.SetValue("")
				m.textInput.EchoMode = textinput.EchoNormal
				m.statusMsg = "Cancelled."
			}
			m.selectedCmd = -1
		case tea.KeyTab:
			currentInput := m.textInput.Value()
			if m.mode == modeDefault && strings.HasPrefix(currentInput, "/") {
				matches := matchingCommands(currentInput)
				if len(matches) > 0 {
					m.selectedCmd++
					if m.selectedCmd >= len(matches) {
						m.selectedCmd = 0
					}
				}
			}
			return m, nil
		case tea.KeyShiftTab:
			currentInput := m.textInput.Value()
			if m.mode == modeDefault && strings.HasPrefix(currentInput, "/") {
				matches := matchingCommands(currentInput)
				if len(matches) > 0 {
					m.selectedCmd--
					if m.selectedCmd < 0 {
						m.selectedCmd = len(matches) - 1
					}
				}
			}
			return m, nil
		case tea.KeyEnter:
			// If a command is highlighted via Tab, execute it directly
			if m.selectedCmd >= 0 {
				currentInput := m.textInput.Value()
				matches := matchingCommands(currentInput)
				if m.selectedCmd < len(matches) {
					selected := matches[m.selectedCmd].name
					m.textInput.SetValue("")
					m.selectedCmd = -1
	
					return handleCommand(m, selected)
				}
			}

			input := strings.TrimSpace(m.textInput.Value())
			if input == "" {
				break
			}
			m.textInput.SetValue("")
			m.selectedCmd = -1

			if m.mode != modeDefault {
				return handleModeInput(m, input)
			} else if strings.HasPrefix(input, "/") {

				return handleCommand(m, input)
			}
		}

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.textInput.Width = msg.Width - 8
		m.ready = true
	}

	var prevVal string
	if keyMsg, ok := msg.(tea.KeyMsg); ok && keyMsg.Type != tea.KeyTab {
		prevVal = m.textInput.Value()
	}

	var cmd tea.Cmd
	m.textInput, cmd = m.textInput.Update(msg)
	cmds = append(cmds, cmd)

	if _, ok := msg.(tea.KeyMsg); ok && m.textInput.Value() != prevVal && prevVal != "" {
		m.selectedCmd = -1
	}

	return m, tea.Batch(cmds...)
}

func (m model) updateLesson(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyCtrlC:
		return m, tea.Quit
	case tea.KeyEsc:
		m.screen = screenDashboard
		m.statusMsg = "Left lesson."
		return m, fetchSummaryCmd(m.token)
	case tea.KeyTab:
		item := m.lessonItems[m.lessonIdx]
		m.lessonTabIdx++
		if m.lessonTabIdx >= len(item.tabs) {
			m.lessonTabIdx = 0
		}
	case tea.KeyShiftTab:
		item := m.lessonItems[m.lessonIdx]
		m.lessonTabIdx--
		if m.lessonTabIdx < 0 {
			m.lessonTabIdx = len(item.tabs) - 1
		}
	case tea.KeyEnter:
		if m.lessonIdx < len(m.lessonItems)-1 {
			m.lessonIdx++
			m.lessonTabIdx = 0
		} else {
			m.startQuiz()
		}
		return m, nil
	case tea.KeyBackspace, tea.KeyLeft:
		if m.lessonIdx > 0 {
			m.lessonIdx--
			m.lessonTabIdx = 0
		}
		return m, nil
	case tea.KeyRight:
		if m.lessonIdx < len(m.lessonItems)-1 {
			m.lessonIdx++
			m.lessonTabIdx = 0
		}
		return m, nil
	}
	return m, nil
}

func (m model) isReadingQuestion() bool {
	return m.quizIdx < len(m.quizQuestions) && m.quizQuestions[m.quizIdx].question == questionReading
}

func (m *model) syncRomajiDisplay() {
	converted, pending := romajiToHiragana(m.romajiBuffer)
	m.textInput.SetValue(converted + pending)
	m.textInput.CursorEnd()
}

func (m model) updateQuiz(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	isReading := m.isReadingQuestion()

	switch msg.Type {
	case tea.KeyCtrlC:
		return m, tea.Quit
	case tea.KeyEsc:
		m.screen = screenDashboard
		m.statusMsg = "Left quiz."
		m.romajiBuffer = ""
		return m, fetchSummaryCmd(m.token)
	case tea.KeyEnter:
		if m.quizResult != "" {
			wasIncorrect := m.quizResult == "incorrect"
			currentQ := m.quizQuestions[m.quizIdx]

			m.quizResult = ""
			m.quizIdx++
			m.textInput.SetValue("")
			m.romajiBuffer = ""

			// Re-queue incorrect answers at the end
			if wasIncorrect {
				m.quizQuestions = append(m.quizQuestions, currentQ)
			}

			if m.quizIdx >= len(m.quizQuestions) {
				// All done — start assignments on WaniKani
				m.screen = screenDashboard
				m.statusMsg = "Lesson complete! Submitting to WaniKani..."

				// Collect unique subject IDs
				idSet := make(map[int]bool)
				for _, item := range m.lessonItems {
					idSet[item.subjectID] = true
				}
				var subjectIDs []int
				for id := range idSet {
					subjectIDs = append(subjectIDs, id)
				}

				return m, startAssignmentsCmd(m.token, subjectIDs)
			}
			return m, nil
		}

		// For reading: finalize pending "n" as ん before checking
		if isReading && m.romajiBuffer != "" {
			finalized := m.romajiBuffer
			if strings.HasSuffix(finalized, "n") {
				finalized = finalized[:len(finalized)-1] + "nn"
			}
			converted, _ := romajiToHiragana(finalized)
			m.textInput.SetValue(converted)
		}

		input := strings.TrimSpace(m.textInput.Value())
		if input == "" {
			return m, nil
		}

		q := m.quizQuestions[m.quizIdx]
		correct := false
		inputLower := strings.ToLower(input)
		for _, a := range q.answers {
			if strings.ToLower(a) == inputLower {
				correct = true
				break
			}
		}

		if correct {
			m.quizResult = "correct"
		} else {
			m.quizResult = "incorrect"
		}
		m.romajiBuffer = ""
		return m, nil

	case tea.KeyBackspace:
		if isReading {
			if len(m.romajiBuffer) > 0 {
				m.romajiBuffer = m.romajiBuffer[:len(m.romajiBuffer)-1]
				m.syncRomajiDisplay()
			}
			return m, nil
		}

	case tea.KeyRunes:
		if isReading {
			typed := strings.ToLower(string(msg.Runes))
			m.romajiBuffer += typed
			m.syncRomajiDisplay()
			return m, nil
		}
	}

	// For meaning questions or other keys, pass to text input normally
	var cmd tea.Cmd
	m.textInput, cmd = m.textInput.Update(msg)
	return m, cmd
}

func (m *model) startQuiz() {
	m.screen = screenQuiz
	m.quizIdx = 0
	m.quizResult = ""
	m.romajiBuffer = ""
	m.textInput.SetValue("")
	m.textInput.Placeholder = "Type your answer..."

	var questions []quizQuestion
	for _, item := range m.lessonItems {
		// Meaning question for all types
		questions = append(questions, quizQuestion{
			subjectID:  item.subjectID,
			characters: item.characters,
			kind:       item.kind,
			question:   questionMeaning,
			answers:    item.meanings,
		})
		// Reading question for kanji and vocabulary
		if item.kind != itemRadical && len(item.readings) > 0 {
			questions = append(questions, quizQuestion{
				subjectID:  item.subjectID,
				characters: item.characters,
				kind:       item.kind,
				question:   questionReading,
				answers:    item.readings,
			})
		}
	}
	m.quizQuestions = questions
}

// ── View ────────────────────────────────────────────────────

func (m model) View() string {
	if !m.ready {
		return "Loading..."
	}

	switch m.screen {
	case screenLesson:
		return m.viewLesson()
	case screenQuiz:
		return m.viewQuiz()
	case screenReview:
		return m.viewReview()
	default:
		return m.viewDashboard()
	}
}

func (m model) viewDashboard() string {
	logo := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#FF86C8")).
		Render("██╗    ██╗ █████╗ ███╗  ██╗██╗████████╗ █████╗ ███╗  ██╗██╗") + "\n" +
		lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#FF69B4")).
			Render("██║    ██║██╔══██╗████╗ ██║██║╚══██╔══╝██╔══██╗████╗ ██║██║") + "\n" +
		lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#FF4DA6")).
			Render("██║ █╗ ██║███████║██╔██╗██║██║   ██║   ███████║██╔██╗██║██║") + "\n" +
		lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#FF3399")).
			Render("██║███╗██║██╔══██║██║╚████║██║   ██║   ██╔══██║██║╚████║██║") + "\n" +
		lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#FF1A8C")).
			Render("╚███╔███╔╝██║  ██║██║ ╚███║██║   ██║   ██║  ██║██║ ╚███║██║") + "\n" +
		lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#FF007F")).
			Render(" ╚══╝╚══╝ ╚═╝  ╚═╝╚═╝  ╚══╝╚═╝   ╚═╝   ╚═╝  ╚═╝╚═╝  ╚══╝╚═╝")

	var content string
	if !m.authenticated {
		content = renderNotAuthenticated()
	} else if m.loadingLessons {
		content = lipgloss.NewStyle().
			Align(lipgloss.Center).
			Width(m.width).
			Foreground(lipgloss.Color("#888888")).
			Render("Fetching lessons...")
	} else if m.loadingReviews {
		content = lipgloss.NewStyle().
			Align(lipgloss.Center).
			Width(m.width).
			Foreground(lipgloss.Color("#888888")).
			Render("Fetching reviews...")
	} else if m.summary != nil {
		content = renderDashboard(*m.summary)
	} else if m.loadingSummary {
		content = lipgloss.NewStyle().
			Align(lipgloss.Center).
			Width(m.width).
			Foreground(lipgloss.Color("#888888")).
			Render("Loading summary...")
	} else {
		content = lipgloss.NewStyle().
			Align(lipgloss.Center).
			Width(m.width).
			Foreground(lipgloss.Color("#888888")).
			Render("Type /help for commands.")
	}

	centeredLogo := lipgloss.NewStyle().
		Width(m.width).
		Align(lipgloss.Center).
		Render(logo)

	centeredBlock := lipgloss.NewStyle().
		Width(m.width).
		Align(lipgloss.Center).
		Render(centeredLogo + "\n\n" + content)

	status := statusStyle.Render("  " + m.statusMsg)

	maxSuggestionLines := len(commands)
	currentInput := m.textInput.Value()
	suggestions := ""
	if m.mode == modeDefault && strings.HasPrefix(currentInput, "/") {
		suggestions = renderCommandSuggestions(currentInput, m.selectedCmd)
	}

	promptLabel := ""
	if m.mode == modeAwaitingToken {
		promptLabel = promptLabelStyle.Render("  Enter your WaniKani API token:")
	}

	inputBox := inputBoxFocusStyle.
		Width(m.width - 4).
		Render(m.textInput.View())

	blockHeight := lipgloss.Height(centeredBlock)
	bottomLines := 4 + maxSuggestionLines
	if promptLabel != "" {
		bottomLines++
	}
	topPad := (m.height - blockHeight - bottomLines) / 2
	if topPad < 1 {
		topPad = 1
	}

	var suggestionBlock string
	if suggestions != "" {
		renderedLines := strings.Count(suggestions, "\n") + 1
		suggestionBlock = suggestions
		if renderedLines < maxSuggestionLines {
			suggestionBlock += strings.Repeat("\n", maxSuggestionLines-renderedLines)
		}
	} else {
		suggestionBlock = strings.Repeat("\n", maxSuggestionLines-1)
	}

	var b strings.Builder
	b.WriteString(strings.Repeat("\n", topPad))
	b.WriteString(centeredBlock)
	b.WriteString("\n")
	b.WriteString(status)
	b.WriteString("\n")
	if promptLabel != "" {
		b.WriteString(promptLabel)
		b.WriteString("\n")
	}
	b.WriteString(inputBox)
	b.WriteString("\n")
	b.WriteString(suggestionBlock)

	return b.String()
}

func (m model) viewLesson() string {
	if len(m.lessonItems) == 0 {
		return "No lesson items."
	}

	item := m.lessonItems[m.lessonIdx]
	color := itemColor(item.kind)

	// ── Top: kind label + large characters ──
	kindLabel := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#FFFFFF")).
		Background(color).
		Bold(true).
		Padding(0, 2).
		Render(" " + item.kind.String() + " ")

	kindBlock := lipgloss.NewStyle().
		Width(m.width).
		Align(lipgloss.Center).
		Render(kindLabel)

	characters := charDisplayStyle.
		Foreground(color).
		Render(item.characters)

	charBlock := lipgloss.NewStyle().
		Width(m.width).
		Align(lipgloss.Center).
		Render(characters)

	banner := lipgloss.JoinVertical(lipgloss.Center,
		kindBlock, "", "", charBlock, "")

	// ── Dots for lesson items ──
	var dots []string
	for i := range m.lessonItems {
		if i == m.lessonIdx {
			dots = append(dots, dotFilledStyle.Render("●"))
		} else {
			dots = append(dots, dotEmptyStyle.Render("○"))
		}
	}
	dotRow := lipgloss.NewStyle().
		Width(m.width).
		Align(lipgloss.Center).
		Render(strings.Join(dots, " "))

	// ── Tab bar ──
	var tabs []string
	for i, tab := range item.tabs {
		if i == m.lessonTabIdx {
			tabs = append(tabs, tabActiveStyle.Foreground(color).Render(tab.String()))
		} else {
			tabs = append(tabs, tabInactiveStyle.Render(tab.String()))
		}
	}
	tabBar := lipgloss.NewStyle().
		Width(m.width).
		Align(lipgloss.Center).
		Render(strings.Join(tabs, "    "))

	// ── Content ──
	currentTab := item.tabs[m.lessonTabIdx]
	contentText := contentStyle.
		Width(m.width - 8).
		Render(item.tabContent(currentTab))

	contentBlock := lipgloss.NewStyle().
		Width(m.width).
		Align(lipgloss.Center).
		Render(contentText)

	// ── Hints ──
	hints := hintStyle.Render("  Tab/Shift+Tab: switch section  |  Enter/→: next  |  ←/Backspace: prev  |  Esc: back")

	// ── Layout ──
	// Calculate spacing between content and hints
	usedHeight := lipgloss.Height(banner) + 1 + lipgloss.Height(dotRow) + 1 +
		lipgloss.Height(tabBar) + 1 + lipgloss.Height(contentBlock) + 1 +
		lipgloss.Height(hints)
	spacer := m.height - usedHeight
	if spacer < 1 {
		spacer = 1
	}

	var b strings.Builder
	b.WriteString(banner)
	b.WriteString("\n")
	b.WriteString(dotRow)
	b.WriteString("\n\n")
	b.WriteString(tabBar)
	b.WriteString("\n\n")
	b.WriteString(contentBlock)
	b.WriteString("\n")
	b.WriteString(strings.Repeat("\n", spacer))
	b.WriteString(hints)

	return b.String()
}

func (m model) viewQuiz() string {
	if m.quizIdx >= len(m.quizQuestions) {
		return "Quiz complete!"
	}

	q := m.quizQuestions[m.quizIdx]
	color := itemColor(q.kind)

	// ── Top: kind label + large characters ──
	kindLabel := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#FFFFFF")).
		Background(color).
		Bold(true).
		Padding(0, 2).
		Render(" " + q.kind.String() + " ")

	kindBlock := lipgloss.NewStyle().
		Width(m.width).
		Align(lipgloss.Center).
		Render(kindLabel)

	characters := charDisplayStyle.
		Foreground(color).
		Render(q.characters)

	charBlock := lipgloss.NewStyle().
		Width(m.width).
		Align(lipgloss.Center).
		Render(characters)

	banner := lipgloss.JoinVertical(lipgloss.Center,
		kindBlock, "", "", charBlock, "")

	// ── Progress dots ──
	var dots []string
	for i := range m.quizQuestions {
		if i == m.quizIdx {
			dots = append(dots, dotFilledStyle.Render("●"))
		} else if i < m.quizIdx {
			dots = append(dots, correctStyle.Render("●"))
		} else {
			dots = append(dots, dotEmptyStyle.Render("○"))
		}
	}
	dotRow := lipgloss.NewStyle().
		Width(m.width).
		Align(lipgloss.Center).
		Render(strings.Join(dots, " "))

	// ── Prompt ──
	prompt := quizPromptStyle.
		Width(m.width).
		Render(q.promptLabel())

	// ── Result line (shown above input) ──
	resultLine := ""
	if m.quizResult != "" {
		var resultText string
		if m.quizResult == "correct" {
			resultText = correctStyle.Render("Correct!")
		} else {
			resultText = incorrectStyle.Render("Incorrect — ") +
				lipgloss.NewStyle().Foreground(lipgloss.Color("#CCCCCC")).Render(strings.Join(q.answers, ", "))
		}
		resultLine = lipgloss.NewStyle().
			Width(m.width).
			Align(lipgloss.Center).
			Render(resultText)
	}

	// ── Input box (always shown) ──
	inputBox := inputBoxFocusStyle.
		Width(m.width - 4).
		BorderForeground(color).
		Render(m.textInput.View())

	continueHint := ""
	if m.quizResult != "" {
		continueHint = lipgloss.NewStyle().
			Width(m.width).
			Align(lipgloss.Center).
			Render(hintStyle.Render("Press Enter to continue"))
	}

	// ── Layout: vertically center everything together ──
	var centerBlock strings.Builder
	centerBlock.WriteString(banner)
	centerBlock.WriteString("\n")
	centerBlock.WriteString(dotRow)
	centerBlock.WriteString("\n\n")
	centerBlock.WriteString(prompt)
	centerBlock.WriteString("\n\n")
	if resultLine != "" {
		centerBlock.WriteString(resultLine)
		centerBlock.WriteString("\n")
	}
	centerBlock.WriteString(inputBox)
	if continueHint != "" {
		centerBlock.WriteString("\n")
		centerBlock.WriteString(continueHint)
	}

	return centerBlock.String()
}

// ── Review ──────────────────────────────────────────────────

func (m model) updateReview(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	isReading := m.isReadingQuestion()

	switch msg.Type {
	case tea.KeyCtrlC:
		return m, tea.Quit
	case tea.KeyEsc:
		m.screen = screenDashboard
		m.statusMsg = "Left review."
		m.romajiBuffer = ""
		return m, fetchSummaryCmd(m.token)
	case tea.KeyEnter:
		if m.quizResult != "" {
			wasIncorrect := m.quizResult == "incorrect"
			currentQ := m.quizQuestions[m.quizIdx]

			// Track incorrect answers for SRS submission
			if wasIncorrect {
				score, ok := m.reviewIncorrect[currentQ.subjectID]
				if !ok {
					score = &reviewScore{}
					m.reviewIncorrect[currentQ.subjectID] = score
				}
				if currentQ.question == questionMeaning {
					score.incorrectMeaning++
				} else {
					score.incorrectReading++
				}
				// Re-queue
				m.quizQuestions = append(m.quizQuestions, currentQ)
			}

			m.quizResult = ""
			m.quizIdx++
			m.textInput.SetValue("")
			m.romajiBuffer = ""

			if m.quizIdx >= len(m.quizQuestions) {
				m.screen = screenDashboard
				m.statusMsg = "Review complete! Submitting to WaniKani..."
				return m, submitReviewsCmd(m.token, m.reviewIncorrect, m.quizQuestions)
			}
			return m, nil
		}

		// For reading: finalize pending "n" as ん before checking
		if isReading && m.romajiBuffer != "" {
			finalized := m.romajiBuffer
			if strings.HasSuffix(finalized, "n") {
				finalized = finalized[:len(finalized)-1] + "nn"
			}
			converted, _ := romajiToHiragana(finalized)
			m.textInput.SetValue(converted)
		}

		input := strings.TrimSpace(m.textInput.Value())
		if input == "" {
			return m, nil
		}

		q := m.quizQuestions[m.quizIdx]
		correct := false
		inputLower := strings.ToLower(input)
		for _, a := range q.answers {
			if strings.ToLower(a) == inputLower {
				correct = true
				break
			}
		}

		if correct {
			m.quizResult = "correct"
		} else {
			m.quizResult = "incorrect"
		}
		m.romajiBuffer = ""
		return m, nil

	case tea.KeyBackspace:
		if isReading {
			if len(m.romajiBuffer) > 0 {
				m.romajiBuffer = m.romajiBuffer[:len(m.romajiBuffer)-1]
				m.syncRomajiDisplay()
			}
			return m, nil
		}

	case tea.KeyRunes:
		if isReading {
			typed := strings.ToLower(string(msg.Runes))
			m.romajiBuffer += typed
			m.syncRomajiDisplay()
			return m, nil
		}
	}

	var cmd tea.Cmd
	m.textInput, cmd = m.textInput.Update(msg)
	return m, cmd
}

func (m model) viewReview() string {
	if m.quizIdx >= len(m.quizQuestions) {
		return "Review complete!"
	}

	q := m.quizQuestions[m.quizIdx]
	color := itemColor(q.kind)

	// ── Top: kind label + characters ──
	kindLabel := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#FFFFFF")).
		Background(color).
		Bold(true).
		Padding(0, 2).
		Render(" " + q.kind.String() + " ")

	kindBlock := lipgloss.NewStyle().
		Width(m.width).
		Align(lipgloss.Center).
		Render(kindLabel)

	characters := charDisplayStyle.
		Foreground(color).
		Render(q.characters)

	charBlock := lipgloss.NewStyle().
		Width(m.width).
		Align(lipgloss.Center).
		Render(characters)

	banner := lipgloss.JoinVertical(lipgloss.Center,
		kindBlock, "", "", charBlock, "")

	// ── Progress ──
	progress := lipgloss.NewStyle().
		Width(m.width).
		Align(lipgloss.Center).
		Foreground(lipgloss.Color("#888888")).
		Render(fmt.Sprintf("%d / %d", m.quizIdx+1, len(m.quizQuestions)))

	// ── Prompt ──
	prompt := quizPromptStyle.
		Width(m.width).
		Render(q.promptLabel())

	// ── Result line ──
	resultLine := ""
	if m.quizResult != "" {
		var resultText string
		if m.quizResult == "correct" {
			resultText = correctStyle.Render("Correct!")
		} else {
			resultText = incorrectStyle.Render("Incorrect — ") +
				lipgloss.NewStyle().Foreground(lipgloss.Color("#CCCCCC")).Render(strings.Join(q.answers, ", "))
		}
		resultLine = lipgloss.NewStyle().
			Width(m.width).
			Align(lipgloss.Center).
			Render(resultText)
	}

	// ── Input box ──
	inputBox := inputBoxFocusStyle.
		Width(m.width - 4).
		BorderForeground(color).
		Render(m.textInput.View())

	continueHint := ""
	if m.quizResult != "" {
		continueHint = lipgloss.NewStyle().
			Width(m.width).
			Align(lipgloss.Center).
			Render(hintStyle.Render("Press Enter to continue"))
	}

	// ── Layout ──
	var b strings.Builder
	b.WriteString(banner)
	b.WriteString("\n")
	b.WriteString(progress)
	b.WriteString("\n\n")
	b.WriteString(prompt)
	b.WriteString("\n\n")
	if resultLine != "" {
		b.WriteString(resultLine)
		b.WriteString("\n")
	}
	b.WriteString(inputBox)
	if continueHint != "" {
		b.WriteString("\n")
		b.WriteString(continueHint)
	}

	return b.String()
}

// ── Render helpers ──────────────────────────────────────────

func renderNotAuthenticated() string {
	msg := notAuthStyle.Render("Not authenticated")
	sub := notAuthSubStyle.Render("Use /add-token to connect to WaniKani")
	return lipgloss.JoinVertical(lipgloss.Center, msg, "", sub)
}

func renderDashboard(summary wkSummary) string {
	lessons := fmt.Sprintf("%s  %s",
		lessonCountStyle.Render(fmt.Sprintf("%d Lessons", summary.LessonCount)),
		countLabelStyle.Render("available — /learn"),
	)
	reviews := fmt.Sprintf("%s  %s",
		reviewCountStyle.Render(fmt.Sprintf("%d Reviews", summary.ReviewCount)),
		countLabelStyle.Render("available — /review"),
	)
	return lipgloss.JoinVertical(lipgloss.Center, lessons, "", reviews)
}

func matchingCommands(input string) []command {
	if !strings.HasPrefix(input, "/") {
		return nil
	}
	var matches []command
	for _, c := range commands {
		if strings.HasPrefix(c.name, strings.ToLower(input)) {
			matches = append(matches, c)
		}
	}
	return matches
}

func renderCommandSuggestions(input string, selectedIdx int) string {
	matches := matchingCommands(input)
	if len(matches) == 0 {
		return ""
	}

	var lines []string
	for i, c := range matches {
		var line string
		if i == selectedIdx {
			line = fmt.Sprintf("  %s  %s",
				cmdSelectedStyle.Render(c.name),
				cmdSelectedDescStyle.Render(c.desc),
			)
		} else {
			line = fmt.Sprintf("  %s  %s",
				cmdNameStyle.Render(c.name),
				cmdDescStyle.Render(c.desc),
			)
		}
		lines = append(lines, line)
	}
	return strings.Join(lines, "\n")
}

// ── Command handling ────────────────────────────────────────

func handleCommand(m model, input string) (model, tea.Cmd) {
	cmd := strings.ToLower(strings.TrimSpace(input))

	switch cmd {
	case "/help":
		var helpLines []string
		for _, c := range commands {
			helpLines = append(helpLines, fmt.Sprintf("%s — %s", c.name, c.desc))
		}
		m.statusMsg = strings.Join(helpLines, "  |  ")
	case "/add-token":
		m.mode = modeAwaitingToken
		m.textInput.Placeholder = "Paste your API token here..."
		m.textInput.Prompt = inputPrefixStyle.Render("🔑 ")
		m.textInput.EchoMode = textinput.EchoPassword
		m.statusMsg = "Press Esc to cancel."
	case "/learn":
		if !m.authenticated {
			m.statusMsg = "Not authenticated. Use /add-token first."
		} else {
			m.loadingLessons = true
			m.statusMsg = "Fetching lessons..."
			return m, fetchLessonsCmd(m.token, 5)
		}
	case "/review":
		if !m.authenticated {
			m.statusMsg = "Not authenticated. Use /add-token first."
		} else {
			m.loadingReviews = true
			m.statusMsg = "Fetching reviews..."
			return m, fetchReviewsCmd(m.token, 10)
		}
	case "/refresh":
		if !m.authenticated {
			m.statusMsg = "Not authenticated. Use /add-token first."
		} else {
			m.statusMsg = "Refreshing..."
			return m, fetchSummaryCmd(m.token)
		}
	case "/quit":
		return m, tea.Quit
	default:
		m.statusMsg = fmt.Sprintf("Unknown command: %s (try /help)", cmd)
	}

	return m, nil
}

func handleModeInput(m model, input string) (model, tea.Cmd) {
	var cmd tea.Cmd

	switch m.mode {
	case modeAwaitingToken:
		if err := saveToken(input); err != nil {
			m.statusMsg = fmt.Sprintf("Error saving token: %s", err)
		} else {
			m.token = input
			m.authenticated = true
			m.loadingSummary = true
			m.statusMsg = "Token saved. Fetching summary..."
			cmd = fetchSummaryCmd(input)
		}
	}

	m.mode = modeDefault
	m.textInput.Placeholder = "Type answer or /help"
	m.textInput.Prompt = inputPrefixStyle.Render("❯ ")
	m.textInput.EchoMode = textinput.EchoNormal

	return m, cmd
}

// ── Main ────────────────────────────────────────────────────

func main() {
	p := tea.NewProgram(initialModel(), tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
