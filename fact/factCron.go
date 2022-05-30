package fact

import (
	"fmt"

	"github.com/robfig/cron"

	"ChatWire/cfg"
	"ChatWire/cwlog"
)

var CronVar *cron.Cron

func SetupSchedule() {
	if cfg.Local.Options.Schedule != "" {
		CronVar = cron.New()

		err := CronVar.AddFunc(cfg.Local.Options.Schedule, doMapReset)
		if err != nil {
			cwlog.DoLogCW("Error setting up schedule: " + err.Error())
		} else {
			cwlog.DoLogCW("Schedule set up: " + cfg.Local.Options.Schedule)
			CronVar.Start()
		}
	} else {
		cwlog.DoLogCW("No schedule set, skipping.")
	}
}

func doMapReset() {
	buf := "Meep, reset map."
	cwlog.DoLogCW(buf)
	fmt.Println(buf)
}
