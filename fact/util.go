package fact

import (
	"fmt"
	"os"
	"strings"
	"time"

	"ChatWire/cfg"
	"ChatWire/constants"
	"ChatWire/cwlog"
	"ChatWire/disc"
	"ChatWire/glob"
	"ChatWire/sclean"
)

func IsPlayerOnline(who string) bool {

	if len(who) <= 0 {
		return false
	}

	OnlinePlayersLock.RLock()
	defer OnlinePlayersLock.RUnlock()

	for _, p := range glob.OnlinePlayers {
		if strings.EqualFold(p.Name, who) {
			return true
		}
	}

	return false
}

/* Send chat to factorio */
func FactChat(input string) {

	/* Limit length, Discord does this... but just in case */
	input = sclean.TruncateStringEllipsis(input, 2048)

	/*
		* If we are running our softmod, use the much safer
		 * /cchat comamnd
	*/
	if glob.SoftModVersion != constants.Unknown {
		WriteFact("/cchat " + input)
	} else {
		/*
		 * Just in case there is no soft-mod,
		 * filter out potential threats
		 */
		input = sclean.StripControlAndSubSpecial(input)
		input = sclean.RemoveFactorioTags(input)
		input = sclean.RemoveDiscordMarkdown(input)

		/* Attempt to prevent anyone from running a command. */
		strlen := len(input)
		for z := 0; z < strlen; z++ {
			input = strings.TrimLeft(input, " ")
			input = strings.TrimLeft(input, "/")
			input = strings.TrimRight(input, " ")
		}
		WriteFact(input)
	}
}

func WaitFactQuit() {
	for x := 0; x < constants.MaxFactorioCloseWait && FactIsRunning && glob.ServerRunning; x++ {
		time.Sleep(time.Millisecond * 100)
	}

}

/* Auto generates a steam connect URL */
func MakeSteamURL() (string, bool) {
	if cfg.Global.Paths.URLs.Domain != "localhost" && cfg.Global.Paths.URLs.Domain != "" {
		buf := fmt.Sprintf("steam://run/427520//--mp-connect%%20%v:%v/", cfg.Global.Paths.URLs.Domain, cfg.Local.Port)
		return buf, true
	} else {
		return "(not configured)", false
	}
}

/* Program shutdown */
func DoExit(delay bool) {

	if CronVar != nil {
		CronVar.Stop()
	}

	//Wait a few seconds for CMS to finish
	for i := 0; i < 15; i++ {
		if len(disc.CMSBuffer) > 0 {
			time.Sleep(time.Second)
		}
	}

	/* This kills all loops! */
	glob.ServerRunning = false
	time.Sleep(time.Second)
	cwlog.DoLogCW("CW closing, load/save db.")
	//LoadPlayers(false)
	WritePlayers()

	/* File locks */
	glob.PlayerListWriteLock.Lock()

	cwlog.DoLogCW("Closing log files.")
	glob.GameLogDesc.Close()
	glob.CWLogDesc.Close()

	_ = os.Remove("cw.lock")
	/* Logs are closed, don't report */

	os.Remove("log/newest.log")
	fmt.Println("Goodbye.")
	if delay {
		time.Sleep(constants.ErrorDelayShutdown * time.Second)
	}

	if disc.DS != nil {
		time.Sleep(time.Second)
		disc.DS.Close()
	}
	os.Exit(1)
}

func AddFactColor(color string, text string) string {
	text = sclean.StripControlAndSubSpecial(text)
	text = sclean.RemoveDiscordMarkdown(text)
	text = sclean.RemoveFactorioTags(text)

	color = sclean.StripControlAndSubSpecial(color)
	color = sclean.RemoveFactorioTags(color)
	color = strings.TrimSpace(color)
	color = strings.ToLower(color)

	return fmt.Sprintf("[color=%v]%v[/color]", color, text)
}

/* Generate full path to Factorio binary */
func GetFactorioBinary() string {
	bloc := ""
	if strings.HasPrefix(cfg.Global.Paths.Binaries.FactBinary, "/") {
		/* Absolute path */
		bloc = cfg.Global.Paths.Binaries.FactBinary
	} else {
		/* Relative path */
		bloc = cfg.Global.Paths.Folders.ServersRoot +
			cfg.Global.Paths.ChatWirePrefix +
			cfg.Local.Callsign + "/" +
			cfg.Global.Paths.Folders.FactorioDir + "/" +
			cfg.Global.Paths.Binaries.FactBinary
	}
	return bloc
}

/* Write a Discord message to the buffer */
func CMS(channel string, text string) {

	/* Split at newlines, so we can batch neatly */
	lines := strings.Split(text, "\n")

	disc.CMSBufferLock.Lock()

	for _, line := range lines {

		if len(line) <= 2000 {
			var item disc.CMSBuf
			item.Channel = channel
			item.Text = line

			disc.CMSBuffer = append(disc.CMSBuffer, item)
		} else {
			cwlog.DoLogCW("CMS: Line too long! Discarding...")
		}
	}

	disc.CMSBufferLock.Unlock()
}

/* Log AND send this message to Discord */
func LogCMS(channel string, text string) {
	cwlog.DoLogCW(text)
	CMS(channel, text)
}
