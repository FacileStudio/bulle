package tui

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/FacileStudio/bulle/internal/settings"
)

type editorFinishedMsg struct {
	path    string
	cleanup func()
	err     error
}

func resolveEditor(cfg settings.Config) string {
	if cfg.Editor.Editor != "" {
		return cfg.Editor.Editor
	}
	if e := os.Getenv("GIT_EDITOR"); e != "" {
		return e
	}
	if e := os.Getenv("EDITOR"); e != "" {
		return e
	}
	if e := os.Getenv("VISUAL"); e != "" {
		return e
	}
	return "vi"
}

func editInExternalEditor(content string, editor string) (string, error) {
	if editor == "" {
		return content, errors.New("no editor configured")
	}

	tmpPath, closeFn, err := createTempFile(content, "bulle-*.md")
	if err != nil {
		return content, err
	}
	defer closeFn()

	parts := strings.Fields(editor)
	var cmd *exec.Cmd
	if len(parts) > 1 {
		cmd = exec.Command(parts[0], append(parts[1:], tmpPath)...)
	} else {
		cmd = exec.Command(editor, tmpPath)
	}
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		if exitErr, ok := errors.AsType[*exec.ExitError](err); ok {
			return content, fmt.Errorf("editor exited with code %d", exitErr.ExitCode())
		}
		return content, fmt.Errorf("running editor: %w", err)
	}

	edited, err := os.ReadFile(tmpPath)
	if err != nil {
		return content, fmt.Errorf("reading edited file: %w", err)
	}
	return string(edited), nil
}

func createTempFile(content string, suffix string) (string, func(), error) {
	tmpFile, err := os.CreateTemp("", suffix)
	if err != nil {
		return "", func() {}, fmt.Errorf("creating temp file: %w", err)
	}
	path := tmpFile.Name()

	if _, err := tmpFile.WriteString(content); err != nil {
		if closeErr := tmpFile.Close(); closeErr != nil {
			err = errors.Join(err, closeErr)
		}
		cleanupTemp(path)
		return "", func() {}, fmt.Errorf("writing temp file: %w", err)
	}

	if err := tmpFile.Close(); err != nil {
		cleanupTemp(path)
		return "", func() {}, fmt.Errorf("closing temp file: %w", err)
	}

	return path, func() { cleanupTemp(path) }, nil
}

func cleanupTemp(path string) {
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return
	}
}

func (m *Model) openEditor() tea.Cmd {
	editor := resolveEditor(settings.Config{Editor: settings.Editor{Editor: m.run.editorPath, PromptEditKey: m.run.promptEditKey}})
	if editor == "" {
		return nil
	}
	content := m.prompt.Value()

	tmpPath, cleanup, err := createTempFile(content, "bulle-prompt-*.md")
	if err != nil {
		m.say(fromReader, "editor failed: "+err.Error())
		return nil
	}

	parts := strings.Fields(editor)
	var cmd *exec.Cmd
	if len(parts) > 1 {
		cmd = exec.Command(parts[0], append(parts[1:], tmpPath)...)
	} else {
		cmd = exec.Command(editor, tmpPath)
	}

	return tea.ExecProcess(cmd, func(err error) tea.Msg {
		return editorFinishedMsg{
			path:    tmpPath,
			cleanup: cleanup,
			err:     err,
		}
	})
}

func (m *Model) finishEditor(msg editorFinishedMsg) tea.Cmd {
	defer msg.cleanup()
	if msg.err != nil {
		if exitErr, ok := errors.AsType[*exec.ExitError](msg.err); ok {
			m.say(fromReader, fmt.Sprintf("editor exited with code %d", exitErr.ExitCode()))
		} else {
			m.say(fromReader, "editor failed: "+msg.err.Error())
		}
		return nil
	}

	editedBytes, err := os.ReadFile(msg.path)
	if err != nil {
		m.say(fromReader, "reading edited file: "+err.Error())
		return nil
	}

	edited := string(editedBytes)
	edited = strings.TrimSuffix(edited, "\r\n")
	edited = strings.TrimSuffix(edited, "\n")
	m.prompt.SetValue(edited)
	m.prompt.CursorEnd()
	m.refreshMenu()
	return nil
}
