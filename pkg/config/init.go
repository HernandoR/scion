package config

import (
	"fmt"
	"os"
	"path/filepath"
)

const DefaultSettingsJSON = `{
  "yolo": true,
  "security": {
    "auth": {
      "selectedType": "gemini-api-key"
    }
  },
	"telemetry": {
    "enabled": false
  },
	"general": {
    "disableAutoUpdate": true,
    "disableUpdateNag": true,
    "previewFeatures": true
  },
	"ui": {
    "accessibility": {
      "disableLoadingPhrases": true
    },
    "hideFooter": true,
    "hideWindowTitle": true
  },
  "hooks": {
    "SessionStart": [{"matcher": "*", "hooks": [{"name": "scion-status", "type": "command", "command": "python3 /home/node/scion_hook.py"}]}],
    "SessionEnd": [{"matcher": "*", "hooks": [{"name": "scion-status", "type": "command", "command": "python3 /home/node/scion_hook.py"}]}],
    "BeforeAgent": [{"matcher": "*", "hooks": [{"name": "scion-status", "type": "command", "command": "python3 /home/node/scion_hook.py"}]}],
    "AfterAgent": [{"matcher": "*", "hooks": [{"name": "scion-status", "type": "command", "command": "python3 /home/node/scion_hook.py"}]}],
    "BeforeTool": [{"matcher": "*", "hooks": [{"name": "scion-status", "type": "command", "command": "python3 /home/node/scion_hook.py"}]}],
    "AfterTool": [{"matcher": "*", "hooks": [{"name": "scion-status", "type": "command", "command": "python3 /home/node/scion_hook.py"}]}],
    "Notification": [{"matcher": "ToolPermission", "hooks": [{"name": "scion-status", "type": "command", "command": "python3 /home/node/scion_hook.py"}]}]
  }
}
`

const DefaultScionHookPy = `import json
import sys
import os
import tempfile
from datetime import datetime

SCION_JSON_PATH = "/home/node/scion.json"
AGENT_LOG_PATH = "/home/node/agent.log"

def log_event(state, message):
    timestamp = datetime.now().strftime("%Y-%m-%d %H:%M:%S")
    with open(AGENT_LOG_PATH, "a") as f:
        f.write(f"{timestamp} [{state}] {message}\n")

def update_status(status):
    if not os.path.exists(SCION_JSON_PATH):
        return
    try:
        with open(SCION_JSON_PATH, "r") as f:
            data = json.load(f)
        
        if "agent" not in data:
            data["agent"] = {}
        data["agent"]["status"] = status
        
        # Atomic write
        fd, temp_path = tempfile.mkstemp(dir=os.path.dirname(SCION_JSON_PATH))
        with os.fdopen(fd, 'w') as f:
            json.dump(data, f, indent=2)
        os.replace(temp_path, SCION_JSON_PATH)
    except Exception as e:
        log_event("ERROR", f"Failed to update scion.json: {e}")

def main():
    try:
        input_data = json.load(sys.stdin)
    except Exception:
        # Non-JSON input, skip
        return

    event = input_data.get("hook_event_name")
    
    state = "IDLE"
    log_msg = f"Event: {event}"

    if event == "SessionStart":
        state = "STARTING"
        log_msg = f"Session started (source: {input_data.get('source')})"
    elif event == "BeforeAgent":
        state = "THINKING"
        prompt = input_data.get("prompt", "")
        log_msg = f"User prompt: {prompt[:100]}..." if prompt else "Planning turn"
    elif event == "BeforeTool":
        tool_name = input_data.get("tool_name")
        state = f"EXECUTING ({tool_name})"
        log_msg = f"Running tool: {tool_name}"
    elif event == "AfterTool":
        state = "THINKING"
        tool_name = input_data.get("tool_name")
        log_msg = f"Tool {tool_name} completed"
    elif event == "Notification":
        state = "WAITING"
        log_msg = f"Notification: {input_data.get('message')}"
    elif event == "AfterAgent":
        state = "IDLE"
        log_msg = "Agent turn completed"
    elif event == "SessionEnd":
        state = "EXITED"
        log_msg = f"Session ended (reason: {input_data.get('reason')})"

    update_status(state)
    log_event(state, log_msg)

if __name__ == "__main__":
    main()
`

const DefaultSystemPrompt = `# Scion Agent
You are a specialized agent working within a Scion.
`

const DefaultScionJSON = `{
  "image": "gemini-cli-sandbox",
  "use_tmux": true,
  "model": "flash"
}
`

const DefaultGeminiMD = `## Scion Context
`

const DefaultBashrc = `# scion agent bashrc
alias g="gemini"
`

func InitProject(targetDir string) error {
	var projectDir string
	var err error

	if targetDir != "" {
		projectDir = targetDir
	} else {
		projectDir, err = GetTargetProjectDir()
		if err != nil {
			return err
		}
	}

	templatesDir := filepath.Join(projectDir, "templates")
	defaultTemplateDir := filepath.Join(templatesDir, "default")
	agentsDir := filepath.Join(projectDir, "agents")

	// Create directories
	dirs := []string{
		projectDir,
		templatesDir,
		defaultTemplateDir,
		filepath.Join(defaultTemplateDir, ".gemini"),
		filepath.Join(defaultTemplateDir, ".config", "gcloud"),
		agentsDir,
	}

	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("failed to create directory %s: %w", dir, err)
		}
	}

	// Seed default template files
	files := []struct {
		path    string
		content string
	}{
		{filepath.Join(defaultTemplateDir, "scion.json"), DefaultScionJSON},
		{filepath.Join(defaultTemplateDir, "scion_hook.py"), DefaultScionHookPy},
		{filepath.Join(defaultTemplateDir, ".gemini", "settings.json"), DefaultSettingsJSON},
		{filepath.Join(defaultTemplateDir, ".gemini", "system_prompt.md"), DefaultSystemPrompt},
		{filepath.Join(defaultTemplateDir, "gemini.md"), DefaultGeminiMD},
		{filepath.Join(defaultTemplateDir, ".bashrc"), DefaultBashrc},
	}

	for _, f := range files {
		// Always write settings.json to ensure it matches current defaults
		if filepath.Base(f.path) == "settings.json" {
			if err := os.WriteFile(f.path, []byte(f.content), 0644); err != nil {
				return fmt.Errorf("failed to write file %s: %w", f.path, err)
			}
			continue
		}

		if _, err := os.Stat(f.path); os.IsNotExist(err) {
			if err := os.WriteFile(f.path, []byte(f.content), 0644); err != nil {
				return fmt.Errorf("failed to write file %s: %w", f.path, err)
			}
		}
	}

	return nil
}

func InitGlobal() error {
	globalDir, err := GetGlobalDir()
	if err != nil {
		return err
	}

	templatesDir := filepath.Join(globalDir, "templates")
	defaultTemplateDir := filepath.Join(templatesDir, "default")
	agentsDir := filepath.Join(globalDir, "agents")

	// Create directories
	dirs := []string{
		globalDir,
		templatesDir,
		defaultTemplateDir,
		filepath.Join(defaultTemplateDir, ".gemini"),
		agentsDir,
	}

	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("failed to create global directory %s: %w", dir, err)
		}
	}

	// Seed default template files for global as well
	files := []struct {
		path    string
		content string
	}{
		{filepath.Join(defaultTemplateDir, "scion.json"), DefaultScionJSON},
		{filepath.Join(defaultTemplateDir, "scion_hook.py"), DefaultScionHookPy},
		{filepath.Join(defaultTemplateDir, ".gemini", "settings.json"), DefaultSettingsJSON},
		{filepath.Join(defaultTemplateDir, ".gemini", "system_prompt.md"), DefaultSystemPrompt},
		{filepath.Join(defaultTemplateDir, ".gemini", "gemini.md"), DefaultGeminiMD},
		{filepath.Join(defaultTemplateDir, ".bashrc"), DefaultBashrc},
	}

	for _, f := range files {
		// Always write settings.json to ensure it matches current defaults
		if filepath.Base(f.path) == "settings.json" {
			if err := os.WriteFile(f.path, []byte(f.content), 0644); err != nil {
				return fmt.Errorf("failed to write global file %s: %w", f.path, err)
			}
			continue
		}

		if _, err := os.Stat(f.path); os.IsNotExist(err) {
			if err := os.WriteFile(f.path, []byte(f.content), 0644); err != nil {
				return fmt.Errorf("failed to write global file %s: %w", f.path, err)
			}
		}
	}

	return nil
}
