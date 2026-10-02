package agent

import (
	"github.com/FacileStudio/bulle/internal/approval"
	"github.com/FacileStudio/bulle/internal/settings"
	"github.com/FacileStudio/bulle/internal/skills"
)

// Config aliases settings.Config.
type Config = settings.Config

// Session aliases settings.Session, the four top-level launch settings.
type Session = settings.Session

// Provider aliases settings.Provider, the active backend plus its endpoint.
type Provider = settings.Provider

// Skill aliases skills.Skill.
type Skill = skills.Skill
type skill = skills.Skill

// Approvals aliases approval.Approvals.
type Approvals = approval.Approvals

// Sources aliases settings.Sources.
type Sources = settings.Sources

// Security aliases settings.Security.
type Security = settings.Security

// Toggles aliases settings.Toggles.
type Toggles = settings.Toggles
