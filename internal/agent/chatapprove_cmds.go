package agent

var courrierSubs = map[string]bool{
	"inbox": true, "list": true, "read": true, "search": true, "folders": true,
	"accounts": true, "triage": true, "contacts": true,
}

var listingSubs = map[string]bool{"folders": true, "accounts": true, "contacts": true}

var listingActions = map[string]bool{"list": true, "show": true, "search": true}

var courrierFlags = flagSet{
	"--json": false, "--account": true, "--limit": true, "--page": true,
	"--unread": false, "--no-color": false, "--since": true, "--class": true,
	"--all": false,
}

var agendaFlags = flagSet{
	"--json": false, "--calendar": true, "--from": true, "--to": true,
	"--search": true,
}

var opusFlags = flagSet{
	"--json": false, "-q": false, "--quiet": false, "--no-color": false,
	"--project": true, "--label": true, "--priority": true, "--status": true,
	"--overdue": false, "--done": false, "--limit": true,
}

var myceliumFlags = flagSet{"--limit": true, "--local": false, "--verbose": false, "--no-color": false}

func courrierOK(a []string) bool {
	if len(a) < 2 || !courrierSubs[a[1]] {
		return false
	}
	rest, ok := positionals(a[2:], courrierFlags)
	if !ok {
		return false
	}
	return !listingSubs[a[1]] || len(rest) == 0 || listingActions[rest[0]]
}

func agendaOK(a []string) bool {
	if len(a) < 3 || (a[1] != "calendars" && a[1] != "events") || a[2] != "list" {
		return false
	}
	_, ok := positionals(a[3:], agendaFlags)
	return ok
}

func myceliumOK(a []string) bool {
	if len(a) < 3 || a[1] != "memory" || a[2] != "search" {
		return false
	}
	_, ok := positionals(a[3:], myceliumFlags)
	return ok
}

func opusOK(a []string) bool {
	if len(a) >= 3 && a[1] == "workspace" && a[2] == "list" {
		_, ok := positionals(a[3:], opusFlags)
		return ok
	}
	if len(a) < 5 || a[1] != "-w" || !workspaceID.MatchString(a[2]) || a[3] != "task" {
		return false
	}
	if a[4] != "list" && a[4] != "show" {
		return false
	}
	_, ok := positionals(a[5:], opusFlags)
	return ok
}
