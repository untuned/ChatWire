package admin

import (
	"fmt"
	"io"
	"net/url"
	"os"
	"strings"
	"time"

	"ChatWire/cfg"
	"ChatWire/constants"
	"ChatWire/cwlog"
	"ChatWire/fact"
	"ChatWire/sclean"

	"github.com/bwmarrin/discordgo"
)

/* Archive map */
func ArchiveMap(s *discordgo.Session, i *discordgo.InteractionCreate) {

	fact.GameMapLock.Lock()
	defer fact.GameMapLock.Unlock()

	version := strings.Split(fact.FactorioVersion, ".")
	vlen := len(version)

	if vlen < 3 {
		cwlog.DoLogCW("Unable to determine Factorio version.")
		return
	}

	if fact.GameMapPath != "" && fact.FactorioVersion != constants.Unknown {
		shortversion := strings.Join(version[0:2], ".")

		t := time.Now()
		date := t.Format("2006-01-02")
		newmapname := fmt.Sprintf("%v-%v.zip", sclean.AlphaNumOnly(constants.MembersPrefix+cfg.Local.Callsign)+"-"+cfg.Local.Name, date)
		newmappath := fmt.Sprintf("%v%v maps/%v", cfg.Global.Paths.Folders.MapArchives, shortversion, newmapname)
		newmapurl := fmt.Sprintf("%v%v/%v", cfg.Global.Paths.URLs.ArchiveURL, url.PathEscape(shortversion+" maps"), url.PathEscape(newmapname))

		from, erra := os.Open(fact.GameMapPath)
		if erra != nil {
			cwlog.DoLogCW(fmt.Sprintf("An error occurred when attempting to open the map to archive. Details: %s", erra))
		}
		defer from.Close()

		/* Make directory if it does not exist */
		newdir := fmt.Sprintf("%s%s maps/", cfg.Global.Paths.Folders.MapArchives, shortversion)
		err := os.MkdirAll(newdir, os.ModePerm)
		if err != nil {
			cwlog.DoLogCW(err.Error())
		}

		to, errb := os.OpenFile(newmappath, os.O_RDWR|os.O_CREATE, 0666)
		if errb != nil {
			cwlog.DoLogCW(fmt.Sprintf("An error occurred when attempting to create the archive map file. Details: %s", errb))
		}
		defer to.Close()

		_, errc := io.Copy(to, from)
		if errc != nil {
			cwlog.DoLogCW(fmt.Sprintf("An error occurred when attempting to write the archived map. Details: %s", errc))
		}

		var buf string
		if erra == nil && errb == nil && errc == nil {
			buf = fmt.Sprintf("Map archived as: %s", newmapurl)
		} else {
			buf = "Map archive failed."
		}

		fact.CMS(i.ChannelID, buf)
	} else {
		fact.CMS(i.ChannelID, "No map has been loaded yet.")
	}

}
