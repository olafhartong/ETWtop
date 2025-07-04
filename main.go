package main

import (
	"encoding/csv"
	"fmt"
	"log"
	"os"
	"sort"
	"strconv"
	"strings"
	"syscall"
	"time"
	"unicode/utf16"
	"unsafe"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

const (
	ERROR_SUCCESS          = 0
	ERROR_MORE_DATA        = 234
	MAX_SESSION_NAME_LEN   = 1024
	WNODE_FLAG_TRACED_GUID = 0x00020000
)

// Windows API structures
type WNODE_HEADER struct {
	BufferSize        uint32
	ProviderId        uint32
	HistoricalContext uint64
	TimeStamp         int64
	Guid              [16]byte
	ClientContext     uint32
	Flags             uint32
}

type EVENT_TRACE_PROPERTIES struct {
	Wnode               WNODE_HEADER
	BufferSize          uint32
	MinimumBuffers      uint32
	MaximumBuffers      uint32
	MaximumFileSize     uint32
	LogFileMode         uint32
	FlushTimer          uint32
	EnableFlags         uint32
	AgeLimit            int32
	NumberOfBuffers     uint32
	FreeBuffers         uint32
	EventsLost          uint32
	BuffersWritten      uint32
	LogBuffersLost      uint32
	RealTimeBuffersLost uint32
	LoggerThreadId      uintptr
	LogFileNameOffset   uint32
	LoggerNameOffset    uint32
}

// ETW Session information
type ETWSession struct {
	Name                string
	BufferSize          uint32
	MinimumBuffers      uint32
	MaximumBuffers      uint32
	NumberOfBuffers     uint32
	FreeBuffers         uint32
	BuffersWritten      uint32
	EventsLost          uint32
	RealTimeBuffersLost uint32
	LogFileMode         uint32
	LogFileName         string
	Timestamp           time.Time
}

// Calculated properties
func (s *ETWSession) UtilizationPercent() float64 {
	if s.NumberOfBuffers == 0 {
		return 0.0
	}
	return float64(s.NumberOfBuffers-s.FreeBuffers) / float64(s.NumberOfBuffers) * 100.0
}

func (s *ETWSession) TotalMemoryMB() float64 {
	return float64(s.NumberOfBuffers*s.BufferSize) / 1024.0
}

// Windows API declarations
var (
	advapi32            = syscall.NewLazyDLL("advapi32.dll")
	procQueryAllTracesW = advapi32.NewProc("QueryAllTracesW")
	// procQueryTraceW     = advapi32.NewProc("QueryTraceW")
	// procControlTraceW   = advapi32.NewProc("ControlTraceW")
)

// Helper function to convert UTF16 pointer to Go string
func utf16PtrToString(ptr *uint16) string {
	if ptr == nil {
		return ""
	}

	// Find the length of the string
	length := 0
	for {
		if *(*uint16)(unsafe.Pointer(uintptr(unsafe.Pointer(ptr)) + uintptr(length*2))) == 0 {
			break
		}
		length++
	}

	if length == 0 {
		return ""
	}

	// Create a slice of uint16 from the pointer
	utf16Slice := make([]uint16, length)
	for i := 0; i < length; i++ {
		utf16Slice[i] = *(*uint16)(unsafe.Pointer(uintptr(unsafe.Pointer(ptr)) + uintptr(i*2)))
	}

	// Convert to UTF8 string
	return string(utf16.Decode(utf16Slice))
}

// ETW Buffer Monitor
type ETWBufferMonitor struct {
	monitoring bool
	sessions   []ETWSession
}

func NewETWBufferMonitor() *ETWBufferMonitor {
	return &ETWBufferMonitor{
		monitoring: false,
		sessions:   make([]ETWSession, 0),
	}
}

// Bubble Tea Model for TUI
type model struct {
	monitor          *ETWBufferMonitor
	sessions         []ETWSession
	previousSessions map[string]ETWSession // Track previous state for change detection
	lastUpdate       time.Time
	intervalSeconds  int
	showOnce         bool
	err              error
	exiting          bool
}

// Message types for Bubble Tea
type tickMsg time.Time
type sessionsMsg []ETWSession
type errMsg error

func initialModel(intervalSeconds int, showOnce bool) model {
	return model{
		monitor:          NewETWBufferMonitor(),
		sessions:         []ETWSession{},
		previousSessions: make(map[string]ETWSession),
		intervalSeconds:  intervalSeconds,
		showOnce:         showOnce,
		lastUpdate:       time.Now(),
	}
}

func (m model) Init() tea.Cmd {
	return tea.Batch(
		tea.Tick(time.Duration(m.intervalSeconds)*time.Second, func(t time.Time) tea.Msg {
			return tickMsg(t)
		}),
		m.querySessionsCmd(),
	)
}

func (m model) querySessionsCmd() tea.Cmd {
	return func() tea.Msg {
		sessions, err := m.monitor.QueryAllSessions()
		if err != nil {
			return errMsg(err)
		}
		return sessionsMsg(sessions)
	}
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			m.exiting = true
			return m, tea.Quit
		}

	case tickMsg:
		if m.showOnce {
			return m, nil
		}
		return m, tea.Batch(
			tea.Tick(time.Duration(m.intervalSeconds)*time.Second, func(t time.Time) tea.Msg {
				return tickMsg(t)
			}),
			m.querySessionsCmd(),
		)
	case sessionsMsg:
		// Store previous sessions for change detection
		for _, session := range m.sessions {
			m.previousSessions[session.Name] = session
		}
		m.sessions = []ETWSession(msg)
		m.lastUpdate = time.Now()
		if m.showOnce {
			return m, tea.Quit
		}

	case errMsg:
		m.err = msg
	}

	return m, nil
}

func (m model) View() string {
	var b strings.Builder

	// Enhanced Styles
	headerStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("86")).
		MarginBottom(1)

	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("39"))

	warningStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("196"))

	tableHeaderStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("33"))

	summaryBoxStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("240")).
		Padding(0, 1).
		MarginTop(1).
		Width(58)

	summaryLabelStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("39"))

	summaryValueStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("252"))

	warningBoxStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("196")).
		Padding(0, 1).
		MarginTop(1).
		Width(58)

	if m.exiting {
		return "Shutting down monitor...\n"
	}

	if m.err != nil {
		return fmt.Sprintf("Error: %v\nPress q to quit.", m.err)
	}

	// Header
	b.WriteString(headerStyle.Render("ETW Buffer Monitor v1.0 (Go)"))
	b.WriteString("\n")
	b.WriteString(titleStyle.Render(fmt.Sprintf("%d active sessions", len(m.sessions))))
	b.WriteString("\n")
	b.WriteString(fmt.Sprintf("Timestamp: %s", m.lastUpdate.Format("2006-01-02 15:04:05")))
	if !m.showOnce {
		b.WriteString(fmt.Sprintf(" | Refresh: %ds | Press 'q' to quit", m.intervalSeconds))
	}
	b.WriteString("\n")
	b.WriteString(strings.Repeat("═", 120))
	b.WriteString("\n\n")

	if len(m.sessions) == 0 {
		b.WriteString("No active ETW sessions found.\n")
		b.WriteString("This may be normal if no ETW tracing is currently active.\n")
		return b.String()
	}

	// Table header
	b.WriteString(tableHeaderStyle.Render(fmt.Sprintf("%-30s %-12s %-8s %-8s %-8s %-6s %-10s %-10s %-8s %-12s",
		"Session Name", "Buffer(KB)", "Min", "Max", "Current", "Free", "Written", "Lost", "Util%", "Memory(MB)")))
	b.WriteString("\n")
	b.WriteString(strings.Repeat("─", 120))
	b.WriteString("\n")

	// Session data
	var totalMemory float64
	var totalUtilization float64
	var totalEventsLost uint32

	for _, session := range m.sessions {
		sessionName := session.Name
		if len(sessionName) > 29 {
			sessionName = sessionName[:29]
		}

		utilization := session.UtilizationPercent()
		memory := session.TotalMemoryMB()

		// Check for changes from previous update
		var rowStyle lipgloss.Style
		previousSession, existed := m.previousSessions[session.Name]

		hasChanges := existed && (previousSession.NumberOfBuffers != session.NumberOfBuffers ||
			previousSession.FreeBuffers != session.FreeBuffers ||
			previousSession.EventsLost != session.EventsLost ||
			previousSession.BuffersWritten != session.BuffersWritten)

		// Color code based on state and changes
		if session.EventsLost > 0 {
			rowStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("196")) // Red for lost events
		} else if utilization > 80 {
			rowStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("208")) // Orange for high utilization
		} else if hasChanges && !m.showOnce {
			rowStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("120")) // Subtle green for changes
		} else {
			rowStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("252")) // Normal
		}

		line := fmt.Sprintf("%-30s %-12d %-8d %-8d %-8d %-6d %-10d %-10d %-8.1f %-12.1f",
			sessionName,
			session.BufferSize,
			session.MinimumBuffers,
			session.MaximumBuffers,
			session.NumberOfBuffers,
			session.FreeBuffers,
			session.BuffersWritten,
			session.EventsLost,
			utilization,
			memory)

		b.WriteString(rowStyle.Render(line))
		b.WriteString("\n")

		totalMemory += memory
		totalUtilization += utilization
		totalEventsLost += session.EventsLost
	}
	// Clean Summary Section
	b.WriteString("\n")

	var summaryContent strings.Builder
	summaryContent.WriteString(summaryLabelStyle.Render("Summary") + "\n")
	summaryContent.WriteString(fmt.Sprintf("%-20s %s\n",
		summaryValueStyle.Render("Total Sessions:"),
		summaryLabelStyle.Render(fmt.Sprintf("%d", len(m.sessions)))))
	summaryContent.WriteString(fmt.Sprintf("%-20s %s\n",
		summaryValueStyle.Render("Total Memory:"),
		summaryLabelStyle.Render(fmt.Sprintf("%.1f MB", totalMemory))))
	if len(m.sessions) > 0 {
		summaryContent.WriteString(fmt.Sprintf("%-20s %s\n",
			summaryValueStyle.Render("Avg Utilization:"),
			summaryLabelStyle.Render(fmt.Sprintf("%.1f%%", totalUtilization/float64(len(m.sessions))))))
	}
	summaryContent.WriteString(fmt.Sprintf("%-20s %s",
		summaryValueStyle.Render("Total Events Lost:"),
		summaryLabelStyle.Render(fmt.Sprintf("%d", totalEventsLost))))

	summaryBox := summaryBoxStyle.Render(summaryContent.String())

	// Check for warnings and create warning box
	highUtilSessions := 0
	lostEventSessions := 0
	for _, session := range m.sessions {
		if session.UtilizationPercent() > 80 {
			highUtilSessions++
		}
		if session.EventsLost > 0 {
			lostEventSessions++
		}
	}

	var warningBox string
	if highUtilSessions > 0 || lostEventSessions > 0 {
		var warningContent strings.Builder
		warningContent.WriteString(warningStyle.Render("⚠ Warnings") + "\n")
		if highUtilSessions > 0 {
			warningContent.WriteString(fmt.Sprintf("• %d session(s) have high buffer utilization (>80%%)\n", highUtilSessions))
			warningContent.WriteString("  Consider increasing buffer count\n")
		}
		if lostEventSessions > 0 {
			if highUtilSessions > 0 {
				warningContent.WriteString("\n")
			}
			warningContent.WriteString(fmt.Sprintf("• %d session(s) have lost events\n", lostEventSessions))
			warningContent.WriteString("  Increase buffer size or count")
		}
		warningBox = warningBoxStyle.Render(warningContent.String())
	}

	// Place summary and warning boxes side by side
	if warningBox != "" {
		bottomSection := lipgloss.JoinHorizontal(lipgloss.Top, summaryBox, "  ", warningBox)
		b.WriteString(bottomSection)
	} else {
		b.WriteString(summaryBox)
	}

	return b.String()
}

// Query all active ETW sessions
func (m *ETWBufferMonitor) QueryAllSessions() ([]ETWSession, error) {
	var sessionCount uint32

	// First call to get the number of sessions
	ret, _, _ := procQueryAllTracesW.Call(
		0, // NULL pointer for first call
		0, // 0 count for first call
		uintptr(unsafe.Pointer(&sessionCount)),
	)

	if ret != ERROR_MORE_DATA {
		return nil, fmt.Errorf("failed to get session count, error: %d", ret)
	}

	if sessionCount == 0 {
		return []ETWSession{}, nil
	}

	// Allocate memory for session properties array
	const propertySize = unsafe.Sizeof(EVENT_TRACE_PROPERTIES{}) + MAX_SESSION_NAME_LEN*2 // Unicode strings
	buffer := make([]byte, int(sessionCount)*int(propertySize))
	sessionArray := make([]uintptr, sessionCount)

	for i := uint32(0); i < sessionCount; i++ {
		// Get a pointer to the current session's properties within the buffer
		props := (*EVENT_TRACE_PROPERTIES)(unsafe.Pointer(&buffer[i*uint32(propertySize)]))

		// Initialize the structure
		props.Wnode.BufferSize = uint32(propertySize)
		props.LoggerNameOffset = uint32(unsafe.Sizeof(EVENT_TRACE_PROPERTIES{}))
		props.LogFileNameOffset = props.LoggerNameOffset + MAX_SESSION_NAME_LEN

		sessionArray[i] = uintptr(unsafe.Pointer(props))
	}

	// Second call to get actual session data
	ret, _, _ = procQueryAllTracesW.Call(
		uintptr(unsafe.Pointer(&sessionArray[0])),
		uintptr(sessionCount),
		uintptr(unsafe.Pointer(&sessionCount)),
	)

	var sessions []ETWSession

	if ret == ERROR_SUCCESS {
		for i := uint32(0); i < sessionCount; i++ {
			props := (*EVENT_TRACE_PROPERTIES)(unsafe.Pointer(sessionArray[i]))

			// Extract session name
			namePtr := uintptr(unsafe.Pointer(props)) + uintptr(props.LoggerNameOffset)
			sessionName := utf16PtrToString((*uint16)(unsafe.Pointer(namePtr)))

			// Extract log file name if present
			var logFileName string
			if props.LogFileNameOffset > 0 {
				logFilePtr := uintptr(unsafe.Pointer(props)) + uintptr(props.LogFileNameOffset)
				logFileName = utf16PtrToString((*uint16)(unsafe.Pointer(logFilePtr)))
			}

			session := ETWSession{
				Name:                sessionName,
				BufferSize:          props.BufferSize,
				MinimumBuffers:      props.MinimumBuffers,
				MaximumBuffers:      props.MaximumBuffers,
				NumberOfBuffers:     props.NumberOfBuffers,
				FreeBuffers:         props.FreeBuffers,
				BuffersWritten:      props.BuffersWritten,
				EventsLost:          props.EventsLost,
				RealTimeBuffersLost: props.RealTimeBuffersLost,
				LogFileMode:         props.LogFileMode,
				LogFileName:         logFileName,
				Timestamp:           time.Now(),
			}

			sessions = append(sessions, session)
		}
	} else {
		return nil, fmt.Errorf("failed to query sessions, error: %d", ret)
	}

	// Sort sessions by name for consistent output
	sort.Slice(sessions, func(i, j int) bool {
		return sessions[i].Name < sessions[j].Name
	})
	m.sessions = sessions
	return sessions, nil
}

// Export sessions to CSV
func (m *ETWBufferMonitor) ExportToCSV(sessions []ETWSession, filename string) error {
	file, err := os.Create(filename)
	if err != nil {
		return fmt.Errorf("failed to create CSV file: %w", err)
	}
	defer file.Close()

	writer := csv.NewWriter(file)
	defer writer.Flush()

	// CSV Header
	header := []string{
		"Timestamp", "SessionName", "BufferSize_KB", "MinBuffers", "MaxBuffers",
		"NumberOfBuffers", "FreeBuffers", "BuffersWritten", "EventsLost",
		"RealTimeBuffersLost", "UtilizationPercent", "TotalMemory_MB", "LogFileName",
	}

	if err := writer.Write(header); err != nil {
		return fmt.Errorf("failed to write CSV header: %w", err)
	}

	// Data rows
	for _, session := range sessions {
		record := []string{
			session.Timestamp.Format("2006-01-02 15:04:05"),
			session.Name,
			strconv.FormatUint(uint64(session.BufferSize), 10),
			strconv.FormatUint(uint64(session.MinimumBuffers), 10),
			strconv.FormatUint(uint64(session.MaximumBuffers), 10),
			strconv.FormatUint(uint64(session.NumberOfBuffers), 10),
			strconv.FormatUint(uint64(session.FreeBuffers), 10),
			strconv.FormatUint(uint64(session.BuffersWritten), 10),
			strconv.FormatUint(uint64(session.EventsLost), 10),
			strconv.FormatUint(uint64(session.RealTimeBuffersLost), 10),
			fmt.Sprintf("%.2f", session.UtilizationPercent()),
			fmt.Sprintf("%.2f", session.TotalMemoryMB()),
			session.LogFileName,
		}

		if err := writer.Write(record); err != nil {
			return fmt.Errorf("failed to write CSV record: %w", err)
		}
	}

	fmt.Printf("Buffer statistics exported to: %s\n", filename)
	return nil
}

// Start continuous monitoring with Bubble Tea
func (m *ETWBufferMonitor) StartMonitoring(intervalSeconds int) {
	// Initialize the Bubble Tea model
	p := tea.NewProgram(initialModel(intervalSeconds, false))

	// Run the program
	if _, err := p.Run(); err != nil {
		log.Fatalf("Error running monitor: %v", err)
	}
}

// Start one-time display with Bubble Tea
func (m *ETWBufferMonitor) ShowOnce() {
	// Initialize the Bubble Tea model for one-time display
	p := tea.NewProgram(initialModel(1, true))

	// Run the program
	if _, err := p.Run(); err != nil {
		log.Fatalf("Error running monitor: %v", err)
	}
}

// Stop monitoring
func (m *ETWBufferMonitor) StopMonitoring() {
	m.monitoring = false
}

// Show help information
func showHelp() {
	fmt.Println("ETW Buffer Monitor v1.0 (Go)")
	fmt.Println("=============================")
	fmt.Println()
	fmt.Println("Usage: ETWBufferMonitor.exe [options]")
	fmt.Println()
	fmt.Println("Options:")
	fmt.Println("  -once              Show buffer info once and exit")
	fmt.Println("  -export [filename] Export to CSV file (default: etw_buffer_stats.csv)")
	fmt.Println("  -interval [seconds] Monitoring interval in seconds (default: 1)")
	fmt.Println("  -help              Show this help message")
	fmt.Println("  (no options)       Start continuous monitoring")
	fmt.Println()
	fmt.Println("Examples:")
	fmt.Println("  ETWBufferMonitor.exe                    # Start continuous monitoring")
	fmt.Println("  ETWBufferMonitor.exe -once              # Show current stats once")
	fmt.Println("  ETWBufferMonitor.exe -export stats.csv  # Export to CSV")
	fmt.Println("  ETWBufferMonitor.exe -interval 10       # Monitor with 10-second intervals")
	fmt.Println()
	fmt.Println("Note: This tool requires administrator privileges to access ETW sessions.")
}

// Check if running as administrator
func checkAdminPrivileges() bool {
	// Try to query sessions as a basic check
	monitor := NewETWBufferMonitor()
	_, err := monitor.QueryAllSessions()
	return err == nil
}

func main() {
	// Check for administrator privileges
	if !checkAdminPrivileges() {
		fmt.Println("Warning: This tool requires administrator privileges to access ETW sessions.")
		fmt.Println("Please run as Administrator for full functionality.")
		fmt.Println()
	}

	monitor := NewETWBufferMonitor()

	// Parse command line arguments
	if len(os.Args) > 1 {
		switch strings.ToLower(os.Args[1]) {
		case "-help", "--help", "-h":
			showHelp()
			return
		case "-once", "--once", "-o":
			monitor.ShowOnce()
			return

		case "-export", "--export", "-e":
			filename := "etw_buffer_stats.csv"
			if len(os.Args) > 2 {
				filename = os.Args[2]
			}

			fmt.Println("ETW Buffer Monitor - Exporting to CSV")
			fmt.Println("=====================================")
			sessions, err := monitor.QueryAllSessions()
			if err != nil {
				log.Fatalf("Error querying sessions: %v", err)
			}

			if err := monitor.ExportToCSV(sessions, filename); err != nil {
				log.Fatalf("Error exporting to CSV: %v", err)
			}
			return

		case "-interval", "--interval", "-i":
			intervalSeconds := 1
			if len(os.Args) > 2 {
				if interval, err := strconv.Atoi(os.Args[2]); err == nil && interval > 0 {
					intervalSeconds = interval
				} else {
					fmt.Printf("Invalid interval '%s', using default: %d seconds\n", os.Args[2], intervalSeconds)
				}
			}
			monitor.StartMonitoring(intervalSeconds)
			return

		default:
			fmt.Printf("Unknown option: %s\n", os.Args[1])
			showHelp()
			return
		}
	}

	// Default: start continuous monitoring
	monitor.StartMonitoring(1)
}
