package fact

import (
	"bytes"
	"encoding/base64"
	"encoding/binary"
	"fmt"
	"io"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"

	"ChatWire/cfg"
	"ChatWire/constants"
	"ChatWire/cwlog"
	"ChatWire/disc"
	"ChatWire/glob"
	"ChatWire/sclean"
)

func GetMapTypeNum(mapt string) int {
	i := 0

	if cfg.Local.Settings.MapGenerator != "" && !strings.EqualFold(cfg.Local.Settings.MapGenerator, "none") {
		return 0
	}
	for i = 0; i < len(constants.MapTypes); i = i + 1 {
		if strings.EqualFold(constants.MapTypes[i], mapt) {
			return i
		}
	}
	return -1
}

func GetMapTypeName(num int) string {

	numMaps := len(constants.MapTypes)
	if num >= 0 && num < numMaps {
		return constants.MapTypes[num]
	}
	return "Error"
}

/* Generate map */
func Map_reset(data string, doReport bool) {

	/* Get Factorio version, for archive folder name */
	version := strings.Split(FactorioVersion, ".")
	vlen := len(version)
	if vlen < 3 {
		buf := "Unable to determine Factorio version."
		cwlog.DoLogCW(buf)
		if doReport {
			CMS(cfg.Local.Channel.ChatChannel, buf)
		}
		return
	}

	/* If Factorio is running, and there is a argument... echo it
	 * Otherwise, stop Factorio and generate a new map */
	if FactorioBooted || FactIsRunning {
		if data != "" {
			if !doReport {

				//Silence messages if we are skipping
				if cfg.Local.Options.SkipReset {
					return
				}
			}
			CMS(cfg.Local.Channel.ChatChannel, sclean.EscapeDiscordMarkdown(data))
			FactChat(AddFactColor("orange", data))
			QueueReload = false      //Skip queued reboot
			DoUpdateFactorio = false //Skip queued updates
			return
		} else {

			/* Turn off skip reset flag regardless of reset reason */
			if cfg.Local.Options.SkipReset {
				cfg.Local.Options.SkipReset = false
				cfg.WriteLCfg()

				/*Don't reset if this is an automatic reset, otherwise proceed. */
				if !doReport {
					return
				}
			}
			CMS(cfg.Local.Channel.ChatChannel, "Stopping server, for map reset.")

			cfg.Local.Options.SkipReset = false
			QueueReload = false      //Skip queued reboot
			DoUpdateFactorio = false //Skip queued updates
			cfg.WriteLCfg()

			FactAutoStart = false
			QuitFactorio("Server rebooting for map reset...")
		}
	}

	/* Wait for server to stop if running */
	WaitFactQuit()

	/* Only proceed if we were running a map, and we know our Factorio version. */
	if GameMapPath != "" && FactorioVersion != constants.Unknown {
		shortversion := strings.Join(version[0:2], ".")

		t := time.Now()
		date := t.Format("2006-01-02")
		newmapname := fmt.Sprintf("%v-%v.zip", cfg.Local.Callsign+"-"+cfg.Local.Name, date)
		newmappath := fmt.Sprintf("%v%v%v%v%v", cfg.Global.Paths.Folders.MapArchives, shortversion, constants.ArchiveFolderSuffix, "/", newmapname)
		newmapurl := fmt.Sprintf("https://%v%v%v%v%v%v",
			cfg.Global.Paths.URLs.Domain,
			cfg.Global.Paths.URLs.PathPrefix,
			cfg.Global.Paths.URLs.ArchivePath,
			url.PathEscape(shortversion+constants.ArchiveFolderSuffix),
			"/",
			url.PathEscape(newmapname))

		from, erra := os.Open(GameMapPath)
		if erra != nil {

			buf := fmt.Sprintf("An error occurred when attempting to read the map to archive: %s", erra)
			cwlog.DoLogCW(buf)
			CMS(cfg.Local.Channel.ChatChannel, buf)
			return
		}
		defer from.Close()

		/* Attach map, send to chat */
		dData := &discordgo.MessageSend{Files: []*discordgo.File{
			{Name: newmapname, Reader: from, ContentType: "application/zip"}}}
		_, err := disc.DS.ChannelMessageSendComplex(cfg.Local.Channel.ChatChannel, dData)
		if err != nil {
			cwlog.DoLogCW(err.Error())
		}

		_, err = from.Seek(0, io.SeekStart)
		if err != nil {
			cwlog.DoLogCW(err.Error())
		}

		/* Make directory if it does not exist */
		newdir := fmt.Sprintf("%v%v%v/", cfg.Global.Paths.Folders.MapArchives, shortversion, constants.ArchiveFolderSuffix)
		err = os.MkdirAll(newdir, os.ModePerm)
		if err != nil {
			cwlog.DoLogCW(err.Error())
		}

		to, errb := os.OpenFile(newmappath, os.O_RDWR|os.O_CREATE, 0666)
		if errb != nil {
			buf := fmt.Sprintf("An error occurred when attempting to create the map archive file: %s", errb)
			cwlog.DoLogCW(buf)
			CMS(cfg.Local.Channel.ChatChannel, buf)
			return
		}
		defer to.Close()

		_, errc := io.Copy(to, from)
		if errc != nil {
			buf := fmt.Sprintf("An error occurred when attempting to write the map archive file: %s", errc)
			cwlog.DoLogCW(buf)
			CMS(cfg.Local.Channel.ChatChannel, buf)
			return
		}

		var buf string
		if erra == nil && errb == nil && errc == nil {
			buf = fmt.Sprintf("Map archived as: %s", newmapurl)
			CMS(cfg.Local.Channel.ChatChannel, buf)
		} else {
			buf = "Map archive failed."
			cwlog.DoLogCW(buf)
			CMS(cfg.Local.Channel.ChatChannel, buf)
			return
		}
	}

	genpath := cfg.Global.Paths.Folders.ServersRoot +
		cfg.Global.Paths.ChatWirePrefix +
		cfg.Local.Callsign + "/" +
		cfg.Global.Paths.Folders.FactorioDir + "/" +
		cfg.Global.Paths.Folders.Saves

	flist, err := filepath.Glob(genpath + "/gen-*.zip")
	if err != nil {
		panic(err)
	}
	for _, f := range flist {
		if err := os.Remove(f); err != nil {
			cwlog.DoLogCW("Failed to delete: " + f)
		}
	}

	t := time.Now()
	ourseed := int(t.UnixNano() - constants.CWEpoch)

	//Use seed if specified, then clear it
	if cfg.Local.Settings.Seed > 0 {
		ourseed = cfg.Local.Settings.Seed
		cfg.Local.Settings.Seed = 0
		cfg.WriteLCfg()
	}

	MapPreset := cfg.Local.Settings.MapPreset

	if strings.EqualFold(MapPreset, "error") {
		buf := "Invalid map preset."
		cwlog.DoLogCW(buf)
		CMS(cfg.Local.Channel.ChatChannel, buf)
		return
	}

	CMS(cfg.Local.Channel.ChatChannel, "Generating map...")

	/* Generate code to make filename */
	buf := new(bytes.Buffer)

	_ = binary.Write(buf, binary.BigEndian, uint64(ourseed))
	ourcode := fmt.Sprintf("%02d%v", GetMapTypeNum(MapPreset), base64.RawURLEncoding.EncodeToString(buf.Bytes()))
	filename := cfg.Global.Paths.Folders.ServersRoot +
		cfg.Global.Paths.ChatWirePrefix +
		cfg.Local.Callsign + "/" +
		cfg.Global.Paths.Folders.FactorioDir + "/" +
		cfg.Global.Paths.Folders.Saves +
		"/gen-" + ourcode + ".zip"

	factargs := []string{"--map-gen-seed", fmt.Sprintf("%v", ourseed), "--create", filename}

	/* Append map gen if set */
	if cfg.Local.Settings.MapGenerator != "" && !strings.EqualFold(cfg.Local.Settings.MapGenerator, "none") {
		factargs = append(factargs, "--map-gen-settings")
		factargs = append(factargs, cfg.Global.Paths.Folders.ServersRoot+cfg.Global.Paths.Folders.MapGenerators+"/"+cfg.Local.Settings.MapGenerator+"-gen.json")

		factargs = append(factargs, "--map-settings")
		factargs = append(factargs, cfg.Global.Paths.Folders.ServersRoot+cfg.Global.Paths.Folders.MapGenerators+"/"+cfg.Local.Settings.MapGenerator+"-set.json")
	} else {
		factargs = append(factargs, "--preset")
		factargs = append(factargs, MapPreset)
	}

	lbuf := fmt.Sprintf("EXEC: %v ARGS: %v", GetFactorioBinary(), strings.Join(factargs, " "))
	cwlog.DoLogCW(lbuf)

	cmd := exec.Command(GetFactorioBinary(), factargs...)
	_, aerr := cmd.CombinedOutput()

	if aerr != nil {
		buf := fmt.Sprintf("An error occurred attempting to generate the map: %s", aerr)
		cwlog.DoLogCW(buf)
		CMS(cfg.Local.Channel.ChatChannel, buf)
		return
	}

	/* If available, use per-server ping setting... otherwise use global */
	pingstr := ""
	if cfg.Local.Options.PingString != "" {
		pingstr = cfg.Local.Options.PingString
	} else if cfg.Global.Options.PingString != "" {
		pingstr = cfg.Global.Options.PingString
	}
	CMS(cfg.Global.Discord.AnnounceChannel, pingstr+" Map "+cfg.Local.Callsign+"-"+cfg.Local.Name+" auto-reset.")

	/* Mods queue folder */
	qPath := cfg.Global.Paths.Folders.ServersRoot +
		cfg.Global.Paths.ChatWirePrefix +
		cfg.Local.Callsign + "/" +
		cfg.Global.Paths.Folders.FactorioDir + "/" +
		constants.ModsQueueFolder + "/"

	modPath := cfg.Global.Paths.Folders.ServersRoot +
		cfg.Global.Paths.ChatWirePrefix +
		cfg.Local.Callsign + "/" +
		cfg.Global.Paths.Folders.FactorioDir + "/" +
		constants.ModsFolder + "/"

	files, err := os.ReadDir(qPath)
	if err != nil {
		cwlog.DoLogCW(err.Error())
	}
	_, err = os.Stat(qPath)
	notfound := os.IsNotExist(err)

	if notfound {
		_, err = os.Create(qPath)
		if err != nil {
			cwlog.DoLogCW(err.Error())
		}
	} else {
		for _, f := range files {
			if strings.EqualFold(f.Name(), "mod-settings.dat") {
				err := os.Rename(qPath+f.Name(), modPath+f.Name())
				if err != nil {
					cwlog.DoLogCW(err.Error())
				} else {
					buf := "Installed new mod-settings.dat"
					cwlog.DoLogCW(buf)
					CMS(cfg.Local.Channel.ChatChannel, buf)
				}
			}

			if strings.HasSuffix(f.Name(), ".zip") {

				lowerName := strings.ToLower(f.Name())
				/* Delete mods queued up to be deleted */
				if strings.HasPrefix(lowerName, "deleteme-") {

					err = os.Remove(modPath + strings.TrimPrefix(lowerName, "deleteme-"))
					if err != nil {
						cwlog.DoLogCW(err.Error())
					}
					err = os.Remove(qPath + f.Name())
					if err != nil {
						cwlog.DoLogCW(err.Error())
					} else {
						buf := fmt.Sprintf("Removed mod: %v", strings.TrimPrefix(lowerName, "deleteme-"))
						cwlog.DoLogCW(buf)
						CMS(cfg.Local.Channel.ChatChannel, buf)
					}
				} else {

					/* Otherwise, install new mod */
					err := os.Rename(qPath+f.Name(), modPath+f.Name())
					if err != nil {
						cwlog.DoLogCW(err.Error())
					} else {
						buf := fmt.Sprintf("Installed mod: %v", f.Name())
						cwlog.DoLogCW(buf)
						CMS(cfg.Local.Channel.ChatChannel, buf)
					}
				}
			}
		}
	}

	glob.VoteBox.LastMapChange = time.Now()
	VoidAllVotes() /* Void all votes */
	WriteVotes()

	CMS(cfg.Local.Channel.ChatChannel, "Map reset complete, rebooting.")
	DoExit(false)
}
