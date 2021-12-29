package fact

import (
	"ChatWire/botlog"
	"ChatWire/cfg"
	"ChatWire/constants"
	"ChatWire/glob"
	"time"

	"github.com/bwmarrin/discordgo"
)

//************************
//LOCK HANDLER FUNCTIONS
//************************

func UpdateRoleList() {
	glob.RoleListLock.Lock()
	g := glob.Guild

	if g != nil {
		glob.NitroList = []string{}
		glob.PatreonList = []string{}
		glob.ModeratorList = []string{}

		for _, m := range g.Members {
			for _, r := range m.Roles {
				if r == cfg.Global.RoleData.NitroRoleID {
					glob.NitroList = append(glob.NitroList, m.User.Username)
				} else if r == cfg.Global.RoleData.PatreonRoleID {
					glob.PatreonList = append(glob.PatreonList, m.User.Username)
				} else if r == cfg.Global.RoleData.ModeratorRoleID {
					glob.ModeratorList = append(glob.ModeratorList, m.User.Username)
				}
			}
		}
	}
	glob.RoleListLock.Unlock()
}

func GetUpdateWarnCounter() int {
	glob.UpdateWarnCounterLock.Lock()
	temp := glob.UpdateWarnCounter
	glob.UpdateWarnCounterLock.Unlock()

	return temp
}

func SetUpdateWarnCounter(val int) {
	glob.UpdateWarnCounterLock.Lock()
	glob.UpdateWarnCounter = val
	glob.UpdateWarnCounterLock.Unlock()
}

func GetManMinutes() int {
	glob.ManMinutesLock.Lock()
	temp := glob.ManMinutes
	glob.ManMinutesLock.Unlock()

	return temp
}

func SetManMinutes(val int) {
	glob.ManMinutesLock.Lock()
	glob.ManMinutes = val
	glob.ManMinutesLock.Unlock()
}

func GetDoUpdateFactorio() bool {
	glob.DoUpdateFactorioLock.Lock()
	temp := glob.DoUpdateFactorio
	glob.DoUpdateFactorioLock.Unlock()

	return temp
}

func SetDoUpdateFactorio(val bool) {
	glob.DoUpdateFactorioLock.Lock()
	glob.DoUpdateFactorio = val
	glob.DoUpdateFactorioLock.Unlock()
}

func GetNumLogins() int {
	glob.NumLoginsLock.RLock()
	temp := glob.NumLogins
	glob.NumLoginsLock.RUnlock()

	return temp
}

func GetPausedTicks() int {
	glob.PausedTicksLock.RLock()
	temp := glob.PausedTicks
	glob.PausedTicksLock.RUnlock()

	return temp
}

func SetPausedTicks(val int) {
	glob.PausedTicksLock.Lock()
	glob.PausedTicks = val
	glob.PausedTicksLock.Unlock()
}

func SetRelaunchThrottle(val int) {
	glob.RelaunchThrottleLock.Lock()
	glob.RelaunchThrottle = val
	glob.RelaunchThrottleLock.Unlock()
}

func GetRelaunchThrottle() int {
	glob.RelaunchThrottleLock.RLock()
	temp := glob.RelaunchThrottle
	glob.RelaunchThrottleLock.RUnlock()

	return temp
}

func SetFactorioBooted(isbooted bool) {
	glob.FactorioBootedLock.Lock()
	glob.FactorioBooted = isbooted
	if isbooted {
		glob.FactorioBootedAt = time.Now()
	} else {
		glob.FactorioBootedAt = time.Time{}
	}
	glob.FactorioBootedLock.Unlock()

}

func IsFactorioBooted() bool {
	glob.FactorioBootedLock.RLock()
	booted := glob.FactorioBooted
	glob.FactorioBootedLock.RUnlock()

	glob.FactIsRunningLock.RLock()
	running := glob.FactIsRunning
	glob.FactIsRunningLock.RUnlock()

	if booted && running {
		return true
	}

	return false
}

func SetModLoadString(str string) {
	glob.ModLoadLock.Lock()
	glob.ModLoadString = str
	glob.ModLoadLock.Unlock()
}

func AddModLoadString(str string) {
	glob.ModLoadLock.Lock()
	glob.ModLoadString = glob.ModLoadString + ", " + str
	glob.ModLoadLock.Unlock()
}

func GetModLoadString() string {
	glob.ModLoadLock.Lock()
	temp := glob.ModLoadString
	glob.ModLoadLock.Unlock()

	return temp
}

func SetNumPlayers(num int) {
	glob.NumPlayersLock.Lock()
	glob.NumPlayers = num
	glob.NumPlayersLock.Unlock()
}

func GetNumPlayers() int {
	glob.NumPlayersLock.RLock()
	temp := glob.NumPlayers
	glob.NumPlayersLock.RUnlock()

	return temp
}

func SetNoResponseCount(num int) {
	glob.NoResponseCountLock.Lock()
	glob.NoResponseCount = num
	glob.NoResponseCountLock.Unlock()
}

func GetNoResposeCount() int {
	glob.NoResponseCountLock.RLock()
	temp := glob.NoResponseCount
	glob.NoResponseCountLock.RUnlock()

	return temp
}

func IsSetRebootBot() bool {
	glob.DoRebootBotLock.RLock()
	temp := glob.DoRebootBot
	glob.DoRebootBotLock.RUnlock()

	return temp
}

func SetBotReboot(should bool) {
	glob.DoRebootBotLock.Lock()
	glob.DoRebootBot = should
	glob.DoRebootBotLock.Unlock()
}

func IsQueued() bool {
	glob.QueueReloadLock.RLock()
	temp := glob.QueueReload
	glob.QueueReloadLock.RUnlock()

	return temp
}

func SetQueued(queue bool) {
	glob.QueueReloadLock.Lock()
	glob.QueueReload = queue
	glob.QueueReloadLock.Unlock()
}

func SetAutoStart(auto bool) {
	glob.FactAutoStartLock.Lock()
	glob.FactAutoStart = auto
	glob.FactAutoStartLock.Unlock()
}

func IsSetAutoStart() bool {
	glob.FactAutoStartLock.RLock()
	temp := glob.FactAutoStart
	glob.FactAutoStartLock.RUnlock()

	return temp
}

func SetFactRunning(run bool, err bool) {
	glob.FactIsRunningLock.Lock()
	wasrun := glob.FactIsRunning
	glob.FactIsRunning = run
	glob.FactIsRunningLock.Unlock()

	if run && GetNoResposeCount() >= 10 {
		//CMS(cfg.Local.ChannelData.ChatID, "Server now appears to be responding again.")
		botlog.DoLog("Server now appears to be responding again.")
	}
	SetNoResponseCount(0)

	if wasrun != run {
		UpdateChannelName()
		if !run {
			FactorioIsOffline(err)
		}
		return
	}
}

func IsFactRunning() bool {
	glob.FactIsRunningLock.RLock()
	temp := glob.FactIsRunning
	glob.FactIsRunningLock.RUnlock()

	return temp
}

func GetGuild() *discordgo.Guild {
	glob.GuildLock.RLock()
	defer glob.GuildLock.RUnlock()

	return glob.Guild
}

func GetGuildName() string {
	glob.GuildLock.RLock()
	temp := glob.Guild
	glob.GuildLock.RUnlock()

	if temp == nil {
		return constants.Unknown
	} else {
		return glob.Guildname
	}
}

func GetGameTime() string {
	glob.GametimeLock.RLock()
	temp := glob.Gametime
	glob.GametimeLock.RUnlock()

	return temp
}

func SetGameTime(newtime string) {
	glob.GametimeLock.Lock()
	glob.Gametime = newtime
	glob.GametimeLock.Unlock()
}
