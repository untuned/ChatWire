package support

import (
	"fmt"
	"math/rand"
	"os"
	"os/exec"
	"strings"
	"syscall"
	"time"

	"github.com/bwmarrin/discordgo"

	"ChatWire/banlist"
	"ChatWire/cfg"
	"ChatWire/constants"
	"ChatWire/cwlog"
	"ChatWire/disc"
	"ChatWire/fact"
	"ChatWire/glob"
	"ChatWire/modupdate"
)

func LinuxSetProcessGroup(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
}

/********************
 * Main threads/loops
 ********************/

func MainLoops() {

	time.Sleep(time.Second * 1)

	/***************
	 * Game watchdog
	 ***************/
	go func() {
		for glob.ServerRunning {

			time.Sleep(constants.WatchdogInterval)

			/* Check for updates, or reboot flags */
			if !fact.FactIsRunning &&
				(fact.QueueReload || glob.DoRebootCW || fact.DoUpdateFactorio) {
				if fact.DoUpdateFactorio {
					fact.FactUpdate()
					fact.DoExit(false)
					return
				}
				fact.DoExit(false)
				return

				/* We are running normally */
			} else if fact.FactIsRunning && fact.FactorioBooted {

				/* If the game isn't paused, check game time */
				nores := 0
				if fact.PausedTicks <= constants.PauseThresh {

					glob.NoResponseCount = glob.NoResponseCount + 1
					nores = glob.NoResponseCount

					fact.WriteFact("/time")
				}
				/* Just in case factorio hangs, bogs down or is flooded */
				if nores == 120 {
					msg := "Factorio unresponsive for over two minutes... rebooting."
					fact.LogCMS(cfg.Local.Channel.ChatChannel, msg)
					glob.RelaunchThrottle = 0
					fact.QuitFactorio(msg)

					fact.WaitFactQuit()
					fact.FactorioBooted = false
					fact.SetFactRunning(false)
				}

				/* We aren't running, but should be! */
			} else if !fact.FactIsRunning && fact.FactAutoStart && !fact.DoUpdateFactorio && !*glob.NoAutoLaunch {
				/* Don't relaunch if we are set to auto update */
				launchFactorio()
			}
		}
	}()

	/********************************
	 * Watch ban file for changes
	 ********************************/
	go banlist.WatchBanFile()

	/*************************************************
	 *  Send buffered messages to Discord, batched.
	 *************************************************/
	go func() {
		for glob.ServerRunning {

			if disc.DS != nil {

				/* Check if buffer is active */
				active := false
				disc.CMSBufferLock.Lock()
				if disc.CMSBuffer != nil {
					active = true
				}
				disc.CMSBufferLock.Unlock()

				/* If buffer is active, sleep and wait for it to fill up */
				if active {
					time.Sleep(constants.CMSRate)

					/* Waited for buffer to fill up, grab and clear buffers */
					disc.CMSBufferLock.Lock()
					lcopy := disc.CMSBuffer
					disc.CMSBuffer = nil
					disc.CMSBufferLock.Unlock()

					if lcopy != nil {

						var factmsg []string
						var moder []string

						/* Put messages into proper lists */
						for _, msg := range lcopy {
							if strings.EqualFold(msg.Channel, cfg.Local.Channel.ChatChannel) {
								factmsg = append(factmsg, msg.Text)
							} else if strings.EqualFold(msg.Channel, cfg.Global.Discord.ReportChannel) {
								moder = append(moder, msg.Text)
							} else {
								disc.SmartWriteDiscord(msg.Channel, msg.Text)
							}
						}

						/* Send out buffer, split up if needed */
						/* Factorio */
						buf := ""

						for _, line := range factmsg {
							oldlen := len(buf) + 1
							addlen := len(line)
							if oldlen+addlen >= 2000 {
								disc.SmartWriteDiscord(cfg.Local.Channel.ChatChannel, buf)
								buf = line
							} else {
								buf = buf + "\n" + line
							}
						}
						if buf != "" {
							disc.SmartWriteDiscord(cfg.Local.Channel.ChatChannel, buf)
						}

						/* Moderation */
						buf = ""
						for _, line := range moder {
							oldlen := len(buf) + 1
							addlen := len(line)
							if oldlen+addlen >= 2000 {
								disc.SmartWriteDiscord(cfg.Global.Discord.ReportChannel, buf)
								buf = line
							} else {
								buf = buf + "\n" + line
							}
						}
						if buf != "" {
							disc.SmartWriteDiscord(cfg.Global.Discord.ReportChannel, buf)
						}
					}

					/* Don't send any more messages for a while (throttle) */
					time.Sleep(constants.CMSRestTime)
				}

			}

			/* Sleep for a moment before checking buffer again */
			time.Sleep(constants.CMSPollRate)
		}
	}()

	/************************************
	 * Delete expired registration codes
	 ************************************/
	go func() {

		for glob.ServerRunning {
			time.Sleep(1 * time.Minute)

			t := time.Now()

			glob.PasswordListLock.Lock()
			for _, pass := range glob.PassList {
				if (t.Unix() - pass.Time) > 300 {
					cwlog.DoLogCW("Invalidating unused registration code for player: " + disc.GetNameFromID(pass.DiscID, false))
					delete(glob.PassList, pass.DiscID)
				}
			}
			glob.PasswordListLock.Unlock()
		}
	}()

	/********************************
	 * Save database, if marked dirty
	 ********************************/
	go func() {
		time.Sleep(time.Minute)

		for glob.ServerRunning {
			time.Sleep(5 * time.Second)

			wasDirty := false

			glob.PlayerListDirtyLock.Lock()

			if glob.PlayerListDirty {
				glob.PlayerListDirty = false
				wasDirty = true
				/* Prevent recursive lock */
				go func() {
					cwlog.DoLogCW("Database marked dirty, saving.")
					fact.WritePlayers()
				}()
			}
			glob.PlayerListDirtyLock.Unlock()

			/* Sleep after saving */
			if wasDirty {
				time.Sleep(10 * time.Second)
			}
		}
	}()

	/***********************************************************
	 * Save database (less often), if last seen is marked dirty
	 ***********************************************************/
	go func() {
		time.Sleep(time.Minute)

		for glob.ServerRunning {
			time.Sleep(5 * time.Minute)
			glob.PlayerListSeenDirtyLock.Lock()

			if glob.PlayerListSeenDirty {
				glob.PlayerListSeenDirty = false

				/* Prevent deadlock */
				go func() {
					//cwlog.DoLogCW("Database last seen flagged, saving.")
					fact.WritePlayers()
				}()
			}
			glob.PlayerListSeenDirtyLock.Unlock()
		}
	}()

	/************************************
	 * Database file modification watching
	 ************************************/
	go fact.WatchDatabaseFile()

	/* Read database, if the file was modifed */
	go func() {
		updated := false

		time.Sleep(time.Second * 30)

		for glob.ServerRunning {

			time.Sleep(5 * time.Second)

			/* Detect update */
			glob.PlayerListUpdatedLock.Lock()
			if glob.PlayerListUpdated {
				updated = true
				glob.PlayerListUpdated = false
			}
			glob.PlayerListUpdatedLock.Unlock()

			if updated {
				updated = false

				//cwlog.DoLogCW("Database file modified, loading.")
				fact.LoadPlayers(false)

				/* Sleep after reading */
				time.Sleep(5 * time.Second)
			}

		}
	}()

	/***************************
	 * Get Guild information
	 * Needed for Discord roles
	 ***************************/
	go func() {
		for glob.ServerRunning {

			/* Get guild id, if we need it */

			if disc.Guild == nil && disc.DS != nil {
				var nguild *discordgo.Guild
				var err error

				/*  Attempt to get the guild from the state,
				 *  If there is an error, fall back to the restapi. */
				nguild, err = disc.DS.State.Guild(cfg.Global.Discord.Guild)
				if err != nil {
					nguild, err = disc.DS.Guild(cfg.Global.Discord.Guild)
					if err != nil {
						cwlog.DoLogCW("Failed to get valid guild data, giving up.")
						break
					}
				}

				if err != nil {
					cwlog.DoLogCW(fmt.Sprintf("Was unable to get guild data from GuildID: %s", err))

					break
				}
				if nguild == nil || err != nil {
					disc.Guildname = constants.Unknown
					cwlog.DoLogCW("Guild data came back nil.")
					break
				} else {

					/* Guild found, exit loop */
					disc.Guild = nguild
					disc.Guildname = nguild.Name
					cwlog.DoLogCW("Guild data linked.")
				}
			}

			/* Update role IDs */
			if disc.Guild != nil {
				changed := false
				for _, role := range disc.Guild.Roles {
					if cfg.Global.Discord.Roles.Admin != "" &&
						role.Name == cfg.Global.Discord.Roles.Admin &&
						role.ID != "" && cfg.Global.Discord.Roles.RoleCache.Admin != role.ID {
						cfg.Global.Discord.Roles.RoleCache.Admin = role.ID
						changed = true

					} else if cfg.Global.Discord.Roles.Moderator != "" &&
						role.Name == cfg.Global.Discord.Roles.Moderator &&
						role.ID != "" && cfg.Global.Discord.Roles.RoleCache.Moderator != role.ID {
						cfg.Global.Discord.Roles.RoleCache.Moderator = role.ID
						changed = true

					} else if cfg.Global.Discord.Roles.Regular != "" &&
						role.Name == cfg.Global.Discord.Roles.Regular &&
						role.ID != "" && cfg.Global.Discord.Roles.RoleCache.Regular != role.ID {
						cfg.Global.Discord.Roles.RoleCache.Regular = role.ID
						changed = true

					} else if cfg.Global.Discord.Roles.Member != "" &&
						role.Name == cfg.Global.Discord.Roles.Member &&
						role.ID != "" && cfg.Global.Discord.Roles.RoleCache.Member != role.ID {
						cfg.Global.Discord.Roles.RoleCache.Member = role.ID
						changed = true

					} else if cfg.Global.Discord.Roles.New != "" &&
						role.Name == cfg.Global.Discord.Roles.New &&
						role.ID != "" && cfg.Global.Discord.Roles.RoleCache.New != role.ID {
						cfg.Global.Discord.Roles.RoleCache.New = role.ID
						changed = true
					} else if cfg.Global.Discord.Roles.Patreon != "" &&
						role.Name == cfg.Global.Discord.Roles.Patreon &&
						role.ID != "" && cfg.Global.Discord.Roles.RoleCache.Patreon != role.ID {
						cfg.Global.Discord.Roles.RoleCache.Patreon = role.ID
						changed = true
					} else if cfg.Global.Discord.Roles.Nitro != "" &&
						role.Name == cfg.Global.Discord.Roles.Nitro &&
						role.ID != "" && cfg.Global.Discord.Roles.RoleCache.Nitro != role.ID {
						cfg.Global.Discord.Roles.RoleCache.Nitro = role.ID
						changed = true
					}
				}
				if changed {
					cwlog.DoLogCW("Role IDs updated.")
					cfg.WriteGCfg()
				}
			}

			time.Sleep(time.Minute)
		}
	}()

	/*******************************
	 * Update patreon/nitro players
	 *******************************/
	go func() {
		time.Sleep(time.Minute)
		for glob.ServerRunning {

			if fact.FactorioBooted {
				disc.UpdateRoleList()

				/* Live update server description */
				if disc.RoleListUpdated {
					/* goroutine, avoid deadlock */

					ConfigSoftMod()
					fact.GenerateFactorioConfig()
				}
				disc.RoleListUpdated = false
			}
			time.Sleep(time.Minute * 15)
		}
	}()

	/************************************
	 * Reboot if queued, when server empty
	 ************************************/
	go func() {

		for glob.ServerRunning {
			time.Sleep(2 * time.Second)

			if fact.QueueReload && fact.NumPlayers == 0 && !fact.DoUpdateFactorio {
				if fact.FactIsRunning && fact.FactorioBooted {
					cwlog.DoLogCW("No players currently online, performing scheduled reboot.")
					fact.QuitFactorio("Server rebooting for maintenance.")
					break //We don't need to loop anymore
				}
			}
			if fact.DoUpdateFactorio && fact.NumPlayers == 0 {
				if fact.FactIsRunning && fact.FactorioBooted {
					cwlog.DoLogCW("Stopping Factorio for update.")
					fact.QuitFactorio("")
					break //We don't need to loop anymore
				}
			}
		}
	}()

	/*******************************************
	 * Bug players if there is an pending update
	 *******************************************/
	go func() {

		for glob.ServerRunning {
			time.Sleep(30 * time.Second)

			if cfg.Local.Options.AutoUpdate {
				if fact.FactIsRunning && fact.FactorioBooted && fact.NewVersion != constants.Unknown {
					if fact.NumPlayers > 0 {

						/* Warn players */
						if glob.UpdateWarnCounter < glob.UpdateGraceMinutes {
							msg := fmt.Sprintf("(SYSTEM) Factorio update waiting (%v), please log off as soon as there is a good stopping point, players on the upgraded version will be unable to connect (%vm grace remaining)!", fact.NewVersion, glob.UpdateGraceMinutes-glob.UpdateWarnCounter)
							fact.CMS(cfg.Local.Channel.ChatChannel, msg)
							fact.FactChat(fact.AddFactColor("orange", msg))
						}
						time.Sleep(2 * time.Minute)

						/* Reboot anyway */
						if glob.UpdateWarnCounter > glob.UpdateGraceMinutes {
							msg := "(SYSTEM) Rebooting for Factorio update."
							fact.CMS(cfg.Local.Channel.ChatChannel, msg)
							fact.FactChat(fact.AddFactColor("orange", msg))
							glob.UpdateWarnCounter = 0
							fact.QuitFactorio("Rebooting for Factorio update.")
							break /* Stop looping */
						}
						glob.UpdateWarnCounter = (glob.UpdateWarnCounter + 1)
					} else {
						glob.UpdateWarnCounter = 0
						fact.QuitFactorio("Rebooting for Factorio update.")
						break /* Stop looping */
					}
				}
			}
		}
	}()

	/*********************
	 * Check signal files
	 *********************/
	go func() {
		clearOldSignals()
		failureReported := false
		for glob.ServerRunning {

			time.Sleep(10 * time.Second)

			var err error
			var errb error

			/* Queued reboots, regardless of game state */
			if _, err = os.Stat(".queue"); err == nil {
				if errb = os.Remove(".queue"); errb == nil {
					if !fact.QueueReload {
						fact.QueueReload = true
						cwlog.DoLogCW("Reboot queued!")
					}
				} else if errb != nil && !failureReported {
					failureReported = true
					fact.LogCMS(cfg.Local.Channel.ChatChannel, "Failed to remove .queue file, ignoring.")
				}
			}
			/* Halt, regardless of game state */
			if _, err = os.Stat(".halt"); err == nil {
				if errb = os.Remove(".halt"); errb == nil {
					if fact.FactIsRunning || fact.FactorioBooted {
						fact.LogCMS(cfg.Local.Channel.ChatChannel, "ChatWire is halting, closing Factorio.")
						fact.FactAutoStart = false
						fact.QuitFactorio("Server halted, quitting Factorio.")
						fact.WaitFactQuit()
						fact.DoExit(false)
					} else {
						fact.LogCMS(cfg.Local.Channel.ChatChannel, "ChatWire is halting.")
						fact.DoExit(false)
					}
				} else if errb != nil && !failureReported {
					failureReported = true
					fact.LogCMS(cfg.Local.Channel.ChatChannel, "Failed to remove .halt file, ignoring.")
				}
			}

			/* Only if game is running */
			if fact.FactIsRunning && fact.FactorioBooted {
				/* Quick reboot */
				if _, err = os.Stat(".qrestart"); err == nil {
					if errb = os.Remove(".qrestart"); errb == nil {
						fact.LogCMS(cfg.Local.Channel.ChatChannel, "Factorio quick restarting!")
						fact.QuitFactorio("Server quick restarting...")
					} else if errb != nil && !failureReported {
						failureReported = true
						fact.LogCMS(cfg.Local.Channel.ChatChannel, "Failed to remove .qrestart file, ignoring.")
					}
				}
				/* Stop game */
				if _, err = os.Stat(".stop"); err == nil {
					if errb = os.Remove(".stop"); errb == nil {
						fact.LogCMS(cfg.Local.Channel.ChatChannel, "Factorio stopping!")
						fact.FactAutoStart = false
						fact.QuitFactorio("Server manually stopped.")
					} else if errb != nil && !failureReported {
						failureReported = true
						fact.LogCMS(cfg.Local.Channel.ChatChannel, "Failed to remove .stop file, ignoring.")
					}
				}
			} else { /*  Only if game is NOT running */
				/* Start game */
				if _, err = os.Stat(".start"); err == nil {
					if errb = os.Remove(".start"); errb == nil {
						fact.FactAutoStart = true
						fact.LogCMS(cfg.Local.Channel.ChatChannel, "Factorio starting!")
					} else if errb != nil && !failureReported {
						failureReported = true
						fact.LogCMS(cfg.Local.Channel.ChatChannel, "Failed to remove .start file, ignoring.")
					}
				}
			}
		}
	}()

	/***********************************
	 * Fix lost connection to log files
	 ***********************************/
	go func() {

		for glob.ServerRunning {
			time.Sleep(time.Second * 30)

			var err error
			if _, err = os.Stat(glob.CWLogName); err != nil {

				glob.CWLogDesc.Close()
				glob.CWLogDesc = nil
				cwlog.StartCWLog()
				cwlog.DoLogCW("CWLog file was deleted, recreated.")
			}

			if _, err = os.Stat(glob.GameLogName); err != nil {
				glob.GameLogDesc.Close()
				glob.GameLogDesc = nil
				cwlog.StartGameLog()
				cwlog.DoLogGame("GameLog file was deleted, recreated.")
			}
		}
	}()

	/****************************
	* Check for Factorio updates
	****************************/
	go func() {
		time.Sleep(time.Minute)

		for glob.ServerRunning {
			time.Sleep(time.Second * time.Duration(rand.Intn(300))) //Add 5 minutes of randomness
			time.Sleep(time.Minute * 30)
			fact.CheckFactUpdate(false)
		}
	}()

	/****************************
	* Refresh channel names
	****************************/
	go func() {

		time.Sleep(time.Second * 15)
		for glob.ServerRunning {
			fact.UpdateChannelName()

			disc.UpdateChannelLock.Lock()
			chname := disc.NewChanName
			oldchname := disc.OldChanName
			disc.UpdateChannelLock.Unlock()

			if oldchname != chname {
				fact.DoUpdateChannelName()
			}

			time.Sleep(time.Second * 10)
		}
	}()

	/* Check for expired pauses */
	go func() {
		for glob.ServerRunning {
			glob.PausedLock.Lock()

			if glob.PausedForConnect {

				limit := time.Minute
				if glob.PausedConnectAttempt {
					limit = time.Minute * 2
				}

				if time.Since(glob.PausedAt) > limit {

					fact.WriteFact(
						fmt.Sprintf("/gspeed %0.2f", cfg.Local.Options.Speed))

					if glob.PausedConnectAttempt {
						fact.CMS(cfg.Local.Channel.ChatChannel, "Unpausing, "+glob.PausedFor+" did not finish joining within the time limit.")
					} else {
						fact.CMS(cfg.Local.Channel.ChatChannel, "Pause-on-connect canceled, "+glob.PausedFor+" did not attempt to connect within the time limit.")
					}

					glob.PausedForConnect = false
					glob.PausedFor = ""
					glob.PausedConnectAttempt = false
				}
			} else {
				/* Eventually reset timers */
				if glob.PausedCount > 0 {
					if time.Since(glob.PausedAt) > time.Minute*30 {
						glob.PausedCount = 0
						glob.PausedAt = time.Now()
						glob.PausedFor = ""
						glob.PausedConnectAttempt = false
					}
				}
			}
			glob.PausedLock.Unlock()
			time.Sleep(time.Second * 2)
		}
	}()

	/**********************************
	* Poll online players
	* Just in case there is no soft-mod
	* Also detect paused servers that
	* are frozen
	**********************************/
	var slowOCheck = (time.Minute * 15)
	var fastOCheck = (time.Second * 15)

	go func() {
		time.Sleep(time.Second * 30)

		for {
			if fact.FactIsRunning && fact.FactorioBooted {

				if glob.SoftModVersion != constants.Unknown {
					time.Sleep(slowOCheck)
				} else { /* Run often if no soft-mod */
					if fact.PausedTicks <= constants.PauseThresh {
						time.Sleep(fastOCheck)
					} else {
						time.Sleep(slowOCheck)
					}
				}

				if fact.FactIsRunning && fact.FactorioBooted {
					fact.WriteFact(glob.OnlineCommand)
				}

			}
			time.Sleep(time.Second)

		}
	}()

	/****************************/
	/* Check for mod update     */
	/****************************/
	go func() {
		time.Sleep(time.Minute * 5)

		for glob.ServerRunning &&
			cfg.Local.Options.ModUpdate {
			modupdate.CheckMods(false, false)

			time.Sleep(time.Hour * 3)
		}
	}()

	/****************************/
	/* Update player time       */
	/****************************/
	go func() {
		time.Sleep(time.Second * 15)
		for glob.ServerRunning {
			glob.PlayerListLock.Lock() //Lock
			for _, p := range glob.PlayerList {
				if time.Since(fact.ExpandTime(p.LastSeen)) <= time.Minute {
					p.Minutes++
				}
			}
			glob.PlayerListLock.Unlock() //Unlock
			time.Sleep(time.Minute)
		}
	}()

	/****************************/
	/* Update time till reset   */
	/****************************/
	go func() {
		var lastDur string
		time.Sleep(time.Second * 15)
		for glob.ServerRunning {
			if glob.SoftModVersion != constants.Unknown &&
				fact.FactIsRunning &&
				fact.FactorioBooted &&
				fact.NumPlayers > 0 {
				time.Sleep(time.Minute)

				fact.UpdateScheduleDesc()
				if fact.TillReset != "" && cfg.Local.Options.Schedule != "" {
					buf := "/resetdur " + fact.TillReset + " (" + strings.ToUpper(cfg.Local.Options.Schedule) + ")"
					/* Don't write it, if nothing has changed */
					if !strings.EqualFold(buf, lastDur) {
						fact.WriteFact(buf)
					}

					lastDur = buf
				}
			}

			time.Sleep(time.Second * 5)

		}
	}()

	/****************************/
	/* Auto delete modpack files
	 * at the set expire time
	/****************************/
	go func() {
		delme := -1

		time.Sleep(time.Minute)
		for glob.ServerRunning {

			time.Sleep(time.Second * 15)
			numItems := len(cfg.Local.ModPackList)

			if numItems > 0 {
				for i, item := range cfg.Local.ModPackList {
					if item.Path == "" {
						delme = i
						break
					} else if time.Since(item.Created) > (constants.ModPackLifeMins * time.Minute) {
						delme = i
						break
					}
				}
				if delme >= 0 {
					err := os.Remove(cfg.Local.ModPackList[delme].Path)
					if err != nil {
						cwlog.DoLogCW("Unable to delete expired modpack!")
					}

					cwlog.DoLogCW("Deleted expired modpack: " + cfg.Local.ModPackList[delme].Path)
					if numItems > 1 {
						cfg.Local.ModPackList = append(cfg.Local.ModPackList[:delme], cfg.Local.ModPackList[delme+1:]...)
					} else {
						cfg.Local.ModPackList = []cfg.ModPackData{}
					}
				}
				cfg.WriteLCfg()
			}
		}
	}()

}
