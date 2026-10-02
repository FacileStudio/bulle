package agent

import (
	"bufio"
	"cmp"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"sync"

	"github.com/FacileStudio/bulle/internal/settings"
)

func findCommand(name string) (prefix, sub []string, ok bool) {
	args := os.Args[1:]
	first := 0
	for first < len(args) && strings.HasPrefix(args[first], "-") {
		first++
	}
	if first >= len(args) || args[first] != name {
		return nil, nil, false
	}
	return args[:first], args[first+1:], true
}

func checkCronFlag() (bool, error) {
	prefix, sub, ok := findCommand("cron")
	if !ok {
		return false, nil
	}
	return true, dispatchCron(hoistJSON(prefix, sub))
}

func dispatchCron(sub []string) error {
	if len(sub) == 0 || sub[0] == "list" || sub[0] == "status" {
		return listCronJobs()
	}
	if sub[0] == "help" || sub[0] == "-h" || sub[0] == "--help" {
		return printCronUsage()
	}
	if len(sub) < 2 {
		return usagef("usage: bulle cron %s <name>", sub[0])
	}
	switch sub[0] {
	case "run":
		return runCronJob(sub[1])
	case "trust":
		return trustCronJob(sub[1], os.Stdin)
	case "install":
		return installCronJob(sub[1])
	case "uninstall", "remove", "rm":
		return uninstallCronJob(sub[1])
	default:
		return usagef("unknown cron command %q: want run, trust, install, uninstall, or list", sub[0])
	}
}

func hoistJSON(prefix, sub []string) []string {
	rest := make([]string, 0, len(sub))
	json := ""
	for _, arg := range sub {
		if arg == "-json" || arg == "--json" || strings.HasPrefix(arg, "-json=") || strings.HasPrefix(arg, "--json=") {
			json = arg
			continue
		}
		rest = append(rest, arg)
	}
	if json != "" {
		os.Args = append(append([]string{os.Args[0]}, prefix...), append([]string{json, "cron"}, rest...)...)
	}
	return rest
}

var cronConfigOnce = sync.OnceValues(func() (settings.Config, error) {
	ensureUserPath()
	flags := settings.FromFlags(settings.Defaults(""))
	return settings.Settings(DefaultSystemPrompt(), flags)
})

func loadCronConfig() (settings.Config, error) {
	return cronConfigOnce()
}

func loadCronState() (settings.Config, []settings.JobFile, error) {
	config, err := loadCronConfig()
	if err != nil {
		return settings.Config{}, nil, err
	}
	files, err := settings.LoadJobFiles(settings.JobsDir())
	if err != nil {
		return settings.Config{}, nil, err
	}
	return config, files, nil
}

func trustCronJob(name string, in io.Reader) error {
	_, files, err := loadCronState()
	if err != nil {
		return err
	}
	f, err := findJobFile(files, name)
	if err != nil {
		return err
	}
	fmt.Printf("job:     %s\nfile:    %s\nwhen:    %s\ndelivery: %s\n\n%s\n",
		f.Job.Name, f.Path, cmp.Or(f.Job.When, "daily"), cmp.Or(f.Job.Delivery, "none"), string(f.Raw))
	fmt.Print("\ntrust this job to run unattended? [y/N] ")
	line, err := bufio.NewReader(in).ReadString('\n')
	if err != nil && !errors.Is(err, io.EOF) {
		return err
	}
	answer := strings.ToLower(strings.TrimSpace(line))
	if answer != "y" && answer != "yes" {
		return fmt.Errorf("job %q left untrusted", name)
	}
	return settings.Save(f.Path, f.Raw)
}
