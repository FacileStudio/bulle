package sessions

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/FacileStudio/bulle/internal/settings"
	"github.com/FacileStudio/nacelle"
)

// ListSessionFiles returns a slice of session file paths sorted by modification time (newest first)
// for the given project root. If projectRoot is empty, it lists all sessions.
// If the sessions directory doesn't exist, it returns an empty slice.
func ListSessionFiles(projectRoot string) []string {
	base, err := settings.HomeDir()
	if err != nil {
		return nil
	}
	sessionsDir := filepath.Join(base, "sessions")
	files, err := os.ReadDir(sessionsDir)
	if err != nil {
		return nil
	}

	var sessionFiles []string
	for _, f := range files {
		if !f.IsDir() && strings.HasSuffix(f.Name(), ".jsonl") {
			fullPath := filepath.Join(sessionsDir, f.Name())
			if matchProjectRoot(fullPath, projectRoot) {
				sessionFiles = append(sessionFiles, fullPath)
			}
		}
	}
	sortSessionsByMtime(sessionFiles)
	return sessionFiles
}

func peekSessionRoot(path string) string {
	file, err := os.Open(path)
	if err != nil {
		return ""
	}
	defer func() { _ = file.Close() }()
	scanner := bufio.NewScanner(file)
	if scanner.Scan() {
		var header sessionHeader
		if json.Unmarshal([]byte(scanner.Text()), &header) == nil && header.Version == 1 {
			return header.Root
		}
	}
	return ""
}

// matchProjectRoot reports whether a session file belongs to projectRoot.
// Target-labeled roots ("target:<name>", written by remote and sandbox
// sessions with no configured workdir) are compared as identity strings on
// both sides: resolving either would fold them into the host directory bulle
// was launched from, and a target session must match its target, not the
// launch directory.
func matchProjectRoot(path, projectRoot string) bool {
	if projectRoot == "" {
		return true
	}
	root := peekSessionRoot(path)
	if root == "" || root == "." {
		return false
	}
	if settings.IsTargetRoot(root) || settings.IsTargetRoot(projectRoot) {
		return settings.TargetRootName(root) != "" &&
			settings.TargetRootName(root) == settings.TargetRootName(projectRoot)
	}
	targetAbs, err := filepath.Abs(projectRoot)
	if err != nil {
		targetAbs = filepath.Clean(projectRoot)
	}
	headerAbs, err := filepath.Abs(root)
	if err != nil {
		headerAbs = filepath.Clean(root)
	}
	return filepath.Clean(targetAbs) == filepath.Clean(headerAbs)
}

func sortSessionsByMtime(files []string) {
	sort.Slice(files, func(i, j int) bool {
		infoI, errI := os.Stat(files[i])
		infoJ, errJ := os.Stat(files[j])
		if errI != nil || errJ != nil {
			return false
		}
		return infoI.ModTime().After(infoJ.ModTime())
	})
}

// LoadSession loads and parses a session file, returning the conversation.
// It returns nil if the file cannot be read or parsed.
func LoadSession(path string) []nacelle.Message {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}

	lines := strings.Split(strings.TrimSuffix(string(data), "\n"), "\n")
	if len(lines) > 0 && hasSessionHeader(lines[0]) {
		lines = lines[1:]
	}

	var conversation []nacelle.Message
	for _, line := range lines {
		if msg, ok := parseSessionEntry(line); ok {
			conversation = append(conversation, msg)
		}
	}
	return conversation
}

func hasSessionHeader(firstLine string) bool {
	var header sessionHeader
	return json.Unmarshal([]byte(firstLine), &header) == nil && header.Version == 1
}

func parseSessionEntry(line string) (nacelle.Message, bool) {
	if line == "" {
		return nacelle.Message{}, false
	}
	var entry sessionEntry
	if err := json.Unmarshal([]byte(line), &entry); err != nil {
		return nacelle.Message{}, false
	}
	switch entry.Who {
	case "question":
		return nacelle.UserText(entry.Text), true
	case "answer":
		return nacelle.AssistantText(entry.Text), true
	default:
		return nacelle.Message{}, false
	}
}
